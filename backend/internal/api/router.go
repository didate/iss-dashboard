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

	api := r.Group("/iss/api")

	// Public/protected read endpoints
	read := api.Group("")
	read.Use(DashboardAuth(cfg.DashboardPublic, cfg.AdminToken))
	{
		h := &ReadHandlers{Store: st}
		read.GET("/summary", h.GetSummary)
		read.GET("/quality/summary", h.GetQualitySummary)
		read.GET("/quality/issues", h.GetQualityIssues)
		read.GET("/quality/event/:uid", h.GetEventDetail)
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

	// Admin endpoints
	admin := api.Group("/admin")
	admin.Use(AdminAuth(cfg.AdminToken))
	{
		ah := &AdminHandlers{Store: st, Client: client}
		admin.POST("/sync", ah.TriggerSync)
		admin.GET("/sync/status", ah.GetSyncStatus)
	}

	return r
}
