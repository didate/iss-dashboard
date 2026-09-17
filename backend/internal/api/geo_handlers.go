package api

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
)

// GeoHandlers serve the planners' geographic coverage screens (behind DashboardAuth).
type GeoHandlers struct {
	Store *store.Store
}

// GET /geo/coverage?level=3|4&region=
func (h *GeoHandlers) GetCoverage(c *gin.Context) {
	level, _ := strconv.Atoi(c.DefaultQuery("level", "3"))
	rows, err := h.Store.GetGeoCoverage(level, c.Query("region"))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

func missingParams(c *gin.Context) store.MissingGPSParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	return store.MissingGPSParams{
		District: c.Query("district"),
		Region:   c.Query("region"),
		Type:     c.Query("type"),
		Page:     page,
		PageSize: pageSize,
	}
}

// GET /geo/missing?district=&region=&type=&page=&pageSize=
func (h *GeoHandlers) GetMissing(c *gin.Context) {
	res, err := h.Store.GetMissingGPS(missingParams(c))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /geo/missing.csv?district=&region=&type=  — full list for field teams.
func (h *GeoHandlers) GetMissingCSV(c *gin.Context) {
	p := missingParams(c)
	p.PageSize = 0
	res, err := h.Store.GetMissingGPS(p)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="structures_sans_gps.csv"`)
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM so Excel opens UTF-8 correctly
	w := csv.NewWriter(c.Writer)
	w.Comma = ';'
	w.Write([]string{"uid_dhis2", "structure", "type", "region", "district", "sous_prefecture", "date_recensement"})
	for _, it := range res.Data {
		w.Write([]string{it.OrgUnitUID, it.Name, it.TypeLabel, it.Region, it.District, it.SousPrefecture, it.EventDate})
	}
	w.Flush()
}

// GET /usage/couverture?by=global|region|district|sous_prefecture&indicator=
func (h *GeoHandlers) GetCouverture(c *gin.Context) {
	rows, err := h.Store.GetCouverture(c.DefaultQuery("by", "district"), c.Query("indicator"))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// GET /map/geo?level=3|4
func (h *GeoHandlers) GetMapGeo(c *gin.Context) {
	level, _ := strconv.Atoi(c.DefaultQuery("level", "3"))
	fc, err := h.Store.GetMapGeo(level)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, fc)
}
