package handler

import (
	"net/http"
	"strconv"

	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/models"
	"github.com/adamscao/videoshare/internal/service"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	videoService    *service.VideoService
	importService   *service.ImportService
	subtitleService *service.SubtitleService
}

func NewAdminHandler(videoService *service.VideoService, importService *service.ImportService, subtitleService *service.SubtitleService) *AdminHandler {
	return &AdminHandler{
		videoService:    videoService,
		importService:   importService,
		subtitleService: subtitleService,
	}
}

// ShowDashboard shows admin dashboard
func (h *AdminHandler) ShowDashboard(c *gin.Context) {
	videos, err := h.videoService.GetAllVideos()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"message": "Failed to load videos",
		})
		return
	}

	// Get upload permission setting
	var setting models.Setting
	uploadPermission := models.UploadPermissionPublic
	if err := database.DB.Where("key = ?", models.SettingUploadPermission).First(&setting).Error; err == nil {
		uploadPermission = setting.Value
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"videos":           videos,
		"uploadPermission": uploadPermission,
	})
}

// GetVideos returns all videos as JSON
func (h *AdminHandler) GetVideos(c *gin.Context) {
	videos, err := h.videoService.GetAllVideos()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get videos"})
		return
	}

	c.JSON(http.StatusOK, videos)
}

// GetVideo returns single video details
func (h *AdminHandler) GetVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var video models.Video
	if err := database.DB.First(&video, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	c.JSON(http.StatusOK, video)
}

// UpdateVideo updates video information
func (h *AdminHandler) UpdateVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var req struct {
		Title               string `json:"title"`
		Description         string `json:"description"`
		IsPasswordProtected bool   `json:"is_password_protected"`
		Password            string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	err = h.videoService.UpdateVideo(
		uint(id),
		req.Title,
		req.Description,
		req.IsPasswordProtected,
		req.Password,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteVideo deletes a video
func (h *AdminHandler) DeleteVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	if err := h.videoService.DeleteVideo(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ImportVideos triggers video import from server directory
func (h *AdminHandler) ImportVideos(c *gin.Context) {
	result, err := h.importService.ScanAndImport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetSettings returns system settings
func (h *AdminHandler) GetSettings(c *gin.Context) {
	var setting models.Setting
	uploadPermission := models.UploadPermissionPublic
	if err := database.DB.Where("key = ?", models.SettingUploadPermission).First(&setting).Error; err == nil {
		uploadPermission = setting.Value
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_permission": uploadPermission,
	})
}

// UpdateSettings updates system settings
func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		UploadPermission string `json:"upload_permission"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validate permission value
	if req.UploadPermission != models.UploadPermissionPublic && req.UploadPermission != models.UploadPermissionAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permission value"})
		return
	}

	// Update or create setting
	var setting models.Setting
	result := database.DB.Where("key = ?", models.SettingUploadPermission).First(&setting)
	if result.Error != nil {
		// Create new setting
		setting = models.Setting{
			Key:   models.SettingUploadPermission,
			Value: req.UploadPermission,
		}
		database.DB.Create(&setting)
	} else {
		// Update existing setting
		database.DB.Model(&setting).Update("value", req.UploadPermission)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GenerateSubtitle generates subtitle for a video using Whisper API
func (h *AdminHandler) GenerateSubtitle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var video models.Video
	if err := database.DB.First(&video, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	// Generate subtitle in background (this might take a while)
	go func() {
		h.subtitleService.GenerateSubtitle(uint(id), video.OriginalPath)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Subtitle generation started. This may take a few minutes.",
	})
}

// UploadSubtitle handles subtitle file upload
func (h *AdminHandler) UploadSubtitle(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var video models.Video
	if err := database.DB.First(&video, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	file, err := c.FormFile("subtitle")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No subtitle file provided"})
		return
	}

	// Read file content
	openedFile, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer openedFile.Close()

	content := make([]byte, file.Size)
	_, err = openedFile.Read(content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	// Save subtitle
	subtitlePath, err := h.subtitleService.SaveUploadedSubtitle(video.Slug, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save subtitle"})
		return
	}

	// Update video record
	database.DB.Model(&video).Update("subtitle_path", subtitlePath)

	c.JSON(http.StatusOK, gin.H{"success": true, "subtitle_path": subtitlePath})
}
