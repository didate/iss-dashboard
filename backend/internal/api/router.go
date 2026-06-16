package api

import (
	"iss-dashboard-backend/internal/config"
	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config, st *store.Store, client *dhis2.Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), CORS())

	jwtSecret := cfg.AdminToken
	if jwtSecret == "" {
		jwtSecret = "iss-dashboard-secret"
	}

	api := r.Group("/iss/api")

	// Auth endpoints (public)
	auth := &AuthHandlers{Store: st, JWTSecret: jwtSecret}
	api.POST("/auth/login", auth.Login)

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
		read.GET("/usage/recensement", h.GetUsageRecensement)
		read.GET("/usage/services", h.GetUsageServices)
		read.GET("/usage/equipements", h.GetUsageEquipements)
		read.GET("/usage/rh", h.GetUsageRH)
		read.GET("/usage/commodites", h.GetUsageCommodites)
		read.GET("/usage/plateau", h.GetPlateauTechnique)
		read.GET("/usage/services/matrix", h.GetServiceMatrix)
		read.GET("/usage/rh/summary", h.GetRHSummary)
		read.GET("/meta/filters", h.GetFilters)
	}

	// Authenticated endpoints (JWT required)
	authenticated := api.Group("")
	authenticated.Use(JWTAuth(jwtSecret, st))
	{
		authenticated.GET("/auth/me", auth.GetMe)
		authenticated.GET("/export/excel", h_exportPlaceholder)
	}

	// Admin endpoints (JWT + admin role)
	admin := api.Group("/admin")
	admin.Use(JWTAuth(jwtSecret, st), RequireAdmin())
	{
		ah := &AdminHandlers{Store: st, Client: client}
		admin.POST("/sync", ah.TriggerSync)
		admin.GET("/sync/status", ah.GetSyncStatus)
		admin.GET("/users", auth.ListUsers)
		admin.POST("/users", auth.CreateUser)
		admin.DELETE("/users/:id", auth.DeleteUser)
	}

	return r
}

func h_exportPlaceholder(c *gin.Context) {
	c.JSON(200, gin.H{"message": "use scripts/export_excel.py for now"})
}
