package api

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"iss-dashboard-backend/internal/drh"
	"iss-dashboard-backend/internal/store"
	syncer "iss-dashboard-backend/internal/sync"

	"github.com/gin-gonic/gin"
)

// maxDrhUpload bounds the personnel file: the 2026 millésime is ~1.4 MB for
// 10 000 agents, so 32 MB leaves a decade of growth.
const maxDrhUpload = 32 << 20

// DrhHandlers administer the civil-service personnel file (import, millésimes,
// correspondence table). Read endpoints come with the planning screens.
type DrhHandlers struct {
	Store       *store.Store
	AgeRetraite int
}

// Import ingests one normalised CSV (docs/drh-format.md) and makes it the
// active millésime.
func (h *DrhHandlers) Import(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fichier manquant (champ \"file\")"})
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxDrhUpload))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lecture du fichier impossible"})
		return
	}
	raw, err = gunzipIfNeeded(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fichier .gz illisible : " + err.Error()})
		return
	}

	annee, _ := strconv.Atoi(c.PostForm("annee"))
	if annee <= 0 {
		annee = time.Now().Year()
	}
	label := strings.TrimSpace(c.PostForm("label"))
	if label == "" {
		label = fmt.Sprintf("DRH/CNPS %d", annee)
	}
	age := h.AgeRetraite
	if v, err := strconv.Atoi(c.PostForm("age_retraite")); err == nil && v > 0 {
		age = v
	}

	res, err := syncer.RunDrhImport(h.Store, bytes.NewReader(raw), syncer.DrhImportParams{
		Label:       label,
		Annee:       annee,
		AgeRetraite: age,
		SourceFile:  header.Filename,
		ImportedBy:  currentUsername(c),
	})
	if err != nil {
		status := http.StatusUnprocessableEntity
		body := gin.H{"error": err.Error()}
		if res != nil && len(res.Erreurs) > 0 {
			body["erreurs"] = res.Erreurs
		}
		c.JSON(status, body)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *DrhHandlers) ListImports(c *gin.Context) {
	imports, err := h.Store.ListDrhImports()
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"imports": imports})
}

func (h *DrhHandlers) Activate(c *gin.Context) {
	id, ok := drhIDFromPath(c)
	if !ok {
		return
	}
	if err := h.Store.ActivateDrhImport(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	im, _ := h.Store.GetDrhImport(id)
	c.JSON(http.StatusOK, gin.H{"import": im})
}

func (h *DrhHandlers) Delete(c *gin.Context) {
	id, ok := drhIDFromPath(c)
	if !ok {
		return
	}
	if err := h.Store.DeleteDrhImport(id); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// NonReconnus returns the labels the import could not attach — the list to send
// back to the DRH so the next millésime needs less arbitration.
func (h *DrhHandlers) NonReconnus(c *gin.Context) {
	id, ok := drhIDFromPath(c)
	if !ok {
		return
	}
	list, err := h.Store.GetDrhNonReconnus(id)
	if err != nil {
		internalError(c, err)
		return
	}
	if !strings.HasSuffix(c.Request.URL.Path, ".csv") {
		c.JSON(http.StatusOK, gin.H{"non_reconnus": list})
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="drh_non_reconnus_%d.csv"`, id))
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(c.Writer)
	cw.Comma = ';'
	cw.Write([]string{"libelle_drh", "prefecture", "n_agents"})
	for _, u := range list {
		cw.Write([]string{u.Libelle, u.Prefecture, strconv.Itoa(u.NAgents)})
	}
	cw.Flush()
}

func (h *DrhHandlers) ListCorrespondances(c *gin.Context) {
	corr, err := h.Store.ListDrhCorrespondances()
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"correspondances": corr, "statuts": []string{
		drh.CorrOK, drh.CorrBureau, drh.CorrNonRattache, drh.CorrATrancher}})
}

// ImportCorrespondances replaces the whole correspondence table from a CSV.
// It does not re-run the ingestion: the admin re-imports the millésime to see
// the effect, which keeps the two operations auditable separately.
func (h *DrhHandlers) ImportCorrespondances(c *gin.Context) {
	var body io.Reader = c.Request.Body
	if f, _, err := c.Request.FormFile("file"); err == nil {
		defer f.Close()
		body = f
	}
	raw, err := io.ReadAll(io.LimitReader(body, 4<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lecture du fichier impossible"})
		return
	}
	if raw, err = gunzipIfNeeded(raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fichier .gz illisible : " + err.Error()})
		return
	}
	corr, lineErrs := drh.ParseCorrespondancesCSV(bytes.NewReader(raw))
	if len(lineErrs) > 0 && c.DefaultQuery("strict", "true") == "true" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   fmt.Sprintf("%d ligne(s) invalide(s), rien n'a été importé", len(lineErrs)),
			"erreurs": lineErrs, "valides": len(corr)})
		return
	}
	if err := h.Store.ReplaceDrhCorrespondances(corr); err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"imported": len(corr), "erreurs": lineErrs})
}

func (h *DrhHandlers) ExportCorrespondances(c *gin.Context) {
	corr, err := h.Store.ListDrhCorrespondances()
	if err != nil {
		internalError(c, err)
		return
	}
	structures, err := h.Store.ListDrhStructures()
	if err != nil {
		internalError(c, err)
		return
	}
	nameOf := map[string]string{}
	for _, s := range structures {
		nameOf[s.UID] = s.Name
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="drh_correspondances.csv"`)
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	if err := drh.WriteCorrespondancesCSV(c.Writer, corr, func(uid string) string { return nameOf[uid] }); err != nil {
		internalError(c, err)
	}
}

// gunzipIfNeeded décompresse un fichier gzip, reconnu à ses octets d'en-tête.
//
// Le CSV annuel fait ~1,4 Mo, au-dessus de la limite d'envoi par défaut des
// reverse proxies (1 Mo pour nginx), qui rejettent alors la requête en 413
// avant même qu'elle atteigne l'application. Gzippé il tombe à ~70 Ko : accepter
// le .csv.gz évite de dépendre de la configuration du proxy.
func gunzipIfNeeded(raw []byte) ([]byte, error) {
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		return raw, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(io.LimitReader(zr, maxDrhUpload))
}

func drhIDFromPath(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id invalide"})
		return 0, false
	}
	return id, true
}

// --- Lecture (espace planification) -----------------------------------------

// DrhReadHandlers serve the pre-computed personnel aggregates. Nothing here is
// exposed to the public space.
type DrhReadHandlers struct {
	Store *store.Store
}

// active resolves the millésime the screens read, and answers 404 when no
// personnel file has been imported yet — the front then hides the page rather
// than showing empty charts.
func (h *DrhReadHandlers) active(c *gin.Context) (*store.DrhImport, bool) {
	im, err := h.Store.GetActiveDrhImport()
	if err != nil {
		internalError(c, err)
		return nil, false
	}
	if im == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "aucun fichier de personnel importé"})
		return nil, false
	}
	return im, true
}

