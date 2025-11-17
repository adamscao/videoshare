package handler

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/database"
	"github.com/adamscao/videoshare/internal/middleware"
	"github.com/adamscao/videoshare/internal/models"
	"github.com/adamscao/videoshare/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	config       *config.Config
	videoService *service.VideoService
}

func NewUploadHandler(cfg *config.Config, videoService *service.VideoService) *UploadHandler {
	return &UploadHandler{
		config:       cfg,
		videoService: videoService,
	}
}

// ShowUploadPage shows upload page
func (h *UploadHandler) ShowUploadPage(c *gin.Context) {
	// Check upload permission
	if !h.checkUploadPermission(c) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"message": "Upload is restricted to administrators only",
		})
		return
	}

	c.HTML(http.StatusOK, "upload.html", nil)
}

// UploadVideo handles video upload
func (h *UploadHandler) UploadVideo(c *gin.Context) {
	// Check upload permission
	if !h.checkUploadPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Upload permission denied"})
		return
	}

	// Parse form data
	file, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No video file provided"})
		return
	}

	// Check file size
	if file.Size > h.config.Upload.MaxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large"})
		return
	}

	title := c.PostForm("title")
	if title == "" {
		title = file.Filename
	}
	description := c.PostForm("description")

	// Determine upload type
	uploadType := "web"
	session, _ := middleware.AdminSessionStore.Get(c.Request, middleware.AdminSessionName)
	if adminID, ok := session.Values["admin_id"]; ok && adminID != nil {
		uploadType = "admin"
	}

	// Generate unique filename and save file
	ext := filepath.Ext(file.Filename)
	uniqueFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	savePath := filepath.Join(h.config.Storage.OriginalsDir, uniqueFilename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Create video record and convert to HLS
	video, err := h.videoService.CreateVideo(
		savePath,
		file.Filename,
		title,
		description,
		uploadType,
		false, // For now, don't support password on upload
		"",
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process video: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"slug": video.Slug,
		"url":  "/v/" + video.Slug,
	})
}

func (h *UploadHandler) checkUploadPermission(c *gin.Context) bool {
	// Check if admin is logged in
	session, err := middleware.AdminSessionStore.Get(c.Request, middleware.AdminSessionName)
	if err == nil {
		if adminID, ok := session.Values["admin_id"]; ok && adminID != nil {
			return true
		}
	}

	// Check system setting
	var setting models.Setting
	if err := database.DB.Where("key = ?", models.SettingUploadPermission).First(&setting).Error; err != nil {
		return true // Default to public if setting not found
	}

	return setting.Value == models.UploadPermissionPublic
}
