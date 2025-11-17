package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthRequired middleware checks if admin is authenticated
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, err := AdminSessionStore.Get(c.Request, AdminSessionName)
		if err != nil {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		adminID, ok := session.Values["admin_id"]
		if !ok || adminID == nil {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		c.Set("admin_id", adminID)
		c.Next()
	}
}

// CheckUploadPermission middleware checks upload permission
func CheckUploadPermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		// If admin is logged in, allow upload
		session, err := AdminSessionStore.Get(c.Request, AdminSessionName)
		if err == nil {
			if adminID, ok := session.Values["admin_id"]; ok && adminID != nil {
				c.Next()
				return
			}
		}

		// Check system upload permission setting
		// This will be checked in handler
		c.Next()
	}
}
