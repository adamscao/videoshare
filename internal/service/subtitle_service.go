package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
	"github.com/google/uuid"
)

type SubtitleService struct {
	config *config.Config
}

type TranslationRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type TranslationResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// WhisperSegment represents one timestamped segment from Whisper's verbose_json output.
type WhisperSegment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

// WhisperResponse is the complete response from Whisper API in verbose_json format.
type WhisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Segments []WhisperSegment `json:"segments"`
	Text     string           `json:"text"`
}

// TranslationItem is used for JSON-based translation input/output.
type TranslationItem struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

func NewSubtitleService(cfg *config.Config) *SubtitleService {
	return &SubtitleService{config: cfg}
}

// GenerateSubtitle generates subtitle using OpenAI Whisper API with accurate timestamps.
// Audio is preprocessed to 16kHz mono 48kbps and split if >20MB.
// Uses strict JSON format for both Whisper response and translation to ensure ID alignment.
func (s *SubtitleService) GenerateSubtitle(videoID uint, videoPath string) (string, error) {
	var video models.Video
	if err := database.DB.First(&video, videoID).Error; err != nil {
		return "", err
	}

	// Step 1: Extract and convert audio to Whisper-friendly format (16kHz mono 48kbps)
	audioPath, err := s.prepareAudioForWhisper(videoPath)
	if err != nil {
		return "", fmt.Errorf("failed to prepare audio: %w", err)
	}
	defer os.Remove(audioPath)

	// Step 2: Split into chunks if larger than 20MB
	audioChunks, err := s.splitAudioIfNeeded(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to split audio: %w", err)
	}
	defer func() {
		for _, chunk := range audioChunks {
			if chunk != audioPath {
				os.Remove(chunk)
			}
		}
	}()

	// Step 3: Call Whisper API and get JSON response
	var whisperResp *WhisperResponse
	if len(audioChunks) == 1 {
		whisperResp, err = s.callWhisperAPI(audioChunks[0])
	} else {
		whisperResp, err = s.callWhisperAPIForChunks(audioChunks)
	}
	if err != nil {
		return "", fmt.Errorf("whisper API error: %w", err)
	}

	// Step 4: Save original Whisper JSON for debugging
	if err := s.saveWhisperJSON(video.Slug, whisperResp); err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to save Whisper JSON: %v\n", err)
	}

	// Step 5: Detect language and build VTT
	isChinese := s.containsChinese(whisperResp.Text)

	var subtitleContent string
	if isChinese {
		subtitleContent = s.buildMonolingualVTTFromJSON(whisperResp.Segments)
	} else {
		translations, err := s.translateSegmentsJSON(whisperResp.Segments)
		if err != nil {
			return "", fmt.Errorf("translation error: %w", err)
		}
		subtitleContent = s.buildBilingualVTTFromJSON(whisperResp.Segments, translations)
	}

	subtitlePath := filepath.Join(s.config.Storage.SubtitlesDir, video.Slug+".vtt")
	if err := os.WriteFile(subtitlePath, []byte(subtitleContent), 0644); err != nil {
		return "", fmt.Errorf("failed to save subtitle: %w", err)
	}

	database.DB.Model(&video).Update("subtitle_path", subtitlePath)
	return subtitlePath, nil
}

// prepareAudioForWhisper extracts audio from video and converts to Whisper-friendly format:
// 16kHz sample rate, mono channel, 48kbps bitrate
func (s *SubtitleService) prepareAudioForWhisper(videoPath string) (string, error) {
	tempDir := os.TempDir()
	audioPath := filepath.Join(tempDir, fmt.Sprintf("whisper_audio_%s.mp3", uuid.New().String()))

	args := []string{
		"-i", videoPath,
		"-vn",          // no video
		"-ac", "1",     // mono
		"-ar", "16000", // 16kHz
		"-b:a", "48k",  // 48kbps
		"-f", "mp3",
		audioPath,
	}

	cmd := exec.Command(s.config.FFmpeg.Path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg audio extraction failed: %w, output: %s", err, string(output))
	}

	return audioPath, nil
}

