package api

import (
	"net/http"

	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/store"
	syncer "iss-dashboard-backend/internal/sync"

	"github.com/gin-gonic/gin"
)

type AdminHandlers struct {
	Store  *store.Store
	Client *dhis2.Client
}

func (h *AdminHandlers) TriggerSync(c *gin.Context) {
	if syncer.IsRunning() {
		c.JSON(http.StatusConflict, gin.H{"error": "sync already in progress"})
		return
	}

	// Run sync in background
	go func() {
		syncer.RunSync(h.Store, h.Client)
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "running", "message": "sync started"})
}

func (h *AdminHandlers) GetSyncStatus(c *gin.Context) {
	current, _ := h.Store.GetRunningSyncRun()
	last, _ := h.Store.GetLastSyncRun()
	history, _ := h.Store.GetSyncRunHistory(10)

	c.JSON(http.StatusOK, gin.H{
		"current": current,
		"last":    last,
		"history": history,
	})
}
