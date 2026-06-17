package api

import (
	"net/http"
	"strconv"

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

func (h *ReadHandlers) GetCompareDistricts(c *gin.Context) {
	d1 := c.Query("district1")
	d2 := c.Query("district2")
	if d1 == "" || d2 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "district1 and district2 required"})
		return
	}
	result, err := h.Store.GetCompareData(d1, d2)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReadHandlers) GetStructuresList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	params := store.StructureListParams{
		District: c.Query("district"),
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	}
	result, err := h.Store.GetStructuresList(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReadHandlers) GetMapData(c *gin.Context) {
	data, err := h.Store.GetMapData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
