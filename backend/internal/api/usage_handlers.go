package api

import (
	"net/http"

	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
)

type ReadHandlers struct {
	Store *store.Store
}

func (h *ReadHandlers) GetUsageRecensement(c *gin.Context) {
	by := c.DefaultQuery("by", "global")
	rows, err := h.Store.GetUsageRecensement(by)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetUsageServices(c *gin.Context) {
	district := c.Query("district")
	rows, err := h.Store.GetUsageServices(district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetUsageEquipements(c *gin.Context) {
	focus := c.DefaultQuery("focus", "all")
	district := c.Query("district")
	rows, err := h.Store.GetUsageEquipements(focus, district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetUsageRH(c *gin.Context) {
	district := c.Query("district")
	rows, err := h.Store.GetUsageRH(district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetUsageCommodites(c *gin.Context) {
	district := c.Query("district")
	rows, err := h.Store.GetUsageCommodites(district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetClosedOUs(c *gin.Context) {
	district := c.Query("district")
	rows, err := h.Store.GetClosedOUs(district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetReportingRate(c *gin.Context) {
	by := c.DefaultQuery("by", "global")
	rows, err := h.Store.GetReportingRate(by)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetPlateauTechnique(c *gin.Context) {
	district := c.Query("district")
	rows, err := h.Store.GetPlateauTechnique(district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetServiceMatrix(c *gin.Context) {
	rows, err := h.Store.GetServiceMatrix()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetRHSummary(c *gin.Context) {
	district := c.Query("district")
	result, err := h.Store.GetRHSummary(district)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReadHandlers) GetFilters(c *gin.Context) {
	filters, err := h.Store.GetFilters()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, filters)
}
