package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
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

// SRTSegment represents one timed subtitle entry from Whisper's SRT output.
type SRTSegment struct {
	ID    int
	Start string // "00:00:00,000"
	End   string // "00:00:02,500"
	Text  string
}

func NewSubtitleService(cfg *config.Config) *SubtitleService {
	return &SubtitleService{config: cfg}
}

// GenerateSubtitle generates subtitle using OpenAI Whisper API with accurate timestamps.
func (s *SubtitleService) GenerateSubtitle(videoID uint, videoPath string) (string, error) {
	var video models.Video
	if err := database.DB.First(&video, videoID).Error; err != nil {
		return "", err
	}

	// Get SRT with real timestamps from Whisper
	srtContent, err := s.callWhisperAPI(videoPath)
	if err != nil {
		return "", fmt.Errorf("whisper API error: %w", err)
	}

	// Parse SRT into timestamped segments
	segments := s.parseSRT(srtContent)
	if len(segments) == 0 {
		return "", fmt.Errorf("no subtitle segments found in Whisper response")
	}

	// Detect language and build VTT
	allText := ""
	for _, seg := range segments {
		allText += seg.Text
	}

	var subtitleContent string
	if s.containsChinese(allText) {
		// Already Chinese: convert SRT timestamps directly to VTT
		subtitleContent = s.buildMonolingualVTT(segments)
	} else {
		// Translate to Chinese, keep real timestamps for both
		translations, err := s.translateSegments(segments)
		if err != nil {
			return "", fmt.Errorf("translation error: %w", err)
		}
		subtitleContent = s.buildBilingualVTT(segments, translations)
	}

	subtitlePath := filepath.Join(s.config.Storage.SubtitlesDir, video.Slug+".vtt")
	if err := os.WriteFile(subtitlePath, []byte(subtitleContent), 0644); err != nil {
		return "", fmt.Errorf("failed to save subtitle: %w", err)
	}

	database.DB.Model(&video).Update("subtitle_path", subtitlePath)
	return subtitlePath, nil
}

// callWhisperAPI calls OpenAI Whisper API and returns SRT-formatted content with timestamps.
func (s *SubtitleService) callWhisperAPI(audioPath string) (string, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", err
	}
	io.Copy(part, file)

	writer.WriteField("model", s.config.OpenAI.WhisperModel)
	writer.WriteField("response_format", "srt") // returns timestamped SRT
	writer.Close()

	req, err := http.NewRequest("POST", s.config.OpenAI.APIBase+"/audio/transcriptions", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.config.OpenAI.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper API returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	return string(responseBody), nil
}

// parseSRT parses SRT content into a slice of segments with timing info.
func (s *SubtitleService) parseSRT(srt string) []SRTSegment {
	var segments []SRTSegment
	// Normalize line endings and split by blank line between entries
	srt = strings.ReplaceAll(srt, "\r\n", "\n")
	blocks := strings.Split(strings.TrimSpace(srt), "\n\n")

	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 3 {
			continue
		}

		id, err := strconv.Atoi(strings.TrimSpace(lines[0]))
		if err != nil {
			continue
		}

		// "00:00:00,000 --> 00:00:02,500"
		timeParts := strings.Split(lines[1], " --> ")
		if len(timeParts) != 2 {
			continue
		}

		text := strings.Join(lines[2:], "\n")

		segments = append(segments, SRTSegment{
			ID:    id,
			Start: strings.TrimSpace(timeParts[0]),
			End:   strings.TrimSpace(timeParts[1]),
			Text:  strings.TrimSpace(text),
		})
	}

	return segments
}

// srtTimeToVTT converts SRT timestamp "00:00:00,000" to VTT "00:00:00.000"
func srtTimeToVTT(ts string) string {
	return strings.ReplaceAll(ts, ",", ".")
}

// buildMonolingualVTT builds a single-language VTT from SRT segments with real timestamps.
func (s *SubtitleService) buildMonolingualVTT(segments []SRTSegment) string {
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")
	for _, seg := range segments {
		vtt.WriteString(fmt.Sprintf("%d\n", seg.ID))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n", srtTimeToVTT(seg.Start), srtTimeToVTT(seg.End)))
		vtt.WriteString(seg.Text + "\n\n")
	}
	return vtt.String()
}

// buildBilingualVTT builds a bilingual VTT with original + Chinese translation, using real timestamps.
func (s *SubtitleService) buildBilingualVTT(segments []SRTSegment, translations []string) string {
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")
	for i, seg := range segments {
		vtt.WriteString(fmt.Sprintf("%d\n", seg.ID))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n", srtTimeToVTT(seg.Start), srtTimeToVTT(seg.End)))
		if i < len(translations) && translations[i] != "" {
			vtt.WriteString(fmt.Sprintf("<v original>%s</v>\n", seg.Text))
			vtt.WriteString(fmt.Sprintf("<v chinese>%s</v>\n", translations[i]))
		} else {
			vtt.WriteString(seg.Text + "\n")
		}
		vtt.WriteString("\n")
	}
	return vtt.String()
}

// translateSegments translates all segments to Chinese in one API call using numbered markers.
func (s *SubtitleService) translateSegments(segments []SRTSegment) ([]string, error) {
	var input strings.Builder
	for _, seg := range segments {
		input.WriteString(fmt.Sprintf("%d: %s\n", seg.ID, seg.Text))
	}

	systemPrompt := "You are a professional subtitle translator. Translate the following numbered subtitle lines to Chinese. Keep the exact same numbering format '1: text'. Return only the translated lines, one per line, nothing else."
	translated, err := s.translateWithPrompt(input.String(), systemPrompt)
	if err != nil {
		return nil, err
	}

	// Map segment ID -> translation
	idToTranslation := make(map[int]string)
	for _, line := range strings.Split(translated, "\n") {
		line = strings.TrimSpace(line)
		colonIdx := strings.Index(line, ":")
		if colonIdx < 1 {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(line[:colonIdx]))
		if err != nil {
			continue
		}
		idToTranslation[id] = strings.TrimSpace(line[colonIdx+1:])
	}

	// Build results in segment order
	results := make([]string, len(segments))
	for i, seg := range segments {
		results[i] = idToTranslation[seg.ID]
	}
	return results, nil
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
