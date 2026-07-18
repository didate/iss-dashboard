package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// A registered API route, mimicking the real API group.
	r.GET("/iss/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"pong": true})
	})
	Register(r, "/iss")
	return r
}

func do(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestServesIndexAtBase(t *testing.T) {
	w := do(newTestRouter(), "/iss/")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /iss/ = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ISS Dashboard") {
		t.Fatalf("GET /iss/ body missing index.html marker: %q", w.Body.String())
	}
}

func TestSPAFallbackForClientRoute(t *testing.T) {
	// Unknown non-API path under base -> serve index.html so React routing works.
	w := do(newTestRouter(), "/iss/quality/some-uid")
	if w.Code != http.StatusOK {
		t.Fatalf("client route = %d, want 200 (SPA fallback)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ISS Dashboard") {
		t.Fatalf("SPA fallback did not return index.html")
	}
}

func TestUnknownAPIReturnsJSON404(t *testing.T) {
	w := do(newTestRouter(), "/iss/api/does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown API = %d, want 404", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unknown API content-type = %q, want json", ct)
	}
}

func TestRegisteredAPIStillWorks(t *testing.T) {
	w := do(newTestRouter(), "/iss/api/ping")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "pong") {
		t.Fatalf("registered API broke: code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestOutsideBaseIs404(t *testing.T) {
	w := do(newTestRouter(), "/somewhere-else")
	if w.Code != http.StatusNotFound {
		t.Fatalf("outside base = %d, want 404", w.Code)
	}
}
