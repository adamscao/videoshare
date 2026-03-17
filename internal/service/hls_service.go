package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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
	Streams []FFProbeStream `json:"streams"`
}

type FFProbeStream struct {
	CodecType          string `json:"codec_type"`
	CodecName          string `json:"codec_name"`
	Profile            string `json:"profile"`
	PixFmt             string `json:"pix_fmt"`
	BitsPerRawSample   string `json:"bits_per_raw_sample"`
	ColorSpace         string `json:"color_space"`
	ColorTransfer      string `json:"color_transfer"`
	RFrameRate         string `json:"r_frame_rate"`
}

type VideoCodecInfo struct {
	VideoCodec    string
	VideoProfile  string
	PixelFormat   string
	BitDepth      int
	FrameRate     float64
	ColorSpace    string
	ColorTransfer string
	AudioCodec    string
}

func NewHLSService(cfg *config.Config) *HLSService {
	return &HLSService{config: cfg}
}

// ConvertToHLS converts video to HLS, intelligently choosing copy or transcode based on compatibility.
func (s *HLSService) ConvertToHLS(inputPath, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	// Detect codec details
	codecInfo, err := s.getDetailedCodecInfo(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to detect codec: %w", err)
	}

	outputPlaylist := filepath.Join(outputDir, "playlist.m3u8")

	var args []string
	baseArgs := []string{
		"-i", inputPath,
	}

	// Check if format is compatible with browsers/mobile
	videoCompatible := s.isVideoCompatible(codecInfo)
	audioCompatible := codecInfo.AudioCodec == "" || codecInfo.AudioCodec == "aac"

	if videoCompatible && audioCompatible {
		// Fully compatible - fast copy mode
		args = append(baseArgs,
			"-c:v", "copy",
			"-c:a", "copy",
		)
	} else if videoCompatible && !audioCompatible {
		// Video OK but audio needs transcoding
		args = append(baseArgs,
			"-c:v", "copy",
			"-c:a", "aac",
			"-b:a", "128k",
		)
	} else {
		// Video needs transcoding - use compatibility settings
		args = append(baseArgs,
			"-c:v", "libx264",
			"-profile:v", "high",
			"-level", "4.1",
			"-pix_fmt", "yuv420p", // 8-bit for broad compatibility
			"-c:a", "aac",
			"-b:a", "128k",
		)

		// Limit framerate if too high
		if codecInfo.FrameRate > 60 {
			args = append(args, "-r", "60")
		}
	}

	// Add HLS-specific args
	args = append(args,
		"-hls_time", strconv.Itoa(s.config.FFmpeg.HLSTime),
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outputDir, s.config.FFmpeg.HLSSegmentFilename),
		"-f", "hls",
		outputPlaylist,
	)

	cmd := exec.Command(s.config.FFmpeg.Path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg error: %w, output: %s", err, string(output))
	}

	return outputPlaylist, nil
}

// getDetailedCodecInfo extracts detailed codec information using ffprobe.
func (s *HLSService) getDetailedCodecInfo(inputPath string) (*VideoCodecInfo, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name,profile,pix_fmt,bits_per_raw_sample,color_space,color_transfer,r_frame_rate",
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

	info := &VideoCodecInfo{}

	for _, stream := range probeOutput.Streams {
		if stream.CodecType == "video" {
			info.VideoCodec = stream.CodecName
			info.VideoProfile = stream.Profile
			info.PixelFormat = stream.PixFmt
			info.ColorSpace = stream.ColorSpace
			info.ColorTransfer = stream.ColorTransfer

			// Parse bit depth
			if stream.BitsPerRawSample != "" {
				bitDepth, _ := strconv.Atoi(stream.BitsPerRawSample)
				info.BitDepth = bitDepth
			} else {
				// Infer from pixel format
				if strings.Contains(stream.PixFmt, "10le") || strings.Contains(stream.PixFmt, "10be") {
					info.BitDepth = 10
				} else {
					info.BitDepth = 8
				}
			}

			// Parse framerate (e.g., "120/1" -> 120, "30000/1001" -> 29.97)
			if stream.RFrameRate != "" {
				parts := strings.Split(stream.RFrameRate, "/")
				if len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den > 0 {
						info.FrameRate = num / den
					}
				}
			}
		} else if stream.CodecType == "audio" {
			info.AudioCodec = stream.CodecName
		}
	}

	return info, nil
}

// isVideoCompatible checks if video encoding is compatible with mobile/browsers.
func (s *HLSService) isVideoCompatible(info *VideoCodecInfo) bool {
	// Must be H.264
	if info.VideoCodec != "h264" {
		return false
	}

	// High 10 profile is not compatible (requires 10-bit decoding)
	profileLower := strings.ToLower(info.VideoProfile)
	if strings.Contains(profileLower, "high 10") ||
		strings.Contains(profileLower, "high 4:2:2") ||
		strings.Contains(profileLower, "high 4:4:4") {
		return false
	}

	// 10-bit color depth not compatible with most mobile devices
	if info.BitDepth > 8 {
		return false
	}

	// HDR formats (bt2020, HLG, PQ) not broadly supported
	colorSpaceLower := strings.ToLower(info.ColorSpace)
	colorTransferLower := strings.ToLower(info.ColorTransfer)
	if strings.Contains(colorSpaceLower, "bt2020") ||
		strings.Contains(colorTransferLower, "arib-std-b67") || // HLG
		strings.Contains(colorTransferLower, "smpte2084") { // PQ/HDR10
		return false
	}

	// Very high framerates (>60fps) may cause issues
	if info.FrameRate > 60 {
		return false
	}

	return true
}

// GetVideoInfo extracts video metadata using ffprobe.
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