// splitAudioIfNeeded splits audio file into ~20MB chunks to stay within Whisper API limits.
func (s *SubtitleService) splitAudioIfNeeded(audioPath string) ([]string, error) {
	fileInfo, err := os.Stat(audioPath)
	if err != nil {
		return nil, err
	}

	const maxSize = 20 * 1024 * 1024 // 20MB
	if fileInfo.Size() <= maxSize {
		return []string{audioPath}, nil
	}

	// Get audio duration
	durationCmd := exec.Command(s.config.FFmpeg.FFprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	)
	durationOutput, err := durationCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get audio duration: %w", err)
	}

	totalDuration, _ := strconv.ParseFloat(strings.TrimSpace(string(durationOutput)), 64)
	if totalDuration <= 0 {
		return nil, fmt.Errorf("invalid audio duration: %f", totalDuration)
	}

	// Calculate chunk duration to get ~20MB chunks
	chunkCount := int(math.Ceil(float64(fileInfo.Size()) / float64(maxSize)))
	chunkDuration := totalDuration / float64(chunkCount)

	var chunks []string
	tempDir := filepath.Dir(audioPath)
	baseName := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))

	for i := 0; i < chunkCount; i++ {
		startTime := float64(i) * chunkDuration
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("%s_chunk_%d.mp3", baseName, i))

		args := []string{
			"-i", audioPath,
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%.2f", chunkDuration),
			"-c", "copy",
			chunkPath,
		}

		cmd := exec.Command(s.config.FFmpeg.Path, args...)
		if err := cmd.Run(); err != nil {
			for _, c := range chunks {
				os.Remove(c)
			}
			return nil, fmt.Errorf("failed to split audio chunk %d: %w", i, err)
		}

		chunks = append(chunks, chunkPath)
	}

	return chunks, nil
}

// callWhisperAPIForChunks processes multiple audio chunks through Whisper and merges results.
func (s *SubtitleService) callWhisperAPIForChunks(audioChunks []string) (*WhisperResponse, error) {
	var allSegments []WhisperSegment
	timeOffset := 0.0
	nextID := 0

	for i, chunkPath := range audioChunks {
		whisperResp, err := s.callWhisperAPI(chunkPath)
		if err != nil {
			return nil, fmt.Errorf("failed to transcribe chunk %d: %w", i, err)
		}

		// Adjust timestamps and IDs
		for _, seg := range whisperResp.Segments {
			adjustedSeg := WhisperSegment{
				ID:               nextID,
				Start:            seg.Start + timeOffset,
				End:              seg.End + timeOffset,
				Text:             seg.Text,
				Tokens:           seg.Tokens,
				Temperature:      seg.Temperature,
				AvgLogprob:       seg.AvgLogprob,
				CompressionRatio: seg.CompressionRatio,
				NoSpeechProb:     seg.NoSpeechProb,
			}
			allSegments = append(allSegments, adjustedSeg)
			nextID++
		}

		// Update offset for next chunk
		if len(whisperResp.Segments) > 0 {
			lastSeg := whisperResp.Segments[len(whisperResp.Segments)-1]
			timeOffset += lastSeg.End
		}
	}

	return &WhisperResponse{
		Task:     "transcribe",
		Language: "auto",
		Duration: timeOffset,
		Segments: allSegments,
	}, nil
}

// callWhisperAPI calls OpenAI Whisper API and returns verbose JSON with timestamps.
func (s *SubtitleService) callWhisperAPI(audioPath string) (*WhisperResponse, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, err
	}
	io.Copy(part, file)

	writer.WriteField("model", s.config.OpenAI.WhisperModel)
	writer.WriteField("response_format", "verbose_json") // Get detailed JSON with timestamps
	writer.Close()

	req, err := http.NewRequest("POST", s.config.OpenAI.APIBase+"/audio/transcriptions", body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.config.OpenAI.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper API returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	var whisperResp WhisperResponse
	if err := json.Unmarshal(responseBody, &whisperResp); err != nil {
		return nil, fmt.Errorf("failed to parse Whisper JSON: %w", err)
	}

	return &whisperResp, nil
}

