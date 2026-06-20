package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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

// LoginRateLimit limits login attempts per IP (max 5 per minute).
func LoginRateLimit() gin.HandlerFunc {
	type attempt struct {
		count    int
		resetAt  time.Time
	}
	var mu sync.Mutex
	attempts := make(map[string]*attempt)

	// Cleanup old entries every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			now := time.Now()
			for ip, a := range attempts {
				if now.After(a.resetAt) {
					delete(attempts, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		a, ok := attempts[ip]
		if !ok || time.Now().After(a.resetAt) {
			a = &attempt{count: 0, resetAt: time.Now().Add(1 * time.Minute)}
			attempts[ip] = a
		}
		a.count++
		count := a.count
		mu.Unlock()

		if count > 5 {
			log.Printf("WARN: rate limit exceeded for login from IP %s", ip)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "trop de tentatives, reessayez dans 1 minute"})
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
