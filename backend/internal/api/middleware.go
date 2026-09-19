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
// hasValidJWT reports whether the request carries a valid bearer token.
func hasValidJWT(c *gin.Context, jwtSecret string) bool {
	auth := c.GetHeader("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	token, err := jwt.Parse(strings.TrimPrefix(auth, "Bearer "), func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	return err == nil && token.Valid
}

// IsAuthenticated is true when DashboardAuth saw a valid token — even in public
// mode, where it lets read handlers hide personal data from anonymous readers.
func IsAuthenticated(c *gin.Context) bool {
	v, _ := c.Get("authenticated")
	b, _ := v.(bool)
	return b
}

func DashboardAuth(isPublic bool, jwtSecret string, st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		authenticated := hasValidJWT(c, jwtSecret)
		c.Set("authenticated", authenticated)
		if isPublic || authenticated {
			c.Next()
			return
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
		count   int
		resetAt time.Time
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
	isProduction := os.Getenv("GIN_MODE") == "release" || os.Getenv("ENV") == "production"
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		if isProduction {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
