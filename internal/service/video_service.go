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

// CreateVideo creates a video record and converts to HLS synchronously.
// Used by the import service where blocking is acceptable.
func (s *VideoService) CreateVideo(originalPath, filename, title, description, uploadType string, setPassword bool, password string) (*models.Video, error) {
	slug, err := s.generateUniqueSlug()
	if err != nil {
		return nil, err
	}

	info, err := s.hlsService.GetVideoInfo(originalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	hlsDir := filepath.Join(s.config.Storage.HLSDir, slug)
	hlsPlaylist, err := s.hlsService.ConvertToHLS(originalPath, hlsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to HLS: %w", err)
	}

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
		Status:              "ready",
	}

	if setPassword && password != "" {
		hash, err := utils.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		video.PasswordHash = hash
	}

	if err := database.DB.Create(video).Error; err != nil {
		os.RemoveAll(hlsDir)
		return nil, fmt.Errorf("failed to save video: %w", err)
	}

	return video, nil
}

// CreateVideoAsync saves a video record immediately with status=pending and
// starts HLS transcoding in the background. Returns the video with its slug
// so the caller can redirect the user right away.
func (s *VideoService) CreateVideoAsync(originalPath, filename, title, description, uploadType string, setPassword bool, password string) (*models.Video, error) {
	slug, err := s.generateUniqueSlug()
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(originalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	video := &models.Video{
		Slug:                slug,
		Title:               title,
		Description:         description,
		OriginalFilename:    filename,
		OriginalPath:        originalPath,
		FileSize:            fileInfo.Size(),
		IsPasswordProtected: setPassword,
		UploadType:          uploadType,
		Status:              "pending",
	}

	if setPassword && password != "" {
		hash, err := utils.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		video.PasswordHash = hash
	}

	if err := database.DB.Create(video).Error; err != nil {
		return nil, fmt.Errorf("failed to save video: %w", err)
	}

	go s.processVideoAsync(video.ID, originalPath, slug)

	return video, nil
}

// processVideoAsync does HLS conversion in the background and updates the video status.
func (s *VideoService) processVideoAsync(videoID uint, originalPath, slug string) {
	database.DB.Model(&models.Video{}).Where("id = ?", videoID).Update("status", "processing")

	info, err := s.hlsService.GetVideoInfo(originalPath)
	if err != nil {
		database.DB.Model(&models.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
			"status":          "failed",
			"transcode_error": err.Error(),
		})
		return
	}

	hlsDir := filepath.Join(s.config.Storage.HLSDir, slug)
	hlsPlaylist, err := s.hlsService.ConvertToHLS(originalPath, hlsDir)
	if err != nil {
		os.RemoveAll(hlsDir)
		database.DB.Model(&models.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
			"status":          "failed",
			"transcode_error": err.Error(),
		})
		return
	}

	database.DB.Model(&models.Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
		"status":   "ready",
		"hls_path": hlsPlaylist,
		"duration": int(info.Duration),
	})
}

func (s *VideoService) generateUniqueSlug() (string, error) {
	slug, err := utils.GenerateSlug(10)
	if err != nil {
		return "", fmt.Errorf("failed to generate slug: %w", err)
	}
	var existing models.Video
	for {
		if database.DB.Where("slug = ?", slug).First(&existing).Error != nil {
			break
		}
		slug, _ = utils.GenerateSlug(10)
	}
	return slug, nil
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
