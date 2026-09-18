package api

import (
	"fmt"
	"log"
	"time"

	"iss-dashboard-backend/internal/config"
	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/store"
	syncer "iss-dashboard-backend/internal/sync"
	"iss-dashboard-backend/internal/webui"

	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config, st *store.Store, client *dhis2.Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), CORS(), SecurityHeaders())

	jwtSecret := cfg.AdminToken
	if jwtSecret == "" {
		log.Println("WARN: ADMIN_TOKEN not set, using insecure default JWT secret")
		jwtSecret = "iss-dashboard-change-me-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	api := r.Group("/iss/api")

	// Auth endpoints (public)
	auth := &AuthHandlers{Store: st, JWTSecret: jwtSecret}
	api.POST("/auth/login", LoginRateLimit(), auth.Login)

	// Carte sanitaire : espace public, toujours ouvert, projections réduites uniquement
	pub := api.Group("/public")
	{
		ph := &PublicHandlers{Store: st}
		pub.GET("/points.geojson", ph.GetPoints)
		pub.GET("/filters", ph.GetFilters)
		pub.GET("/structures", ph.SearchStructures)
		pub.GET("/annuaire", ph.GetAnnuaire)
		pub.GET("/structures.csv", ph.GetAnnuaireCSV)
		pub.GET("/structure/:uid", ph.GetStructure)
		pub.GET("/summary", ph.GetSummary)
	}

	normesH := &NormesHandlers{Store: st}

	// Public/protected read endpoints
	read := api.Group("")
	read.Use(DashboardAuth(cfg.DashboardPublic, jwtSecret, st))
	{
		h := &ReadHandlers{Store: st}
		read.GET("/summary", h.GetSummary)
		read.GET("/quality/summary", h.GetQualitySummary)
		read.GET("/quality/issues", h.GetQualityIssues)
		read.GET("/quality/event/:uid", h.GetEventDetail)
		read.GET("/usage/reporting", h.GetReportingRate)
		read.GET("/usage/closed-ous", h.GetClosedOUs)
		read.GET("/usage/recensement", h.GetUsageRecensement)
		read.GET("/usage/services", h.GetUsageServices)
		read.GET("/usage/equipements", h.GetUsageEquipements)
		read.GET("/usage/rh", h.GetUsageRH)
		read.GET("/usage/commodites", h.GetUsageCommodites)
		read.GET("/usage/plateau", h.GetPlateauTechnique)
		read.GET("/usage/services/matrix", h.GetServiceMatrix)
		read.GET("/usage/rh/summary", h.GetRHSummary)
		read.GET("/meta/filters", h.GetFilters)
		read.GET("/structures", h.GetStructuresList)
		read.GET("/compare", h.GetCompareDistricts)
		read.GET("/map/districts", h.GetMapData)

		gh := &GeoHandlers{Store: st}
		read.GET("/geo/coverage", gh.GetCoverage)
		read.GET("/geo/missing", gh.GetMissing)
		read.GET("/geo/missing.csv", gh.GetMissingCSV)
		read.GET("/usage/couverture", gh.GetCouverture)
		read.GET("/map/geo", gh.GetMapGeo)
		read.GET("/map/points", gh.GetMapPoints)

		read.GET("/meta/normes", normesH.Active)

		pdfH := &PDFHandlers{Store: st}
		read.GET("/export/pdf", pdfH.ExportDistrictPDF)
		read.GET("/export/pdf/structure/:uid", pdfH.ExportStructurePDF)
	}

	// Authenticated endpoints (JWT required)
	authenticated := api.Group("")
	authenticated.Use(JWTAuth(jwtSecret, st))
	{
		authenticated.GET("/auth/me", auth.GetMe)
		exp := &ExportHandlers{Store: st}
		authenticated.GET("/export/excel", exp.ExportExcel)
	}

	// Admin endpoints (JWT + admin role)
	admin := api.Group("/admin")
	admin.Use(JWTAuth(jwtSecret, st), RequireAdmin())
	{
		ah := &AdminHandlers{Store: st, Client: client, SyncOptions: syncer.OptionsFromConfig(cfg)}
		admin.POST("/sync", ah.TriggerSync)
		admin.GET("/sync/status", ah.GetSyncStatus)
		admin.GET("/users", auth.ListUsers)
		admin.POST("/users", auth.CreateUser)
		admin.DELETE("/users/:id", auth.DeleteUser)

		// Référentiel de normes (palier 2)
		admin.GET("/normes", normesH.List)
		admin.POST("/normes", normesH.Create)
		admin.GET("/normes/targets", normesH.Targets)
		admin.POST("/normes/recompute", normesH.RecomputeNow)
		admin.PUT("/normes/:id", normesH.Update)
		admin.DELETE("/normes/:id", normesH.Delete)
		admin.POST("/normes/:id/duplicate", normesH.Duplicate)
		admin.POST("/normes/:id/activate", normesH.Activate)
		admin.GET("/normes/:id/rules", normesH.GetRules)
		admin.PUT("/normes/:id/rules", normesH.PutRules)
		admin.POST("/normes/:id/rules/import", normesH.ImportRules)
		admin.GET("/normes/:id/rules/export.csv", normesH.ExportRules)
	}

	// Serve the embedded React SPA (built Vite output) under the same base path.
	// Must be registered last: it uses gin's NoRoute fallback for the UI while
	// leaving all API routes above untouched.
	webui.Register(r, "/iss")

	return r
}
