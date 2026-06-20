package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
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
		auth := c.GetHeader("Authorization")
		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
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

// CORS middleware with configurable origin.
func CORS() gin.HandlerFunc {
	allowedOrigin := os.Getenv("CORS_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "*"
		log.Println("WARN: CORS_ORIGIN not set, allowing all origins")
	}
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// internalError logs the real error and returns a generic message to the client.
func internalError(c *gin.Context, err error) {
	log.Printf("ERROR: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur serveur"})
}

// SecurityHeaders adds common security headers.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}
