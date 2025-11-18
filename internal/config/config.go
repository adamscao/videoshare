package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Upload   UploadConfig   `yaml:"upload"`
	FFmpeg   FFmpegConfig   `yaml:"ffmpeg"`
	Session  SessionConfig  `yaml:"session"`
	OpenAI   OpenAIConfig   `yaml:"openai"`
	Subtitle SubtitleConfig `yaml:"subtitle"`
}

type ServerConfig struct {
	Port      int    `yaml:"port"`
	Host      string `yaml:"host"`
	GitHubURL string `yaml:"github_url"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type StorageConfig struct {
	VideosDir     string `yaml:"videos_dir"`
	OriginalsDir  string `yaml:"originals_dir"`
	HLSDir        string `yaml:"hls_dir"`
	ImportDir     string `yaml:"import_dir"`
	SubtitlesDir  string `yaml:"subtitles_dir"`
}

type UploadConfig struct {
	MaxSize        int64    `yaml:"max_size"`
	AllowedTypes   []string `yaml:"allowed_types"`
	SubtitleTypes  []string `yaml:"subtitle_types"`
}

type FFmpegConfig struct {
	Path               string `yaml:"path"`
	FFprobePath        string `yaml:"ffprobe_path"`
	HLSTime            int    `yaml:"hls_time"`
	HLSSegmentFilename string `yaml:"hls_segment_filename"`
}

type SessionConfig struct {
	Secret string `yaml:"secret"`
	MaxAge int    `yaml:"max_age"`
}

type OpenAIConfig struct {
	APIKey           string `yaml:"api_key"`
	APIBase          string `yaml:"api_base"`
	WhisperModel     string `yaml:"whisper_model"`
	TranslationModel string `yaml:"translation_model"`
	UseLocalWhisper  bool   `yaml:"use_local_whisper"`  // Use local whisper command instead of API
	WhisperPath      string `yaml:"whisper_path"`       // Path to local whisper executable
}

type SubtitleConfig struct {
	ChineseColor  string `yaml:"chinese_color"`
	ChineseFont   string `yaml:"chinese_font"`
	OriginalColor string `yaml:"original_color"`
	OriginalFont  string `yaml:"original_font"`
	FontSize      string `yaml:"font_size"`
	Background    string `yaml:"background"`
	Position      string `yaml:"position"`
}

var GlobalConfig *Config

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	GlobalConfig = &config
	return &config, nil
}
