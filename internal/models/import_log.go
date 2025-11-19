package models

import (
	"time"
)

type ImportLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Filename     string    `gorm:"size:500;not null" json:"filename"`
	Status       string    `gorm:"size:20;not null" json:"status"` // processing/success/failed/skipped
	Message      string    `gorm:"type:text" json:"message"`
	FileSize     int64     `json:"file_size"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	ImportTime   time.Time `gorm:"not null" json:"import_time"`
	VideoID      *uint     `json:"video_id"` // NULL if failed
	CreatedAt    time.Time `json:"created_at"`
}
