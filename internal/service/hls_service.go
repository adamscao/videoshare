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

type CodecInfo struct {
	VideoCodec string
	AudioCodec string
}

type FFProbeOutput struct {
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
	} `json:"streams"`
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

	// Detect video codec and get file info
	codecInfo, err := s.getVideoCodec(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to detect codec: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	outputPlaylist := filepath.Join(outputDir, "playlist.m3u8")

	var args []string

	// Check if video is already h264
	if codecInfo.VideoCodec == "h264" {
		// H.264 video: use copy mode for fast processing
		args = []string{
			"-i", inputPath,
			"-c:v", "copy",
			"-c:a", "copy",
			"-hls_time", strconv.Itoa(s.config.FFmpeg.HLSTime),
			"-hls_list_size", "0",
			"-hls_segment_filename", filepath.Join(outputDir, s.config.FFmpeg.HLSSegmentFilename),
			"-f", "hls",
			outputPlaylist,
		}
	} else {
		// Non-H.264 video: check file size before transcoding
		maxSize := int64(512 * 1024 * 1024) // 512MB
		if fileSize > maxSize {
			return "", fmt.Errorf("video is not H.264 encoded and exceeds 512MB (%.2f MB). Please convert to H.264 first or upload a smaller file", float64(fileSize)/(1024*1024))
		}

		// File is small enough, proceed with transcoding
		args = []string{
			"-i", inputPath,
			"-c:v", "libx264",
			"-c:a", "aac",
			"-hls_time", strconv.Itoa(s.config.FFmpeg.HLSTime),
			"-hls_list_size", "0",
			"-hls_segment_filename", filepath.Join(outputDir, s.config.FFmpeg.HLSSegmentFilename),
			"-f", "hls",
			outputPlaylist,
		}
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

// getVideoCodec detects video and audio codecs
func (s *HLSService) getVideoCodec(inputPath string) (*CodecInfo, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name",
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

	codecInfo := &CodecInfo{}
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == "video" {
			codecInfo.VideoCodec = stream.CodecName
		} else if stream.CodecType == "audio" {
			codecInfo.AudioCodec = stream.CodecName
		}
	}

	return codecInfo, nil
}
