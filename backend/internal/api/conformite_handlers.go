package api

import (
	"net/http"
	"strconv"

	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
)

// ConformiteHandlers expose the pre-computed conformity to the planners' space.
type ConformiteHandlers struct {
	Store *store.Store
}

// GET /conformite/summary?by=global|region|district|sous_prefecture|type&type=
func (h *ConformiteHandlers) GetSummary(c *gin.Context) {
	rows, err := h.Store.GetConformiteSummary(c.DefaultQuery("by", "district"), c.Query("type"))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// GET /conformite/gaps?by=district&key=&type=&kind=&level=&limit=
func (h *ConformiteHandlers) GetGaps(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	rows, err := h.Store.GetConformiteGaps(store.GapParams{
		Dimension: c.DefaultQuery("by", "global"),
		Key:       c.Query("key"),
		TypeCode:  c.Query("type"),
		Kind:      c.Query("kind"),
		Level:     c.Query("level"),
		Limit:     limit,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// GET /conformite/structures?region=&district=&sous_prefecture=&type=&status=&search=&page=&pageSize=
func (h *ConformiteHandlers) GetStructures(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "25"))
	res, err := h.Store.GetConformiteStructures(store.ConformiteStructureParams{
		Region: c.Query("region"), District: c.Query("district"), SousPrefecture: c.Query("sous_prefecture"),
		TypeCode: c.Query("type"), Status: c.Query("status"), Search: c.Query("search"),
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
