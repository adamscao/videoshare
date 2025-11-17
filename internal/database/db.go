package database

import (
	"github.com/adamscao/videoshare/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes database connection
func InitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	// Auto migrate models
	err = DB.AutoMigrate(
		&models.Video{},
		&models.Admin{},
		&models.Setting{},
	)
	if err != nil {
		return err
	}

	// Initialize default settings
	initDefaultSettings()

	return nil
}

func initDefaultSettings() {
	var count int64
	DB.Model(&models.Setting{}).Where("key = ?", models.SettingUploadPermission).Count(&count)
	if count == 0 {
		DB.Create(&models.Setting{
			Key:   models.SettingUploadPermission,
			Value: models.UploadPermissionPublic,
		})
	}
}
