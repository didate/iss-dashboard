package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"iss-dashboard-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type ExportHandlers struct {
	Store *store.Store
}

func (h *ExportHandlers) ExportExcel(c *gin.Context) {
	db := h.Store.DB()

	// Fetch metadata
	type deMeta struct {
		uid, code, name string
	}
	deRows, err := db.Query("SELECT de_uid, COALESCE(code,''), name FROM metadata_de ORDER BY section_prefix, name")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer deRows.Close()

	var des []deMeta
	deMap := make(map[string]string)
	for deRows.Next() {
		var d deMeta
		deRows.Scan(&d.uid, &d.code, &d.name)
		des = append(des, d)
		deMap[d.uid] = d.name
	}

	// Fetch events
	events, err := db.Query("SELECT event_uid, org_unit_uid, org_unit_name, district, region, event_date, status FROM event ORDER BY region, district, org_unit_name")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer events.Close()

	type eventRow struct {
		uid, ouUID, ouName, district, region, date, status string
	}
	var evts []eventRow
	for events.Next() {
		var e eventRow
		events.Scan(&e.uid, &e.ouUID, &e.ouName, &e.district, &e.region, &e.date, &e.status)
		evts = append(evts, e)
	}

	// Fetch all values
	valRows, err := db.Query("SELECT event_uid, de_uid, value FROM event_value")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer valRows.Close()

	valMap := make(map[string]map[string]string)
	for valRows.Next() {
		var evtUID, deUID, value string
		valRows.Scan(&evtUID, &deUID, &value)
		if valMap[evtUID] == nil {
			valMap[evtUID] = make(map[string]string)
		}
		valMap[evtUID][deUID] = value
	}

	// Fetch quality scores
	scoreRows, err := db.Query("SELECT event_uid, score, n_error, n_warning, n_info FROM event_quality")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer scoreRows.Close()

	type scoreInfo struct {
		score, nErr, nWarn, nInfo int
	}
	scoreMap := make(map[string]scoreInfo)
	for scoreRows.Next() {
		var uid string
		var s scoreInfo
		scoreRows.Scan(&uid, &s.score, &s.nErr, &s.nWarn, &s.nInfo)
		scoreMap[uid] = s
	}

	// Build Excel
	f := excelize.NewFile()
	sheet := "Donnees ISS"
	f.SetSheetName("Sheet1", sheet)

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", WrapText: true},
	})

	// Fixed headers
	fixedHeaders := []string{"Region", "District", "Structure", "Org Unit UID", "Event UID", "Date", "Statut", "Score Qualite", "Erreurs", "Avertissements", "Infos"}
	for i, h := range fixedHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	for i, d := range des {
		cell, _ := excelize.CoordinatesToCellName(len(fixedHeaders)+i+1, 1)
		f.SetCellValue(sheet, cell, d.name)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	// Data rows
	for rowIdx, evt := range evts {
		row := rowIdx + 2
		f.SetCellValue(sheet, cellName(1, row), evt.region)
		f.SetCellValue(sheet, cellName(2, row), evt.district)
		f.SetCellValue(sheet, cellName(3, row), evt.ouName)
		f.SetCellValue(sheet, cellName(4, row), evt.ouUID)
		f.SetCellValue(sheet, cellName(5, row), evt.uid)
		f.SetCellValue(sheet, cellName(6, row), evt.date)
		f.SetCellValue(sheet, cellName(7, row), evt.status)

		if sc, ok := scoreMap[evt.uid]; ok {
			f.SetCellValue(sheet, cellName(8, row), sc.score)
			f.SetCellValue(sheet, cellName(9, row), sc.nErr)
			f.SetCellValue(sheet, cellName(10, row), sc.nWarn)
			f.SetCellValue(sheet, cellName(11, row), sc.nInfo)
		}

		if vals, ok := valMap[evt.uid]; ok {
			for deIdx, d := range des {
				if v, ok := vals[d.uid]; ok && v != "" {
					col := len(fixedHeaders) + deIdx + 1
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						f.SetCellValue(sheet, cellName(col, row), num)
					} else {
						f.SetCellValue(sheet, cellName(col, row), v)
					}
				}
			}
		}
	}

	// Column widths
	widths := map[int]float64{1: 15, 2: 18, 3: 30, 4: 15, 5: 15, 6: 12, 7: 12, 8: 10, 9: 8, 10: 12, 11: 6}
	for col, w := range widths {
		colName, _ := excelize.ColumnNumberToName(col)
		f.SetColWidth(sheet, colName, colName, w)
	}

	// Auto filter
	lastCol, _ := excelize.ColumnNumberToName(len(fixedHeaders) + len(des))
	f.AutoFilter(sheet, fmt.Sprintf("A1:%s%d", lastCol, len(evts)+1), nil)

	// Freeze panes
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      11,
		YSplit:      1,
		TopLeftCell: "L2",
		ActivePane:  "bottomRight",
	})

	// Write response
	filename := fmt.Sprintf("extraction_iss_%s.xlsx", time.Now().Format("20060102_1504"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	f.Write(c.Writer)
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
