package api

import (
	"net/http"
	"strings"

	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// DashboardAuth optionally protects read endpoints.
func DashboardAuth(isPublic bool, jwtSecret string, st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublic {
			c.Next()
			return
		}
		// Try JWT
		auth := c.GetHeader("Authorization")
		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})
			if err == nil && token.Valid {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentification requise"})
	}
}

// CORS middleware.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
