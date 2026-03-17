package handler

import (
	"net/http"
	"path/filepath"

	"github.com/adamscao/videoshare/internal/config"
	"github.com/adamscao/videoshare/internal/middleware"
	"github.com/adamscao/videoshare/internal/service"
	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	config       *config.Config
	videoService *service.VideoService
}

func NewVideoHandler(cfg *config.Config, videoService *service.VideoService) *VideoHandler {
	return &VideoHandler{
		config:       cfg,
		videoService: videoService,
	}
}

// ShowVideoPage shows video watch page
func (h *VideoHandler) ShowVideoPage(c *gin.Context) {
	slug := c.Param("slug")

	video, err := h.videoService.GetVideoBySlug(slug)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"message": "Video not found",
		})
		return
	}

	// Check if password is required and verified
	if video.IsPasswordProtected && !h.isVideoUnlocked(c, video.Slug) {
		c.HTML(http.StatusOK, "password.html", gin.H{
			"slug": video.Slug,
		})
		return
	}

	status := video.Status
	if status == "" {
		status = "ready"
	}

	c.HTML(http.StatusOK, "watch.html", gin.H{
		"video":          video,
		"slug":           video.Slug,
		"subtitleConfig": h.config.Subtitle,
		"videoReady":     status == "ready",
		"videoFailed":    status == "failed",
	})
}

// GetVideoStatus returns the transcoding status of a video.
func (h *VideoHandler) GetVideoStatus(c *gin.Context) {
	slug := c.Param("slug")

	video, err := h.videoService.GetVideoBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	status := video.Status
	if status == "" {
		status = "ready"
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"error":  video.TranscodeError,
	})
}

// GetVideoInfo returns video information
func (h *VideoHandler) GetVideoInfo(c *gin.Context) {
	slug := c.Param("slug")

	video, err := h.videoService.GetVideoBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	// Check if password is required
	if video.IsPasswordProtected && !h.isVideoUnlocked(c, video.Slug) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":              "Password required",
			"password_protected": true,
		})
		return
	}

	c.JSON(http.StatusOK, video)
}

// VerifyVideoPassword verifies video password
func (h *VideoHandler) VerifyVideoPassword(c *gin.Context) {
	slug := c.Param("slug")

	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	video, err := h.videoService.GetVideoBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	if !h.videoService.VerifyPassword(video, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// Save to session - use individual key for each video
	session, _ := middleware.VideoSessionStore.Get(c.Request, middleware.VideoSessionName)
	session.Values["video_"+slug] = true
	if err := session.Save(c.Request, c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ServeHLSPlaylist serves m3u8 playlist
func (h *VideoHandler) ServeHLSPlaylist(c *gin.Context) {
	slug := c.Param("slug")

	video, err := h.videoService.GetVideoBySlug(slug)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// Check access permission
	if video.IsPasswordProtected && !h.isVideoUnlocked(c, video.Slug) {
		c.Status(http.StatusForbidden)
		return
	}

	c.File(video.HLSPath)
}

// ServeHLSSegment serves HLS segment files
func (h *VideoHandler) ServeHLSSegment(c *gin.Context) {
	slug := c.Param("slug")
	segment := c.Param("segment")

	video, err := h.videoService.GetVideoBySlug(slug)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// Check access permission
	if video.IsPasswordProtected && !h.isVideoUnlocked(c, video.Slug) {
		c.Status(http.StatusForbidden)
		return
	}

	segmentPath := filepath.Join(filepath.Dir(video.HLSPath), segment)
	c.File(segmentPath)
}

func (h *VideoHandler) isVideoUnlocked(c *gin.Context, slug string) bool {
	session, err := middleware.VideoSessionStore.Get(c.Request, middleware.VideoSessionName)
	if err != nil {
		return false
	}

	unlocked, ok := session.Values["video_"+slug].(bool)
	return ok && unlocked
}
