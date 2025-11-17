package handler

import (
	"net/http"

	"github.com/adamscao/videoshare/internal/middleware"
	"github.com/adamscao/videoshare/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// ShowLoginPage shows admin login page
func (h *AuthHandler) ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

// Login handles admin login
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	admin, err := h.authService.AuthenticateAdmin(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Set session
	session, _ := middleware.AdminSessionStore.Get(c.Request, middleware.AdminSessionName)
	session.Values["admin_id"] = admin.ID
	session.Save(c.Request, c.Writer)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Logout handles admin logout
func (h *AuthHandler) Logout(c *gin.Context) {
	session, _ := middleware.AdminSessionStore.Get(c.Request, middleware.AdminSessionName)
	session.Values["admin_id"] = nil
	session.Options.MaxAge = -1
	session.Save(c.Request, c.Writer)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
