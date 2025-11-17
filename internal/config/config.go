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
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type StorageConfig struct {
	VideosDir    string `yaml:"videos_dir"`
	OriginalsDir string `yaml:"originals_dir"`
	HLSDir       string `yaml:"hls_dir"`
	ImportDir    string `yaml:"import_dir"`
}

type UploadConfig struct {
	MaxSize      int64    `yaml:"max_size"`
	AllowedTypes []string `yaml:"allowed_types"`
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
