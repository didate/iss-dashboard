package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func signed(secret string) string {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": "viewer", "exp": time.Now().Add(time.Hour).Unix()})
	s, _ := t.SignedString([]byte(secret))
	return s
}

func run(isPublic bool, authHeader string) (int, bool) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var authed bool
	r.GET("/x", DashboardAuth(isPublic, "s3cret", nil), func(c *gin.Context) {
		authed = IsAuthenticated(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/x", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, authed
}

func TestDashboardAuth_PublicMode(t *testing.T) {
	if code, authed := run(true, ""); code != 200 || authed {
		t.Fatalf("anonymous in public mode: code=%d authed=%v", code, authed)
	}
	if code, authed := run(true, "Bearer "+signed("s3cret")); code != 200 || !authed {
		t.Fatalf("valid token in public mode must be flagged authenticated: code=%d authed=%v", code, authed)
	}
	if code, authed := run(true, "Bearer "+signed("wrong")); code != 200 || authed {
		t.Fatalf("bad token in public mode: still readable, but anonymous: code=%d authed=%v", code, authed)
	}
}

func TestDashboardAuth_ProtectedMode(t *testing.T) {
	if code, _ := run(false, ""); code != 401 {
		t.Fatalf("anonymous must be refused: %d", code)
	}
	if code, authed := run(false, "Bearer "+signed("s3cret")); code != 200 || !authed {
		t.Fatalf("valid token: code=%d authed=%v", code, authed)
	}
}
