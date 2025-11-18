package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
	"github.com/adamscao/videoshare/internal/utils"
)

type VideoService struct {
	config     *config.Config
	hlsService *HLSService
}

func NewVideoService(cfg *config.Config, hlsService *HLSService) *VideoService {
	return &VideoService{
		config:     cfg,
		hlsService: hlsService,
	}
}

// CreateVideo creates a new video record from uploaded file
func (s *VideoService) CreateVideo(originalPath, filename, title, description, uploadType string, setPassword bool, password string) (*models.Video, error) {
	// Generate unique slug
	slug, err := utils.GenerateSlug(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate slug: %w", err)
	}

	// Check if slug already exists (rare but possible)
	var existing models.Video
	for {
		result := database.DB.Where("slug = ?", slug).First(&existing)
		if result.Error != nil {
			break // Slug is unique
		}
		slug, _ = utils.GenerateSlug(10)
	}

	// Get video info
	info, err := s.hlsService.GetVideoInfo(originalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// Convert to HLS
	hlsDir := filepath.Join(s.config.Storage.HLSDir, slug)
	hlsPlaylist, err := s.hlsService.ConvertToHLS(originalPath, hlsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to HLS: %w", err)
	}

	// Create video record
	video := &models.Video{
		Slug:                slug,
		Title:               title,
		Description:         description,
		OriginalFilename:    filename,
		OriginalPath:        originalPath,
		HLSPath:             hlsPlaylist,
		Duration:            int(info.Duration),
		FileSize:            info.Size,
		IsPasswordProtected: setPassword,
		UploadType:          uploadType,
	}

	// Set password if required
	if setPassword && password != "" {
		hash, err := utils.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		video.PasswordHash = hash
	}

	// Save to database
	if err := database.DB.Create(video).Error; err != nil {
		// Clean up HLS files on error
		os.RemoveAll(hlsDir)
		return nil, fmt.Errorf("failed to save video: %w", err)
	}

	return video, nil
}

// GetVideoBySlug retrieves video by slug
func (s *VideoService) GetVideoBySlug(slug string) (*models.Video, error) {
	var video models.Video
	if err := database.DB.Where("slug = ?", slug).First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

// GetAllVideos retrieves all videos
func (s *VideoService) GetAllVideos() ([]models.Video, error) {
	var videos []models.Video
	if err := database.DB.Order("created_at DESC").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// UpdateVideo updates video information
func (s *VideoService) UpdateVideo(id uint, title, description string, isPasswordProtected bool, password string) error {
	updates := map[string]interface{}{
		"title":                 title,
		"description":           description,
		"is_password_protected": isPasswordProtected,
	}

	// Update password if provided
	if password != "" {
		hash, err := utils.HashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		updates["password_hash"] = hash
	} else if !isPasswordProtected {
		updates["password_hash"] = ""
	}

	return database.DB.Model(&models.Video{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteVideo deletes video and its files
func (s *VideoService) DeleteVideo(id uint) error {
	var video models.Video
	if err := database.DB.First(&video, id).Error; err != nil {
		return err
	}

	// Delete HLS files
	hlsDir := filepath.Dir(video.HLSPath)
	if err := os.RemoveAll(hlsDir); err != nil {
		return fmt.Errorf("failed to delete HLS files: %w", err)
	}

	// Delete original file
	if err := os.Remove(video.OriginalPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete original file: %w", err)
	}

	// Delete database record
	return database.DB.Delete(&video).Error
}

// VerifyPassword verifies video password
func (s *VideoService) VerifyPassword(video *models.Video, password string) bool {
	if !video.IsPasswordProtected {
		return true
	}
	return utils.CheckPassword(password, video.PasswordHash)
}
