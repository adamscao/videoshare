package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/adamscao/videoshare/internal/config"
)

type HLSService struct {
	config *config.Config
}

type VideoInfo struct {
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

type FFProbeOutput struct {
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
	} `json:"format"`
}

func NewHLSService(cfg *config.Config) *HLSService {
	return &HLSService{config: cfg}
}

// ConvertToHLS converts video to HLS format
func (s *HLSService) ConvertToHLS(inputPath, outputDir string) (string, error) {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	outputPlaylist := filepath.Join(outputDir, "playlist.m3u8")

	// FFmpeg command to convert to HLS
	args := []string{
		"-i", inputPath,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-hls_time", strconv.Itoa(s.config.FFmpeg.HLSTime),
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outputDir, s.config.FFmpeg.HLSSegmentFilename),
		"-f", "hls",
		outputPlaylist,
	}

	cmd := exec.Command(s.config.FFmpeg.Path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	return outputPlaylist, nil
}

// GetVideoInfo extracts video metadata using ffprobe
func (s *HLSService) GetVideoInfo(inputPath string) (*VideoInfo, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration,size",
		"-of", "json",
		inputPath,
	}

	cmd := exec.Command(s.config.FFmpeg.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe error: %w", err)
	}

	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	duration, _ := strconv.ParseFloat(probeOutput.Format.Duration, 64)
	size, _ := strconv.ParseInt(probeOutput.Format.Size, 10, 64)

	return &VideoInfo{
		Duration: duration,
		Size:     size,
	}, nil
}
