package models

import (
	"time"
)

type Video struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Slug                string    `gorm:"uniqueIndex;size:20" json:"slug"`
	Title               string    `gorm:"size:255" json:"title"`
	Description         string    `gorm:"type:text" json:"description"`
	OriginalFilename    string    `gorm:"size:255" json:"original_filename"`
	OriginalPath        string    `gorm:"size:500" json:"original_path"`
	HLSPath             string    `gorm:"size:500" json:"hls_path"`
	SubtitlePath        string    `gorm:"size:500" json:"subtitle_path"` // 字幕文件路径
	Duration            int       `json:"duration"`           // 时长（秒）
	FileSize            int64     `json:"file_size"`          // 文件大小（字节）
	IsPasswordProtected bool      `json:"is_password_protected"`
	PasswordHash        string    `gorm:"size:255" json:"-"` // 不返回给前端
	UploadType          string    `gorm:"size:20" json:"upload_type"` // web/import/admin
	Status              string    `gorm:"size:20;default:ready" json:"status"` // pending/processing/ready/failed
	TranscodeError      string    `gorm:"type:text" json:"transcode_error,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
