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
	"strings"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
)

type SubtitleService struct {
	config *config.Config
}

type WhisperResponse struct {
	Text string `json:"text"`
}

type TranslationRequest struct {
	Model    string          `json:"model"`
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

func NewSubtitleService(cfg *config.Config) *SubtitleService {
	return &SubtitleService{config: cfg}
}

// GenerateSubtitle generates subtitle using OpenAI Whisper API
func (s *SubtitleService) GenerateSubtitle(videoID uint, videoPath string) (string, error) {
	var video models.Video
	if err := database.DB.First(&video, videoID).Error; err != nil {
		return "", err
	}

	// Call Whisper API
	transcription, err := s.callWhisperAPI(videoPath)
	if err != nil {
		return "", fmt.Errorf("whisper API error: %w", err)
	}

	// Detect if text is Chinese
	isChinese := s.containsChinese(transcription)

	var subtitleContent string
	if isChinese {
		// Chinese transcription - use as is
		subtitleContent = s.convertToVTT(transcription, "")
	} else {
		// Non-Chinese - translate to Chinese and create bilingual subtitle
		translation, err := s.translateToChinese(transcription)
		if err != nil {
			return "", fmt.Errorf("translation error: %w", err)
		}
		subtitleContent = s.convertToVTT(transcription, translation)
	}

	// Save subtitle file
	subtitlePath := filepath.Join(s.config.Storage.SubtitlesDir, video.Slug+".vtt")
	if err := os.WriteFile(subtitlePath, []byte(subtitleContent), 0644); err != nil {
		return "", fmt.Errorf("failed to save subtitle: %w", err)
	}

	// Update database
	database.DB.Model(&video).Update("subtitle_path", subtitlePath)

	return subtitlePath, nil
}

// callWhisperAPI calls OpenAI Whisper API
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
	writer.WriteField("response_format", "text")
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

// translateToChinese translates text to Chinese using OpenAI API
func (s *SubtitleService) translateToChinese(text string) (string, error) {
	reqBody := TranslationRequest{
		Model: s.config.OpenAI.TranslationModel,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are a professional translator. Translate the following text to Chinese. Only return the translation, no explanations.",
			},
			{
				Role:    "user",
				Content: text,
			},
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

// convertToVTT converts text to WebVTT format with optional translation
func (s *SubtitleService) convertToVTT(original, translation string) string {
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")

	// Split text into sentences (simple split by period, question mark, exclamation)
	sentences := s.splitIntoSentences(original)
	translatedSentences := []string{}

	if translation != "" {
		translatedSentences = s.splitIntoSentences(translation)
	}

	duration := 5 // seconds per subtitle
	for i, sentence := range sentences {
		if sentence == "" {
			continue
		}

		start := i * duration
		end := start + duration

		// Format timestamp
		startTime := s.formatTimestamp(start)
		endTime := s.formatTimestamp(end)

		vtt.WriteString(fmt.Sprintf("%d\n", i+1))
		vtt.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))

		if translation != "" && i < len(translatedSentences) {
			// Bilingual subtitle
			vtt.WriteString(fmt.Sprintf("<v original>%s</v>\n", strings.TrimSpace(sentence)))
			vtt.WriteString(fmt.Sprintf("<v chinese>%s</v>\n", strings.TrimSpace(translatedSentences[i])))
		} else {
			// Single language
			vtt.WriteString(fmt.Sprintf("%s\n", strings.TrimSpace(sentence)))
		}
		vtt.WriteString("\n")
	}

	return vtt.String()
}

// splitIntoSentences splits text into sentences
func (s *SubtitleService) splitIntoSentences(text string) []string {
	// Simple sentence split
	re := regexp.MustCompile(`[.!?。！？]+`)
	sentences := re.Split(text, -1)

	var result []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// formatTimestamp formats seconds to VTT timestamp format
func (s *SubtitleService) formatTimestamp(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d.000", hours, minutes, secs)
}

// containsChinese checks if text contains Chinese characters
func (s *SubtitleService) containsChinese(text string) bool {
	re := regexp.MustCompile(`\p{Han}`)
	return re.MatchString(text)
}

// SaveUploadedSubtitle saves an uploaded subtitle file
func (s *SubtitleService) SaveUploadedSubtitle(videoSlug string, content []byte) (string, error) {
	subtitlePath := filepath.Join(s.config.Storage.SubtitlesDir, videoSlug+".vtt")

	// Convert SRT to VTT if needed
	contentStr := string(content)
	if !strings.HasPrefix(contentStr, "WEBVTT") {
		contentStr = s.convertSRTtoVTT(contentStr)
	}

	if err := os.WriteFile(subtitlePath, []byte(contentStr), 0644); err != nil {
		return "", err
	}

	return subtitlePath, nil
}

// convertSRTtoVTT converts SRT subtitle to VTT format
func (s *SubtitleService) convertSRTtoVTT(srt string) string {
	lines := strings.Split(srt, "\n")
	var vtt strings.Builder
	vtt.WriteString("WEBVTT\n\n")

	for _, line := range lines {
		// Replace SRT timestamp format with VTT format
		if strings.Contains(line, " --> ") {
			line = strings.ReplaceAll(line, ",", ".")
		}
		vtt.WriteString(line + "\n")
	}

	return vtt.String()
}