// dimension validates the ?by= parameter against the dimensions actually computed.
func dimension(c *gin.Context, allowed []string, fallback string) (string, bool) {
	by := c.DefaultQuery("by", fallback)
	for _, d := range allowed {
		if by == d {
			return by, true
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("by doit être l'un de %s", strings.Join(allowed, ", "))})
	return "", false
}

// Summary gives the page header: millésime, national headcount, density,
// breakdown by posting and by category.
func (h *DrhReadHandlers) Summary(c *gin.Context) {
	im, ok := h.active(c)
	if !ok {
		return
	}
	national, err := h.Store.GetDrhEffectifs(im.ID, store.DrhEffectifParams{
		Dimension: drh.DimGlobal, Key: drh.KeyNational, Categorie: drh.CategorieToutes})
	if err != nil {
		internalError(c, err)
		return
	}
	categories, err := h.Store.GetDrhEffectifs(im.ID, store.DrhEffectifParams{
		Dimension: drh.DimGlobal, Key: drh.KeyNational, Categorie: "*"})
	if err != nil {
		internalError(c, err)
		return
	}
	var total *drh.EffectifRow
	if len(national) > 0 {
		total = &national[0]
	}
	c.JSON(http.StatusOK, gin.H{
		"import":     im,
		"national":   total,
		"categories": categories,
		"catalogue":  drh.Categories,
		"tranches":   drh.Tranches,
	})
}

func (h *DrhReadHandlers) Effectifs(c *gin.Context) {
	im, ok := h.active(c)
	if !ok {
		return
	}
	// Les dimensions de rollup, plus l'administration centrale, qui n'en est pas
	// une : ses entités ne se cumulent qu'au national, mais se lisent une à une.
	by, ok := dimension(c, append(append([]string{}, drh.RollupDimensions...), drh.AffCentrale), drh.DimDistrict)
	if !ok {
		return
	}
	rows, err := h.Store.GetDrhEffectifs(im.ID, store.DrhEffectifParams{
		Dimension: by,
		Key:       c.Query("key"),
		Categorie: c.DefaultQuery("categorie", drh.CategorieToutes),
		District:  c.Query("district"),
	})
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"effectifs": rows, "import_id": im.ID})
}

func (h *DrhReadHandlers) Pyramide(c *gin.Context) {
	im, ok := h.active(c)
	if !ok {
		return
	}
	by, ok := dimension(c, []string{drh.DimGlobal, drh.DimRegion, drh.DimDistrict, drh.DimType, drh.AffCentrale, drh.AffStructure}, drh.DimGlobal)
	if !ok {
		return
	}
	key := c.DefaultQuery("key", drh.KeyNational)
	rows, err := h.Store.GetDrhPyramide(im.ID, by, key, c.DefaultQuery("categorie", drh.CategorieToutes))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pyramide": rows, "age_retraite": im.AgeRetraite, "annee": im.Annee})
}

func (h *DrhReadHandlers) Comparaison(c *gin.Context) {
	im, ok := h.active(c)
	if !ok {
		return
	}
	by, ok := dimension(c, []string{drh.DimGlobal, drh.DimDistrict}, drh.DimDistrict)
	if !ok {
		return
	}
	rows, err := h.Store.GetDrhComparaison(im.ID, by, c.Query("key"), c.DefaultQuery("categorie", drh.CategorieToutes))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"comparaison": rows, "annee": im.Annee})
}

func (h *DrhReadHandlers) Structures(c *gin.Context) {
	im, ok := h.active(c)
	if !ok {
		return
	}
	rows, err := h.Store.GetDrhStructuresList(im.ID, c.Query("district"), c.Query("search"))
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"structures": rows})
}

// Structure is the "state staff posted here" block of a facility's page.
func (h *DrhReadHandlers) Structure(c *gin.Context) {
	im, ok := h.active(c)
	if !ok {
		return
	}
	total, err := h.Store.GetDrhEffectifs(im.ID, store.DrhEffectifParams{
		Dimension: drh.AffStructure, Key: c.Param("uid"), Categorie: drh.CategorieToutes})
	if err != nil {
		internalError(c, err)
		return
	}
	categories, err := h.Store.GetDrhStructureCategories(im.ID, c.Param("uid"))
	if err != nil {
		internalError(c, err)
		return
	}
	var row *drh.EffectifRow
	if len(total) > 0 {
		row = &total[0]
	}
	c.JSON(http.StatusOK, gin.H{"total": row, "categories": categories, "annee": im.Annee})
}
