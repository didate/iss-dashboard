// Package webui serves the compiled React single-page application (Vite build)
// embedded directly into the Go binary. This lets the backend ship the frontend
// itself, so a single container/image serves both the API and the UI.
//
// The real assets are injected at build time by copying the Vite `dist/` output
// into the `dist/` directory of this package before `go build` (see the root
// Dockerfile). A placeholder `dist/index.html` is committed so that a plain
// `go build` (without a frontend build) still compiles.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// Register mounts the embedded SPA under basePath (e.g. "/iss") on the given
// gin engine. It must be called AFTER all real routes (API, auth, admin) are
// registered, because it relies on gin's NoRoute handler:
//
//   - Requests matching a registered route (the API) are served normally.
//   - Requests for an embedded asset (e.g. /iss/assets/app.js) are served from
//     the embedded filesystem.
//   - Any other GET under basePath (client-side routes like /iss/quality) falls
//     back to index.html so the React router can take over.
//   - Unknown API paths still return a JSON 404 instead of the HTML shell.
func Register(r *gin.Engine, basePath string) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("webui: cannot open embedded dist: " + err.Error())
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("webui: cannot read embedded index.html: " + err.Error())
	}
	fileServer := http.StripPrefix(basePath, http.FileServer(http.FS(sub)))
	apiPrefix := basePath + "/api"

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path

		// Never serve the SPA shell for API paths: keep JSON 404 semantics.
		if p == apiPrefix || strings.HasPrefix(p, apiPrefix+"/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Anything outside the app's base path is genuinely not found.
		if p != basePath && !strings.HasPrefix(p, basePath+"/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		rel := strings.TrimPrefix(strings.TrimPrefix(p, basePath), "/")

		// SPA fallback: for the base path itself or any path that is not a real
		// embedded asset, serve index.html directly so client-side routing
		// resolves it. Writing the bytes avoids http.FileServer's redirect of
		// "/index.html" -> "/".
		if rel == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", index)
			return
		}
		if _, statErr := fs.Stat(sub, rel); statErr != nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", index)
			return
		}

		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
