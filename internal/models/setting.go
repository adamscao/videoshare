package models

import (
	"time"
)

type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;size:50" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	SettingUploadPermission = "upload_permission"
)

const (
	UploadPermissionPublic = "public"
	UploadPermissionAdmin  = "admin"
)
