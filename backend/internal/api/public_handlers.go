package api

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"

	"iss-dashboard-backend/internal/store"
	"iss-dashboard-backend/internal/usage"

	"github.com/gin-gonic/gin"
)

// PublicHandlers serve the always-open, reduced projections of the carte
// sanitaire. They read pre-computed blobs and the structure_latest view only;
// nothing here may join RH, equipment, responsible-person or quality data.
type PublicHandlers struct {
	Store           *store.Store
	DashboardPublic bool
}

const publicCacheControl = "public, max-age=3600"

// serveBlob writes a pre-serialized snapshot with ETag / 304 handling and gzip
// when the client accepts it (the points GeoJSON is ~1 MB uncompressed).
func (h *PublicHandlers) serveBlob(c *gin.Context, key string) {
	blob, err := h.Store.GetSnapshotBlob(key)
	if err != nil {
		internalError(c, err)
		return
	}
	if blob == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "aucune synchronisation n'a encore été effectuée"})
		return
	}
	c.Header("ETag", blob.ETag)
	c.Header("Cache-Control", publicCacheControl)
	c.Header("X-Built-At", blob.BuiltAt)
	c.Header("Vary", "Accept-Encoding")
	if match := c.GetHeader("If-None-Match"); match != "" && strings.Contains(match, blob.ETag) {
		c.Status(http.StatusNotModified)
		return
	}
	if strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(blob.JSON); err == nil && zw.Close() == nil {
			c.Header("Content-Encoding", "gzip")
			c.Data(http.StatusOK, "application/json; charset=utf-8", buf.Bytes())
			return
		}
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", blob.JSON)
}

// GET /public/points.geojson
func (h *PublicHandlers) GetPoints(c *gin.Context) { h.serveBlob(c, usage.BlobPublicPoints) }

// GET /public/filters
func (h *PublicHandlers) GetFilters(c *gin.Context) { h.serveBlob(c, usage.BlobPublicFilters) }

func publicParams(c *gin.Context) store.PublicSearchParams {
	return store.PublicSearchParams{
		Search:         strings.TrimSpace(c.Query("search")),
		Type:           c.Query("type"),
		Service:        c.Query("service"),
		District:       c.Query("district"),
		Region:         c.Query("region"),
		SousPrefecture: c.Query("sous_prefecture"),
	}
}

// GET /public/annuaire?search=&type=&service=&district=&region=&sous_prefecture=&page=&pageSize=
func (h *PublicHandlers) GetAnnuaire(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	res, err := h.Store.ListPublicStructures(publicParams(c), page, pageSize)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Cache-Control", publicCacheControl)
	c.JSON(http.StatusOK, res)
}

// GET /public/structures.csv?… — full registry with the same filters (open data).
func (h *PublicHandlers) GetAnnuaireCSV(c *gin.Context) {
	res, err := h.Store.ListPublicStructures(publicParams(c), 1, 0)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="structures_sanitaires.csv"`)
	c.Header("Cache-Control", publicCacheControl)
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(c.Writer)
	w.Comma = ';'
	w.Write([]string{"uid", "structure", "type", "statut_juridique", "statut_operationnel", "region", "district", "sous_prefecture", "latitude", "longitude", "nb_services_fonctionnels"})
	f := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', 6, 64)
	}
	for _, r := range res.Data {
		w.Write([]string{r.UID, r.Name, r.TypeLabel, r.StatutJuri, r.StatutOp, r.Region, r.District, r.SousPrefecture, f(r.Lat), f(r.Lng), strconv.Itoa(r.NServices)})
	}
	w.Flush()
}

// GET /public/structures?search=&type=&service=&district=&region=&near=lat,lng&radius_km=&limit=
func (h *PublicHandlers) SearchStructures(c *gin.Context) {
	p := publicParams(c)
	p.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	p.RadiusKm, _ = strconv.ParseFloat(c.DefaultQuery("radius_km", "0"), 64)
	if near := c.Query("near"); near != "" {
		lat, lng, ok := parseLatLng(near)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "near doit être de la forme lat,lng"})
			return
		}
		p.Lat, p.Lng = &lat, &lng
	}
	items, err := h.Store.SearchPublicStructures(p)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Cache-Control", publicCacheControl)
	c.JSON(http.StatusOK, items)
}

// GET /public/structure/:uid  (uid = org unit uid)
func (h *PublicHandlers) GetStructure(c *gin.Context) {
	ps, err := h.Store.GetPublicStructure(c.Param("uid"))
	if err != nil {
		internalError(c, err)
		return
	}
	if ps == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "structure inconnue"})
		return
	}
	c.Header("Cache-Control", publicCacheControl)
	c.JSON(http.StatusOK, ps)
}

// GET /public/summary
func (h *PublicHandlers) GetSummary(c *gin.Context) {
	sum, err := h.Store.GetPublicSummary()
	if err != nil {
		internalError(c, err)
		return
	}
	sum.DashboardPublic = h.DashboardPublic
	c.Header("Cache-Control", publicCacheControl)
	c.JSON(http.StatusOK, sum)
}

func parseLatLng(s string) (lat, lng float64, ok bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}