// saveWhisperJSON saves the original Whisper response for debugging.
func (s *SubtitleService) saveWhisperJSON(slug string, whisperResp *WhisperResponse) error {
	jsonPath := filepath.Join(s.config.Storage.SubtitlesDir, slug+"_whisper.json")
	data, err := json.MarshalIndent(whisperResp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(jsonPath, data, 0644)
}

// translateSegmentsJSON translates segments using strict JSON format with fallback.
//
// The core alignment problem: Whisper splits sentences across multiple segments (e.g.
// segment 10 = "The Pentagon... models" and segment 11 = "for all lawful uses.").
// The LLM merges them into one Chinese sentence, but to keep the output count correct it
// shifts all subsequent translations forward by one — validateTranslationAlignment passes
// (same count, same IDs) but every translation after the merge point is wrong.
//
// Fix: pre-merge incomplete-sentence segments into whole sentences before translation.
// The LLM receives complete sentences and has no reason to merge. The result is then
// expanded back to the original segment count by duplicating each translation across all
// segments that were merged into it.
func (s *SubtitleService) translateSegmentsJSON(segments []WhisperSegment) ([]TranslationItem, error) {
	// Pre-merge fragments into complete sentences
	mergedSegments, groups := preMergeSegments(segments)

	result, err := s.translateSegmentsBatch(mergedSegments)
	if err == nil && s.validateTranslationAlignment(mergedSegments, result) {
		return expandByGroups(segments, groups, result), nil
	}

	// LLM still merged some items despite clean input; try expanding the partial result
	if err == nil && len(result) < len(mergedSegments) && len(result) > 0 {
		expanded, expandErr := s.expandMergedTranslations(mergedSegments, result)
		if expandErr == nil {
			fmt.Printf("Expanded %d merged translations to %d merged segments\n", len(result), len(mergedSegments))
			return expandByGroups(segments, groups, expanded), nil
		}
		fmt.Printf("Failed to expand merged translations: %v\n", expandErr)
	}

	// Final fallback: one-by-one on original segments
	fmt.Printf("Batch translation misaligned (%d merged segments -> %d translations), using one-by-one...\n", len(mergedSegments), len(result))
	return s.translateSegmentsOneByOne(segments)
}

// preMergeSegments groups consecutive segments into complete sentences.
// Sentence boundaries are detected by terminal punctuation (. ? !).
// Returns merged segments (each using the first original segment's ID and start time)
// and groups[i] = slice of original segment IDs that were merged into merged segment i.
func preMergeSegments(segments []WhisperSegment) ([]WhisperSegment, [][]int) {
	if len(segments) == 0 {
		return nil, nil
	}

	var merged []WhisperSegment
	var groups [][]int

	var groupIDs []int
	var groupText strings.Builder
	var groupStart float64
	var groupFirstID int

	for i, seg := range segments {
		text := strings.TrimSpace(seg.Text)

		if len(groupIDs) == 0 {
			groupStart = seg.Start
			groupFirstID = seg.ID
		}
		groupIDs = append(groupIDs, seg.ID)
		if groupText.Len() > 0 {
			groupText.WriteByte(' ')
		}
		groupText.WriteString(text)

		// Flush on sentence-ending punctuation or at the last segment
		if endsWithSentencePunctuation(text) || i == len(segments)-1 {
			merged = append(merged, WhisperSegment{
				ID:    groupFirstID,
				Start: groupStart,
				End:   seg.End,
				Text:  groupText.String(),
			})
			groups = append(groups, groupIDs)
			groupIDs = nil
			groupText.Reset()
		}
	}

	return merged, groups
}

// endsWithSentencePunctuation reports whether text ends with . ? or !
func endsWithSentencePunctuation(text string) bool {
	if len(text) == 0 {
		return false
	}
	last := text[len(text)-1]
	return last == '.' || last == '?' || last == '!'
}

// expandByGroups maps merged translations back to original segment IDs,
// duplicating each translation across all segments that were merged into it.
func expandByGroups(originalSegments []WhisperSegment, groups [][]int, mergedTranslations []TranslationItem) []TranslationItem {
	// Map merged-group first-ID -> translation text
	transMap := make(map[int]string, len(mergedTranslations))
	for _, t := range mergedTranslations {
		transMap[t.ID] = t.Text
	}

	// Map each original segment ID -> first ID of its group
	segToGroupFirst := make(map[int]int, len(originalSegments))
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		first := group[0]
		for _, id := range group {
			segToGroupFirst[id] = first
		}
	}

	result := make([]TranslationItem, len(originalSegments))
	for i, seg := range originalSegments {
		result[i] = TranslationItem{
			ID:   seg.ID,
			Text: transMap[segToGroupFirst[seg.ID]],
		}
	}
	return result
}

