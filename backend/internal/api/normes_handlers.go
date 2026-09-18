package api

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/normes"
	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
)

// NormesHandlers administer the norms referential (admin) and expose the active set (read).
type NormesHandlers struct {
	Store *store.Store
	// Recompute is called after activation / rule changes on the active set (wired in lot B).
	Recompute func() error
}

func currentUsername(c *gin.Context) string {
	if u, ok := c.Get("user"); ok {
		if user, ok := u.(*models.User); ok {
			return user.Username
		}
	}
	return ""
}

func (h *NormesHandlers) setFromPath(c *gin.Context) (*models.NormeSet, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return nil, false
	}
	ns, err := h.Store.GetNormeSet(id)
	if err != nil {
		internalError(c, err)
		return nil, false
	}
	if ns == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "référentiel introuvable"})
		return nil, false
	}
	return ns, true
}

// recomputeIfActive re-evaluates conformity when the modified set is the active one.
func (h *NormesHandlers) recomputeIfActive(ns *models.NormeSet) {
	if ns.Status != "active" || h.Recompute == nil {
		return
	}
	if err := h.Recompute(); err != nil {
		log.Printf("[NORMES] WARN: recompute after change on set %d: %v", ns.ID, err)
	}
}

// GET /admin/normes
func (h *NormesHandlers) List(c *gin.Context) {
	sets, err := h.Store.ListNormeSets()
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, sets)
}

// POST /admin/normes  {name, notes}
func (h *NormesHandlers) Create(c *gin.Context) {
	var req struct {
		Name  string `json:"name"`
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name requis"})
		return
	}
	ns, err := h.Store.CreateNormeSet(req.Name, req.Notes, currentUsername(c))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, ns)
}

// PUT /admin/normes/:id  {name, notes}
func (h *NormesHandlers) Update(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name requis"})
		return
	}
	if err := h.Store.UpdateNormeSet(ns.ID, req.Name, req.Notes); err != nil {
		internalError(c, err)
		return
	}
	updated, _ := h.Store.GetNormeSet(ns.ID)
	c.JSON(http.StatusOK, updated)
}

// POST /admin/normes/:id/duplicate
func (h *NormesHandlers) Duplicate(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	dup, err := h.Store.DuplicateNormeSet(ns.ID, currentUsername(c))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dup)
}

// POST /admin/normes/:id/activate
func (h *NormesHandlers) Activate(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	if err := h.Store.ActivateNormeSet(ns.ID); err != nil {
		internalError(c, err)
		return
	}
	log.Printf("[AUDIT] Référentiel de normes %d (v%d) activé par %s", ns.ID, ns.Version, currentUsername(c))
	activated, _ := h.Store.GetNormeSet(ns.ID)
	h.recomputeIfActive(activated)
	c.JSON(http.StatusOK, activated)
}

// DELETE /admin/normes/:id  (draft only)
func (h *NormesHandlers) Delete(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	if err := h.Store.DeleteNormeSet(ns.ID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": ns.ID})
}

// GET /admin/normes/:id/rules
func (h *NormesHandlers) GetRules(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	rules, err := h.Store.GetNormeRules(ns.ID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

// PUT /admin/normes/:id/rules  [rules] — replaces every rule; each one is validated.
func (h *NormesHandlers) PutRules(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	var rules []models.NormeRule
	if err := c.ShouldBindJSON(&rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tableau de règles attendu"})
		return
	}
	catalog, err := h.Store.BuildNormeCatalog()
	if err != nil {
		internalError(c, err)
		return
	}
	var errs []normes.LineError
	for i := range rules {
		if msg := normes.Validate(&rules[i], catalog); msg != "" {
			errs = append(errs, normes.LineError{Line: i + 1, Message: msg})
		}
	}
	if len(errs) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "règles invalides", "errors": errs})
		return
	}
	if err := h.Store.ReplaceNormeRules(ns.ID, rules); err != nil {
		internalError(c, err)
		return
	}
	h.recomputeIfActive(ns)
	saved, _ := h.Store.GetNormeRules(ns.ID)
	c.JSON(http.StatusOK, saved)
}

// POST /admin/normes/:id/rules/import  (multipart "file" or raw CSV body) ?mode=replace|append
func (h *NormesHandlers) ImportRules(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	var body io.Reader = c.Request.Body
	if f, _, err := c.Request.FormFile("file"); err == nil {
		defer f.Close()
		body = f
	}
	raw, err := io.ReadAll(io.LimitReader(body, 2<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lecture du fichier impossible"})
		return
	}
	catalog, err := h.Store.BuildNormeCatalog()
	if err != nil {
		internalError(c, err)
		return
	}
	rules, lineErrs := normes.ParseCSV(bytes.NewReader(raw), catalog)
	if len(lineErrs) > 0 && c.DefaultQuery("strict", "true") == "true" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("%d ligne(s) invalide(s), rien n'a été importé", len(lineErrs)), "errors": lineErrs, "valid": len(rules)})
		return
	}
	if c.DefaultQuery("mode", "replace") == "append" {
		existing, err := h.Store.GetNormeRules(ns.ID)
		if err != nil {
			internalError(c, err)
			return
		}
		rules = append(existing, rules...)
	}
	if err := h.Store.ReplaceNormeRules(ns.ID, rules); err != nil {
		internalError(c, err)
		return
	}
	h.recomputeIfActive(ns)
	saved, _ := h.Store.GetNormeRules(ns.ID)
	c.JSON(http.StatusOK, gin.H{"imported": len(rules), "total": len(saved), "errors": lineErrs})
}

// GET /admin/normes/:id/rules/export.csv
func (h *NormesHandlers) ExportRules(c *gin.Context) {
	ns, ok := h.setFromPath(c)
	if !ok {
		return
	}
	rules, err := h.Store.GetNormeRules(ns.ID)
	if err != nil {
		internalError(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="normes_v%d.csv"`, ns.Version))
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	if err := normes.WriteCSV(c.Writer, rules); err != nil {
		internalError(c, err)
	}
}

// GET /admin/normes/targets — catalogue des cibles admissibles
func (h *NormesHandlers) Targets(c *gin.Context) {
	catalog, err := h.Store.BuildNormeCatalog()
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"targets": catalog.Sorted(), "kinds": normes.Kinds, "levels": []string{normes.LevelEssentiel, normes.LevelRecommande}})
}

// POST /admin/normes/recompute
func (h *NormesHandlers) RecomputeNow(c *gin.Context) {
	if h.Recompute == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "évaluation non disponible"})
		return
	}
	if err := h.Recompute(); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /meta/normes — active referential for display (read scope)
func (h *NormesHandlers) Active(c *gin.Context) {
	ns, err := h.Store.GetActiveNormeSet()
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"active": ns})
}
