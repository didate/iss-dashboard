package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminAuth checks the X-Admin-Token header.
func AdminAuth(adminToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminToken == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin token not configured"})
			return
		}
		token := c.GetHeader("X-Admin-Token")
		if token != adminToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin token"})
			return
		}
		c.Next()
	}
}

// DashboardAuth optionally protects read endpoints.
func DashboardAuth(isPublic bool, adminToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublic {
			c.Next()
			return
		}
		token := c.GetHeader("X-Admin-Token")
		if token != adminToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Next()
	}
}

// CORS middleware.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Admin-Token")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