// validateTranslationAlignment checks if translation IDs match original segments.
func (s *SubtitleService) validateTranslationAlignment(segments []WhisperSegment, translations []TranslationItem) bool {
	if len(translations) != len(segments) {
		return false
	}
	for i, seg := range segments {
		if i >= len(translations) || translations[i].ID != seg.ID {
			return false
		}
	}
	return true
}

// expandMergedTranslations duplicates merged translations across original segment IDs.
// When LLM merges segments (e.g., ID 10+11 -> one translation with ID 10), this function
// duplicates the merged translation to both segments, preserving translation quality while
// maintaining timeline alignment.
//
// Example: segments [0,1,2,3,4,5] with translations [{id:0,text:"A"}, {id:3,text:"B"}]
// becomes: [{id:0,text:"A"}, {id:1,text:"A"}, {id:2,text:"A"}, {id:3,text:"B"}, {id:4,text:"B"}, {id:5,text:"B"}]
func (s *SubtitleService) expandMergedTranslations(segments []WhisperSegment, translations []TranslationItem) ([]TranslationItem, error) {
	if len(translations) == 0 || len(segments) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	result := make([]TranslationItem, len(segments))

	// For each segment, find which translation covers it
	for i, seg := range segments {
		// Find the translation with the largest ID <= seg.ID
		var foundTranslation *TranslationItem
		for j := range translations {
			if translations[j].ID <= seg.ID {
				foundTranslation = &translations[j]
			} else {
				// Translations are expected to be sorted by ID, so we can break
				break
			}
		}

		if foundTranslation == nil {
			return nil, fmt.Errorf("no translation found for segment ID %d", seg.ID)
		}

		result[i] = TranslationItem{
			ID:   seg.ID,
			Text: foundTranslation.Text,
		}
	}

	return result, nil
}

// translateSegmentsBatch attempts to translate all segments in one API call.
func (s *SubtitleService) translateSegmentsBatch(segments []WhisperSegment) ([]TranslationItem, error) {
	// Build input JSON array
	var input []TranslationItem
	for _, seg := range segments {
		input = append(input, TranslationItem{
			ID:   seg.ID,
			Text: strings.TrimSpace(seg.Text),
		})
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	systemPrompt := `You are a professional subtitle translator. Translate the following JSON array of subtitles to Chinese.

Input format: [{"id": 0, "text": "original text"}, {"id": 1, "text": "another text"}, ...]
Output format: [{"id": 0, "text": "翻译文本"}, {"id": 1, "text": "另一个文本"}, ...]

CRITICAL RULES:
1. Keep the EXACT same IDs from input
2. Output EXACTLY the same number of entries as input
3. Do NOT merge multiple entries even if sentences seem incomplete
4. Each ID must have EXACTLY ONE translation
5. Translate each entry independently
6. Return ONLY the JSON array, no markdown, no explanations`

	translatedText, err := s.translateWithPrompt(string(inputJSON), systemPrompt)
	if err != nil {
		return nil, err
	}

	// Clean up markdown code blocks if LLM added them
	translatedText = strings.TrimSpace(translatedText)
	translatedText = strings.TrimPrefix(translatedText, "```json")
	translatedText = strings.TrimPrefix(translatedText, "```")
	translatedText = strings.TrimSuffix(translatedText, "```")
	translatedText = strings.TrimSpace(translatedText)

	// Parse translation JSON
	var output []TranslationItem
	if err := json.Unmarshal([]byte(translatedText), &output); err != nil {
		return nil, fmt.Errorf("failed to parse translation JSON: %w", err)
	}

	return output, nil
}

// translateSegmentsOneByOne translates each segment individually for guaranteed alignment.
func (s *SubtitleService) translateSegmentsOneByOne(segments []WhisperSegment) ([]TranslationItem, error) {
	result := make([]TranslationItem, len(segments))

	systemPrompt := "You are a professional subtitle translator. Translate the following subtitle text to Chinese. Even if the text seems incomplete or is just a fragment, translate it as-is. Return ONLY the translated text, no explanations."

	for i, seg := range segments {
		translated, err := s.translateWithPrompt(strings.TrimSpace(seg.Text), systemPrompt)
		if err != nil {
			// Fallback to original text on error
			result[i] = TranslationItem{ID: seg.ID, Text: seg.Text}
			fmt.Printf("Warning: failed to translate segment %d, using original text\n", seg.ID)
			continue
		}

		result[i] = TranslationItem{
			ID:   seg.ID,
			Text: strings.TrimSpace(translated),
		}
	}

	return result, nil
}

// buildMonolingualVTTFromJSON builds a single-language VTT from Whisper JSON segments.
func (s *SubtitleService) buildMonolingualVTTFromJSON(segments []WhisperSegment) string {
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")

	for _, seg := range segments {
		vtt.WriteString(fmt.Sprintf("%d\n", seg.ID+1))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n",
			formatVTTTime(seg.Start),
			formatVTTTime(seg.End)))
		vtt.WriteString(strings.TrimSpace(seg.Text) + "\n\n")
	}

	return vtt.String()
}

