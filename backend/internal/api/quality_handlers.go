package api

import (
	"net/http"
	"strconv"

	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
)

func (h *ReadHandlers) GetQualitySummary(c *gin.Context) {
	by := c.DefaultQuery("by", "global")
	rows, err := h.Store.GetQualitySummary(by)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (h *ReadHandlers) GetQualityIssues(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	params := store.IssueListParams{
		Severity: c.Query("severity"),
		Rule:     c.Query("rule"),
		District: c.Query("district"),
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.Store.GetQualityIssues(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReadHandlers) GetEventDetail(c *gin.Context) {
	uid := c.Param("uid")
	detail, err := h.Store.GetEventDetail(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	c.JSON(http.StatusOK, detail)
}
