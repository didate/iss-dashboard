package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
)

type PDFHandlers struct {
	Store *store.Store
}

func (h *PDFHandlers) ExportDistrictPDF(c *gin.Context) {
	district := c.Query("district")
	if district == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "district parameter required"})
		return
	}

	// Gather data
	var avgScore float64
	var nStructures int
	h.Store.DB().QueryRow(`SELECT COALESCE(avg_score,0), COALESCE(n_structures,0) FROM quality_summary WHERE dimension='district' AND key=?`, district).Scan(&avgScore, &nStructures)

	var reportingPct float64
	var reportingExp, reportingRep int
	h.Store.DB().QueryRow(`SELECT COALESCE(pct,0), COALESCE(n_expected,0), COALESCE(n_reported,0) FROM reporting_rate WHERE dimension='district' AND key=?`, district).Scan(&reportingPct, &reportingExp, &reportingRep)

	services, _ := h.Store.GetUsageServices(district)
	equipements, _ := h.Store.GetUsageEquipements("all", district)
	rhSummary, _ := h.Store.GetRHSummary(district)
	commodites, _ := h.Store.GetUsageCommodites(district)
	issueResult, _ := h.Store.GetQualityIssues(store.IssueListParams{District: district, Page: 1, PageSize: 30})

	// Build PDF
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 10, "Rapport ISS")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.Cell(0, 8, district)
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.Cell(0, 5, fmt.Sprintf("Genere le %s", time.Now().Format("02/01/2006 15:04")))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// KPIs
	pdf.SetFont("Helvetica", "B", 12)
	pdf.Cell(0, 8, "Indicateurs cles")
	pdf.Ln(9)
	pdfKPITable(pdf, [][]string{
		{"Structures analysees", fmt.Sprintf("%d", nStructures)},
		{"Score qualite moyen", fmt.Sprintf("%.1f / 100", avgScore)},
		{"Taux de rapportage", fmt.Sprintf("%.1f%% (%d / %d)", reportingPct, reportingRep, reportingExp)},
	})
	if rhSummary != nil {
		pdfKPITable(pdf, [][]string{
			{"Effectif total RH", fmt.Sprintf("%d", rhSummary.TotalEffectif)},
			{"Medecins / structure", fmt.Sprintf("%.2f", rhSummary.RatioMedPerStr)},
			{"Structures sans medecin", fmt.Sprintf("%d (%.1f%%)", rhSummary.NStrSansMed, rhSummary.PctStrSansMed)},
		})
	}
	pdf.Ln(6)

	// Services
	if len(services) > 0 {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, "Services disponibles")
		pdf.Ln(9)
		svcRows := make([][]string, len(services))
		for i, s := range services {
			svcRows[i] = []string{s.ServiceLabel, fmt.Sprintf("%d", s.NOui), fmt.Sprintf("%d", s.NTotal), fmt.Sprintf("%.1f%%", s.PctFonctionnel)}
		}
		pdfTable(pdf, []string{"Service", "Fonctionnel", "Total", "%"}, []float64{80, 25, 20, 20}, svcRows)
		pdf.Ln(6)
	}

	// Equipements
	if len(equipements) > 0 {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, "Equipements")
		pdf.Ln(9)
		eqRows := make([][]string, len(equipements))
		for i, e := range equipements {
			eqRows[i] = []string{e.Label, fmt.Sprintf("%d", e.SumTotal), fmt.Sprintf("%d", e.SumFonct), fmt.Sprintf("%.1f%%", e.PctFonct)}
		}
		pdfTable(pdf, []string{"Equipement", "Total", "Fonctionnel", "%"}, []float64{80, 25, 25, 20}, eqRows)
		pdf.Ln(6)
	}

	// Commodites
	commFiltered := pdfFilterCommodites(commodites)
	if len(commFiltered) > 0 {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, "Commodites (WASH / Energie)")
		pdf.Ln(9)
		cRows := make([][]string, len(commFiltered))
		for i, c := range commFiltered {
			cRows[i] = []string{c.Indicator, fmt.Sprintf("%d", c.NOui), fmt.Sprintf("%d", c.NTotal), fmt.Sprintf("%.1f%%", c.Pct)}
		}
		pdfTable(pdf, []string{"Indicateur", "Oui", "Total", "%"}, []float64{80, 25, 25, 20}, cRows)
		pdf.Ln(6)
	}

	// Top issues
	if issueResult != nil && len(issueResult.Data) > 0 {
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, fmt.Sprintf("Problemes qualite (%d structures)", issueResult.Total))
		pdf.Ln(9)
		iRows := make([][]string, len(issueResult.Data))
		for i, item := range issueResult.Data {
			iRows[i] = []string{item.OrgUnitName, item.WorstSeverity, fmt.Sprintf("%d", item.Score), fmt.Sprintf("%d/%d/%d", item.NError, item.NWarning, item.NInfo)}
		}
		pdfTable(pdf, []string{"Structure", "Severite", "Score", "E/W/I"}, []float64{75, 30, 20, 25}, iRows)
	}

	// Output
	filename := fmt.Sprintf("rapport_iss_%s_%s.pdf", strings.ReplaceAll(district, " ", "_"), time.Now().Format("20060102"))
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	if err := pdf.Output(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func (h *PDFHandlers) ExportStructurePDF(c *gin.Context) {
	uid := c.Param("uid")
	detail, err := h.Store.GetEventDetail(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "structure not found"})
		return
	}

	evt := detail.Event
	q := detail.Quality

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, "Fiche Structure ISS")
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.Cell(0, 8, evt.OrgUnitName)
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.Cell(0, 5, fmt.Sprintf("Genere le %s", time.Now().Format("02/01/2006 15:04")))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Header info
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 7, "Informations")
	pdf.Ln(8)
	kpis := [][]string{
		{"District", evt.District},
		{"Region", evt.Region},
		{"Date", evt.EventDate},
		{"Statut", evt.Status},
	}
	if q != nil {
		kpis = append(kpis, []string{"Score qualite", fmt.Sprintf("%d / 100", q.Score)})
		kpis = append(kpis, []string{"Erreurs / Avert. / Infos", fmt.Sprintf("%d / %d / %d", q.NError, q.NWarning, q.NInfo)})
	}
	pdfKPITable(pdf, kpis)
	pdf.Ln(6)

	// Quality issues
	if len(detail.Issues) > 0 {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 7, fmt.Sprintf("Problemes qualite (%d)", len(detail.Issues)))
		pdf.Ln(8)
		iRows := make([][]string, len(detail.Issues))
		for i, iss := range detail.Issues {
			iRows[i] = []string{iss.RuleCode, iss.Severity, iss.Message}
		}
		pdfTable(pdf, []string{"Regle", "Severite", "Message"}, []float64{25, 25, 120}, iRows)
		pdf.Ln(6)
	}

	// Group values by section_prefix
	type sectionData struct {
		label  string
		values []store.EventValueDisplay
	}
	sectionLabels := map[string]string{
		"ISS_GEN": "Informations generales", "ISS_SVC": "Services",
		"ISS_EQ": "Equipements", "ISS_EQUI": "Equipements",
		"ISS_RH": "Ressources humaines", "ISS_RH_SPE": "RH specialistes",
		"ISS_COMMO": "Commodites", "ISS_INFRA": "Infrastructure", "ISS_LAB": "Laboratoire",
	}
	getSectionLabel := func(prefix string) string {
		if prefix == "" {
			return "Autres"
		}
		if l, ok := sectionLabels[prefix]; ok {
			return l
		}
		for k, l := range sectionLabels {
			if strings.HasPrefix(prefix, k) {
				return l
			}
		}
		return prefix
	}

	sections := make(map[string]*sectionData)
	var sectionOrder []string
	for _, v := range detail.Values {
		label := getSectionLabel(v.SectionPrefix)
		if sections[label] == nil {
			sections[label] = &sectionData{label: label}
			sectionOrder = append(sectionOrder, label)
		}
		sections[label].values = append(sections[label].values, v)
	}

	for _, label := range sectionOrder {
		sec := sections[label]
		pdf.SetFont("Helvetica", "B", 11)
		pdf.Cell(0, 7, fmt.Sprintf("%s (%d)", sec.label, len(sec.values)))
		pdf.Ln(8)
		rows := make([][]string, len(sec.values))
		for i, v := range sec.values {
			val := v.Value
			if val == "" {
				val = "-"
			}
			rows[i] = []string{v.DEName, val}
		}
		pdfTable(pdf, []string{"Element", "Valeur"}, []float64{100, 60}, rows)
		pdf.Ln(4)
	}

	// Output
	safeName := strings.ReplaceAll(evt.OrgUnitName, " ", "_")
	filename := fmt.Sprintf("fiche_%s_%s.pdf", safeName, time.Now().Format("20060102"))
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	if err := pdf.Output(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func pdfKPITable(pdf *fpdf.Fpdf, rows [][]string) {
	pdf.SetFont("Helvetica", "", 10)
	for _, row := range rows {
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(80, 6, row[0], "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(60, 6, row[1], "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.Ln(6)
	}
}

func pdfTable(pdf *fpdf.Fpdf, headers []string, widths []float64, rows [][]string) {
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(41, 78, 121)
	pdf.SetTextColor(255, 255, 255)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(0, 0, 0)
	for j, row := range rows {
		if j%2 == 0 {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for i, cell := range row {
			align := "L"
			if i > 0 {
				align = "C"
			}
			pdf.CellFormat(widths[i], 5, cell, "1", 0, align, true, 0, "")
		}
		pdf.Ln(-1)
	}
}

func pdfFilterCommodites(commodites []models.UsageCommodite) []models.UsageCommodite {
	var out []models.UsageCommodite
	for _, c := range commodites {
		if !strings.HasPrefix(c.Indicator, "source_eau_") {
			out = append(out, c)
		}
	}
	return out
}