// buildBilingualVTTFromJSON builds a bilingual VTT with original + Chinese translation.
func (s *SubtitleService) buildBilingualVTTFromJSON(segments []WhisperSegment, translations []TranslationItem) string {
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")

	// Build translation map
	transMap := make(map[int]string)
	for _, t := range translations {
		transMap[t.ID] = t.Text
	}

	for _, seg := range segments {
		vtt.WriteString(fmt.Sprintf("%d\n", seg.ID+1))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n",
			formatVTTTime(seg.Start),
			formatVTTTime(seg.End)))

		if translation, ok := transMap[seg.ID]; ok && translation != "" {
			vtt.WriteString(fmt.Sprintf("<v original>%s</v>\n", strings.TrimSpace(seg.Text)))
			vtt.WriteString(fmt.Sprintf("<v chinese>%s</v>\n", translation))
		} else {
			vtt.WriteString(strings.TrimSpace(seg.Text) + "\n")
		}
		vtt.WriteString("\n")
	}

	return vtt.String()
}

// formatVTTTime converts seconds to VTT timestamp format "HH:MM:SS.mmm"
func formatVTTTime(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours)*3600) / 60)
	secs := seconds - float64(hours)*3600 - float64(minutes)*60
	millis := int((secs - float64(int(secs))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, int(secs), millis)
}

// translateWithPrompt calls the chat completion API with a custom system prompt.
func (s *SubtitleService) translateWithPrompt(text, systemPrompt string) (string, error) {
	reqBody := TranslationRequest{
		Model: s.config.OpenAI.TranslationModel,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", s.config.OpenAI.APIBase+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.OpenAI.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var translationResp TranslationResponse
	if err := json.NewDecoder(resp.Body).Decode(&translationResp); err != nil {
		return "", err
	}

	if len(translationResp.Choices) == 0 {
		return "", fmt.Errorf("no translation returned")
	}

	return translationResp.Choices[0].Message.Content, nil
}

// containsChinese checks if text contains Chinese characters.
func (s *SubtitleService) containsChinese(text string) bool {
	re := regexp.MustCompile(`\p{Han}`)
	return re.MatchString(text)
}

// SaveUploadedSubtitle saves an uploaded subtitle file (SRT or VTT).
func (s *SubtitleService) SaveUploadedSubtitle(videoSlug string, content []byte) (string, error) {
	subtitlePath := filepath.Join(s.config.Storage.SubtitlesDir, videoSlug+".vtt")

	contentStr := string(content)
	if !strings.HasPrefix(strings.TrimSpace(contentStr), "WEBVTT") {
		contentStr = convertSRTtoVTT(contentStr)
	}

	if err := os.WriteFile(subtitlePath, []byte(contentStr), 0644); err != nil {
		return "", err
	}

	return subtitlePath, nil
}

// convertSRTtoVTT converts SRT subtitle format to WebVTT.
func convertSRTtoVTT(srt string) string {
	lines := strings.Split(srt, "\n")
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")
	for _, line := range lines {
		if strings.Contains(line, " --> ") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		vtt.WriteString(line + "\n")
	}
	return vtt.String()
}
