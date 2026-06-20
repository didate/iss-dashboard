package store

import (
	"database/sql"
	"fmt"
	"strings"

	"iss-dashboard-backend/internal/models"
)

// --- Summary ---

type SummaryResult struct {
	NStructures    int            `json:"n_structures"`
	NOperationnel  int            `json:"n_operationnel"`
	AvgScore       float64        `json:"avg_score"`
	NError         int            `json:"n_error"`
	NWarning       int            `json:"n_warning"`
	NInfo          int            `json:"n_info"`
	LastSync       *models.SyncRun `json:"last_sync"`
}

func (s *Store) GetSummary() (*SummaryResult, error) {
	r := &SummaryResult{}

	row := s.db.QueryRow(`SELECT COUNT(*) FROM event`)
	row.Scan(&r.NStructures)

	row = s.db.QueryRow(`SELECT COUNT(*) FROM event WHERE status='COMPLETED'`)
	row.Scan(&r.NOperationnel)

	row = s.db.QueryRow(`SELECT COALESCE(AVG(score),0) FROM event_quality`)
	row.Scan(&r.AvgScore)

	row = s.db.QueryRow(`SELECT COALESCE(SUM(n_error),0), COALESCE(SUM(n_warning),0), COALESCE(SUM(n_info),0) FROM event_quality`)
	row.Scan(&r.NError, &r.NWarning, &r.NInfo)

	r.LastSync, _ = s.GetLastSyncRun()
	return r, nil
}

// --- Quality Summary ---

func (s *Store) GetQualitySummary(dimension string) ([]models.QualitySummaryRow, error) {
	if dimension == "" {
		dimension = "global"
	}
	rows, err := s.db.Query(`SELECT dimension, key, label, avg_score, n_error, n_warning, n_info, n_structures FROM quality_summary WHERE dimension=? ORDER BY avg_score ASC`, dimension)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.QualitySummaryRow
	for rows.Next() {
		var r models.QualitySummaryRow
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.AvgScore, &r.NError, &r.NWarning, &r.NInfo, &r.NStructures); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Quality Issues (paginated) ---

type IssueListParams struct {
	Severity string
	Rule     string
	District string
	Search   string
	Page     int
	PageSize int
}

type IssueListItem struct {
	EventUID      string         `json:"event_uid"`
	OrgUnitName   string         `json:"org_unit_name"`
	District      string         `json:"district"`
	Region        string         `json:"region"`
	WorstSeverity string         `json:"worst_severity"`
	Score         int            `json:"score"`
	NError        int            `json:"n_error"`
	NWarning      int            `json:"n_warning"`
	NInfo         int            `json:"n_info"`
	Issues        []models.Issue `json:"issues"`
}

type IssueListResult struct {
	Data     []IssueListItem `json:"data"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

func (s *Store) GetQualityIssues(p IssueListParams) (*IssueListResult, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		p.PageSize = 20
	}

	// SECURITY: only hardcoded conditions go in where[]. User values go in args[] as parameterized placeholders (?).
	where := []string{"1=1"}
	args := []any{}

	if p.Severity != "" {
		where = append(where, "eq.worst_severity = ?")
		args = append(args, p.Severity)
	}
	if p.District != "" {
		where = append(where, "e.district = ?")
		args = append(args, p.District)
	}
	if p.Search != "" {
		where = append(where, "e.org_unit_name LIKE ?")
		args = append(args, "%"+p.Search+"%")
	}
	if p.Rule != "" {
		where = append(where, "e.event_uid IN (SELECT qi.event_uid FROM quality_issue qi WHERE qi.rule_code = ?)")
		args = append(args, p.Rule)
	}

	whereClause := strings.Join(where, " AND ")

	// Count
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM event e JOIN event_quality eq ON e.event_uid = eq.event_uid WHERE %s AND eq.n_error + eq.n_warning + eq.n_info > 0`, whereClause)
	var total int
	s.db.QueryRow(countSQL, args...).Scan(&total)

	// Page
	offset := (p.Page - 1) * p.PageSize
	listSQL := fmt.Sprintf(`
		SELECT e.event_uid, e.org_unit_name, e.district, e.region, eq.worst_severity, eq.score, eq.n_error, eq.n_warning, eq.n_info
		FROM event e
		JOIN event_quality eq ON e.event_uid = eq.event_uid
		WHERE %s AND eq.n_error + eq.n_warning + eq.n_info > 0
		ORDER BY
			CASE eq.worst_severity WHEN 'error' THEN 0 WHEN 'warning' THEN 1 WHEN 'info' THEN 2 ELSE 3 END,
			eq.score ASC
		LIMIT ? OFFSET ?
	`, whereClause)
	listArgs := append(args, p.PageSize, offset)

	rows, err := s.db.Query(listSQL, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []IssueListItem
	for rows.Next() {
		var it IssueListItem
		if err := rows.Scan(&it.EventUID, &it.OrgUnitName, &it.District, &it.Region, &it.WorstSeverity, &it.Score, &it.NError, &it.NWarning, &it.NInfo); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch issues for each item
	for i := range items {
		issues, err := s.getIssuesForEvent(items[i].EventUID)
		if err != nil {
			return nil, err
		}
		items[i].Issues = issues
	}

	return &IssueListResult{Data: items, Total: total, Page: p.Page, PageSize: p.PageSize}, nil
}

func (s *Store) getIssuesForEvent(eventUID string) ([]models.Issue, error) {
	rows, err := s.db.Query(`SELECT rule_code, severity, rule_name, message FROM quality_issue WHERE event_uid=? ORDER BY CASE severity WHEN 'error' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END`, eventUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Issue
	for rows.Next() {
		var iss models.Issue
		if err := rows.Scan(&iss.RuleCode, &iss.Severity, &iss.RuleName, &iss.Message); err != nil {
			return nil, err
		}
		out = append(out, iss)
	}
	return out, rows.Err()
}

// --- Event Detail ---

type EventDetail struct {
	Event   models.Event         `json:"event"`
	Values  []EventValueDisplay  `json:"values"`
	Issues  []models.Issue       `json:"issues"`
	Quality *models.EventQuality `json:"quality"`
}

type EventValueDisplay struct {
	DECode        string `json:"de_code"`
	DEName        string `json:"de_name"`
	Value         string `json:"value"`
	SectionPrefix string `json:"section_prefix"`
}

// --- Structures list ---

type StructureListParams struct {
	District string
	Search   string
	Page     int
	PageSize int
}

type StructureListItem struct {
	EventUID    string `json:"event_uid"`
	OrgUnitName string `json:"org_unit_name"`
	District    string `json:"district"`
	Region      string `json:"region"`
	EventDate   string `json:"event_date"`
	Status      string `json:"status"`
	Score       int    `json:"score"`
	NError      int    `json:"n_error"`
	NWarning    int    `json:"n_warning"`
	NInfo       int    `json:"n_info"`
}

type StructureListResult struct {
	Data     []StructureListItem `json:"data"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

func (s *Store) GetStructuresList(p StructureListParams) (*StructureListResult, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > 100 {
		p.PageSize = 20
	}

	// SECURITY: only hardcoded conditions go in where[]. User values go in args[] as parameterized placeholders (?).
	where := []string{"1=1"}
	args := []any{}

	if p.District != "" {
		where = append(where, "e.district = ?")
		args = append(args, p.District)
	}
	if p.Search != "" {
		where = append(where, "e.org_unit_name LIKE ?")
		args = append(args, "%"+p.Search+"%")
	}

	whereClause := strings.Join(where, " AND ")

	// Count
	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM event e WHERE %s`, whereClause)
	s.db.QueryRow(countQ, args...).Scan(&total)

	// Data
	query := fmt.Sprintf(`
		SELECT e.event_uid, e.org_unit_name, e.district, e.region, e.event_date, e.status,
			COALESCE(eq.score, 100), COALESCE(eq.n_error, 0), COALESCE(eq.n_warning, 0), COALESCE(eq.n_info, 0)
		FROM event e
		LEFT JOIN event_quality eq ON e.event_uid = eq.event_uid
		WHERE %s
		ORDER BY e.org_unit_name ASC
		LIMIT ? OFFSET ?
	`, whereClause)
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []StructureListItem
	for rows.Next() {
		var item StructureListItem
		if err := rows.Scan(&item.EventUID, &item.OrgUnitName, &item.District, &item.Region, &item.EventDate, &item.Status, &item.Score, &item.NError, &item.NWarning, &item.NInfo); err != nil {
			return nil, err
		}
		data = append(data, item)
	}

	return &StructureListResult{Data: data, Total: total, Page: p.Page, PageSize: p.PageSize}, rows.Err()
}

func (s *Store) GetEventDetail(eventUID string) (*EventDetail, error) {
	// Event
	var evt models.Event
	row := s.db.QueryRow(`SELECT event_uid, org_unit_uid, org_unit_name, district, region, event_date, status FROM event WHERE event_uid=?`, eventUID)
	if err := row.Scan(&evt.EventUID, &evt.OrgUnitUID, &evt.OrgUnitName, &evt.District, &evt.Region, &evt.EventDate, &evt.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Values
	valRows, err := s.db.Query(`
		SELECT ev.de_code, COALESCE(NULLIF(md.form_name,''), md.name, ev.de_uid), ev.value, COALESCE(md.section_prefix, '')
		FROM event_value ev
		LEFT JOIN metadata_de md ON ev.de_uid = md.de_uid
		WHERE ev.event_uid=?
		ORDER BY md.section_prefix, COALESCE(NULLIF(md.form_name,''), md.name, ev.de_uid)
	`, eventUID)
	if err != nil {
		return nil, err
	}
	defer valRows.Close()
	var values []EventValueDisplay
	for valRows.Next() {
		var v EventValueDisplay
		if err := valRows.Scan(&v.DECode, &v.DEName, &v.Value, &v.SectionPrefix); err != nil {
			return nil, err
		}
		values = append(values, v)
	}

	issues, err := s.getIssuesForEvent(eventUID)
	if err != nil {
		return nil, err
	}

	// Quality
	var eq models.EventQuality
	eqRow := s.db.QueryRow(`SELECT event_uid, n_error, n_warning, n_info, worst_severity, score FROM event_quality WHERE event_uid=?`, eventUID)
	if err := eqRow.Scan(&eq.EventUID, &eq.NError, &eq.NWarning, &eq.NInfo, &eq.WorstSeverity, &eq.Score); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return &EventDetail{Event: evt, Values: values, Issues: issues, Quality: &eq}, nil
}

// --- Usage queries ---

func (s *Store) GetUsageRecensement(dimension string) ([]models.UsageRecensement, error) {
	if dimension == "" {
		dimension = "global"
	}
	rows, err := s.db.Query(`SELECT dimension, key, label, n_structures, n_operationnel, n_non_operationnel, n_ferme_temp FROM usage_recensement WHERE dimension=? ORDER BY n_structures DESC`, dimension)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageRecensement
	for rows.Next() {
		var r models.UsageRecensement
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.NStructures, &r.NOperationnel, &r.NNonOperationnel, &r.NFermeTemp); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetUsageServices(district string) ([]models.UsageService, error) {
	d := "all"
	if district != "" {
		d = district
	}
	rows, err := s.db.Query(`SELECT service_code, service_label, district, n_oui, n_oui_pas_fonc, n_non, n_total, pct_fonctionnel FROM usage_service WHERE district=? ORDER BY pct_fonctionnel DESC`, d)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageService
	for rows.Next() {
		var r models.UsageService
		if err := rows.Scan(&r.ServiceCode, &r.ServiceLabel, &r.District, &r.NOui, &r.NOuiPasFonc, &r.NNon, &r.NTotal, &r.PctFonctionnel); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetUsageEquipements(focus, district string) ([]models.UsageEquipement, error) {
	d := "all"
	if district != "" {
		d = district
	}
	query := `SELECT equip_root, label, district, sum_total, sum_fonct, pct_fonct, category FROM usage_equipement WHERE district=?`
	args := []any{d}

	if focus != "" && focus != "all" {
		query += ` AND category=?`
		args = append(args, focus)
	}
	query += ` ORDER BY pct_fonct ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageEquipement
	for rows.Next() {
		var r models.UsageEquipement
		if err := rows.Scan(&r.EquipRoot, &r.Label, &r.District, &r.SumTotal, &r.SumFonct, &r.PctFonct, &r.Category); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetUsageRH(district string) ([]models.UsageRH, error) {
	d := "all"
	if district != "" {
		d = district
	}
	rows, err := s.db.Query(`SELECT profil_code, label, district, effectif_fonc, effectif_contr, effectif_benev, effectif_total FROM usage_rh WHERE district=? ORDER BY effectif_total DESC`, d)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageRH
	for rows.Next() {
		var r models.UsageRH
		if err := rows.Scan(&r.ProfilCode, &r.Label, &r.District, &r.EffectifFonc, &r.EffectifContr, &r.EffectifBenev, &r.EffectifTotal); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetUsageCommodites(district string) ([]models.UsageCommodite, error) {
	d := "all"
	if district != "" {
		d = district
	}
	rows, err := s.db.Query(`SELECT indicator, district, n_oui, n_total, pct FROM usage_commodite WHERE district=? ORDER BY indicator`, d)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UsageCommodite
	for rows.Next() {
		var r models.UsageCommodite
		if err := rows.Scan(&r.Indicator, &r.District, &r.NOui, &r.NTotal, &r.Pct); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Reporting Rate ---

func (s *Store) GetReportingRate(dimension string) ([]models.ReportingRate, error) {
	if dimension == "" {
		dimension = "global"
	}
	rows, err := s.db.Query(`SELECT dimension, key, label, n_expected, n_reported, pct FROM reporting_rate WHERE dimension=? ORDER BY pct ASC`, dimension)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ReportingRate
	for rows.Next() {
		var r models.ReportingRate
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.NExpected, &r.NReported, &r.Pct); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Plateau technique ---

type PlateauItem struct {
	ServiceCode  string  `json:"service_code"`
	ServiceLabel string  `json:"service_label"`
	NOui         int     `json:"n_oui"`
	NTotal       int     `json:"n_total"`
	Pct          float64 `json:"pct"`
}

func (s *Store) GetPlateauTechnique(district string) ([]PlateauItem, error) {
	plateauCodes := []string{
		"ISS_SVC_LABO_DE",
		"ISS_SVC_MATERNITE_DE",
		"ISS_SVC_CHIRURGIE_DE",
		"ISS_SVC_IMAGERIE_DE",
		"ISS_SVC_PHARMACIE_DE",
		"ISS_SVC_URGENCES_DE",
		"ISS_SVC_PEDIATRIE_DE",
		"ISS_SVC_MED_GEN_DE",
		"ISS_SVC_HEMODIALYSE_DE",
		"ISS_SVC_NEONAT_DE",
		"ISS_SVC_DENTAIRE_DE",
		"ISS_SVC_ANESTH_REA_DE",
	}
	d := "all"
	if district != "" {
		d = district
	}
	var out []PlateauItem
	for _, code := range plateauCodes {
		row := s.db.QueryRow(`SELECT service_code, service_label, n_oui, n_total, pct_fonctionnel FROM usage_service WHERE service_code=? AND district=?`, code, d)
		var p PlateauItem
		if err := row.Scan(&p.ServiceCode, &p.ServiceLabel, &p.NOui, &p.NTotal, &p.Pct); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// --- Service Matrix ---

type ServiceMatrixRow struct {
	ServiceCode  string             `json:"service_code"`
	ServiceLabel string             `json:"service_label"`
	Districts    map[string]float64 `json:"districts"`
	Overall      float64            `json:"overall"`
}

func (s *Store) GetServiceMatrix() ([]ServiceMatrixRow, error) {
	rows, err := s.db.Query(`SELECT service_code, service_label, district, pct_fonctionnel FROM usage_service ORDER BY service_code, district`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matrixMap := make(map[string]*ServiceMatrixRow)
	var order []string

	for rows.Next() {
		var code, label, district string
		var pct float64
		if err := rows.Scan(&code, &label, &district, &pct); err != nil {
			return nil, err
		}
		if matrixMap[code] == nil {
			matrixMap[code] = &ServiceMatrixRow{
				ServiceCode:  code,
				ServiceLabel: label,
				Districts:    make(map[string]float64),
			}
			order = append(order, code)
		}
		if district == "all" {
			matrixMap[code].Overall = pct
		} else {
			matrixMap[code].Districts[district] = pct
		}
	}

	var out []ServiceMatrixRow
	for _, code := range order {
		out = append(out, *matrixMap[code])
	}
	return out, rows.Err()
}

// --- RH Summary ---

type RHSummaryResult struct {
	TotalEffectif  int     `json:"total_effectif"`
	TotalFonc      int     `json:"total_fonc"`
	TotalContr     int     `json:"total_contr"`
	TotalBenev     int     `json:"total_benev"`
	NStructures    int     `json:"n_structures"`
	RatioMedPerStr float64 `json:"ratio_med_per_structure"`
	NStrSansMed    int     `json:"n_structures_sans_medecin"`
	PctStrSansMed  float64 `json:"pct_structures_sans_medecin"`
}

func (s *Store) GetRHSummary(district string) (*RHSummaryResult, error) {
	r := &RHSummaryResult{}
	d := "all"
	if district != "" {
		d = district
	}

	row := s.db.QueryRow(`SELECT COALESCE(SUM(effectif_fonc),0), COALESCE(SUM(effectif_contr),0), COALESCE(SUM(effectif_benev),0), COALESCE(SUM(effectif_total),0) FROM usage_rh WHERE district=?`, d)
	row.Scan(&r.TotalFonc, &r.TotalContr, &r.TotalBenev, &r.TotalEffectif)

	s.db.QueryRow(`SELECT COUNT(*) FROM event`).Scan(&r.NStructures)

	// Count medecins (generaliste + specialists)
	var totalMed int
	s.db.QueryRow(`SELECT COALESCE(SUM(effectif_total),0) FROM usage_rh WHERE district=? AND (profil_code LIKE '%MED_GEN%' OR profil_code LIKE '%MED_CHIR%' OR profil_code LIKE '%MED_GYNE%' OR profil_code LIKE '%MED_PED%' OR profil_code LIKE '%MED_ANESTH%' OR profil_code LIKE '%MED_AUTRE%' OR profil_code LIKE '%MED_SP_PUB%' OR profil_code LIKE '%MEDECIN_URGENTISTE%')`, d).Scan(&totalMed)

	if r.NStructures > 0 {
		r.RatioMedPerStr = float64(totalMed) / float64(r.NStructures)
	}

	// Structures without any medecin: events where all med DEs are 0 or empty
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT e.event_uid) FROM event e
		WHERE e.event_uid NOT IN (
			SELECT ev.event_uid FROM event_value ev
			WHERE (ev.de_code LIKE '%MED_GEN%' OR ev.de_code LIKE '%MED_CHIR%' OR ev.de_code LIKE '%MED_GYNE%' OR ev.de_code LIKE '%MED_PED%' OR ev.de_code LIKE '%MED_ANESTH%' OR ev.de_code LIKE '%MED_AUTRE%' OR ev.de_code LIKE '%MED_SP_PUB%' OR ev.de_code LIKE '%MEDECIN_URGENTISTE%')
			AND CAST(ev.value AS REAL) > 0
		)
	`).Scan(&r.NStrSansMed)

	if r.NStructures > 0 {
		r.PctStrSansMed = float64(r.NStrSansMed) / float64(r.NStructures) * 100
	}

	return r, nil
}

// --- Closed OUs ---

type ClosedOUItem struct {
	UID        string `json:"uid"`
	Name       string `json:"name"`
	ClosedDate string `json:"closed_date"`
	District   string `json:"district"`
	Region     string `json:"region"`
	HasData    bool   `json:"has_data"`
}

func (s *Store) GetClosedOUs(district string) ([]ClosedOUItem, error) {
	query := `
		SELECT
			ou.uid, ou.name, ou.closed_date,
			COALESCE(p3.name, '') as district,
			COALESCE(p2.name, '') as region,
			CASE WHEN e.event_uid IS NOT NULL THEN 1 ELSE 0 END as has_data
		FROM org_unit ou
		LEFT JOIN org_unit p ON ou.parent_uid = p.uid
		LEFT JOIN org_unit p3 ON
			CASE
				WHEN ou.level = 6 THEN p.parent_uid
				WHEN ou.level = 5 THEN ou.parent_uid
				ELSE ''
			END = p3.uid AND p3.level = 3
		LEFT JOIN org_unit p2 ON p3.parent_uid = p2.uid AND p2.level = 2
		LEFT JOIN event e ON ou.uid = e.org_unit_uid
		WHERE ou.closed_date != ''
	`
	args := []any{}
	if district != "" {
		query += ` AND p3.name = ?`
		args = append(args, district)
	}
	query += ` ORDER BY ou.closed_date DESC, ou.name`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ClosedOUItem
	for rows.Next() {
		var c ClosedOUItem
		var hasData int
		if err := rows.Scan(&c.UID, &c.Name, &c.ClosedDate, &c.District, &c.Region, &hasData); err != nil {
			return nil, err
		}
		c.HasData = hasData == 1
		// Clean closedDate format
		if len(c.ClosedDate) > 10 {
			c.ClosedDate = c.ClosedDate[:10]
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- Filters ---

type RuleInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Filters struct {
	Districts       []string            `json:"districts"`
	Regions         []string            `json:"regions"`
	DistrictRegions map[string]string   `json:"district_regions"`
	DistrictUIDs    map[string]string   `json:"district_uids"`
	Rules           []RuleInfo          `json:"rules"`
	Services        []string            `json:"services"`
	Statuts         []string            `json:"statuts"`
}

func (s *Store) GetFilters() (*Filters, error) {
	f := &Filters{}
	f.Districts = s.distinctCol(`SELECT DISTINCT name FROM org_unit WHERE level=3 ORDER BY name`)
	f.Regions = s.distinctCol(`SELECT DISTINCT name FROM org_unit WHERE level=2 ORDER BY name`)
	f.Services = s.distinctCol(`SELECT DISTINCT service_code FROM usage_service WHERE district='all' ORDER BY service_code`)
	f.Statuts = []string{"publique", "privée"}

	// District UID → Name mapping
	f.DistrictUIDs = make(map[string]string)
	uidRows, err := s.db.Query(`SELECT uid, name FROM org_unit WHERE level=3 ORDER BY name`)
	if err == nil {
		defer uidRows.Close()
		for uidRows.Next() {
			var uid, name string
			if uidRows.Scan(&uid, &name) == nil {
				f.DistrictUIDs[uid] = name
			}
		}
	}

	// District → Region mapping
	f.DistrictRegions = make(map[string]string)
	drRows, err := s.db.Query(`SELECT d.name, COALESCE(r.name,'') FROM org_unit d LEFT JOIN org_unit r ON d.parent_uid = r.uid AND r.level=2 WHERE d.level=3`)
	if err == nil {
		defer drRows.Close()
		for drRows.Next() {
			var dist, region string
			if drRows.Scan(&dist, &region) == nil && region != "" {
				f.DistrictRegions[dist] = region
			}
		}
	}

	// Rules with names
	rows, err := s.db.Query(`SELECT DISTINCT rule_code, rule_name FROM quality_issue ORDER BY rule_code`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r RuleInfo
			if err := rows.Scan(&r.Code, &r.Name); err == nil {
				f.Rules = append(f.Rules, r)
			}
		}
	}
	return f, nil
}

// --- Map Data ---

// --- Compare Districts ---

type CompareDistrictData struct {
	Name           string                  `json:"name"`
	AvgScore       float64                 `json:"avg_score"`
	NStructures    int                     `json:"n_structures"`
	ReportingPct   float64                 `json:"reporting_pct"`
	ReportingExp   int                     `json:"reporting_expected"`
	ReportingRep   int                     `json:"reporting_reported"`
	Services       []models.UsageService   `json:"services"`
	Equipements    []models.UsageEquipement `json:"equipements"`
	RH             []models.UsageRH        `json:"rh"`
	RHSummary      *RHSummaryResult        `json:"rh_summary"`
	Commodites     []models.UsageCommodite `json:"commodites"`
}

type CompareResult struct {
	Districts []CompareDistrictData `json:"districts"`
	National  CompareDistrictData   `json:"national"`
}

func (s *Store) getDistrictCompareData(district string) CompareDistrictData {
	d := CompareDistrictData{Name: district}
	if district == "" {
		d.Name = "National"
	}

	// Quality score
	qKey := district
	if district == "" {
		qKey = "all"
	}
	s.db.QueryRow(`SELECT COALESCE(avg_score,0), COALESCE(n_structures,0) FROM quality_summary WHERE dimension='district' AND key=?`, qKey).Scan(&d.AvgScore, &d.NStructures)
	if district == "" {
		// For national, use global
		s.db.QueryRow(`SELECT COALESCE(avg_score,0), COALESCE(n_structures,0) FROM quality_summary WHERE dimension='global' AND key='all'`).Scan(&d.AvgScore, &d.NStructures)
	}

	// Reporting
	rKey := district
	if district == "" {
		rKey = "all"
	}
	dim := "district"
	if district == "" {
		dim = "global"
	}
	s.db.QueryRow(`SELECT COALESCE(pct,0), COALESCE(n_expected,0), COALESCE(n_reported,0) FROM reporting_rate WHERE dimension=? AND key=?`, dim, rKey).Scan(&d.ReportingPct, &d.ReportingExp, &d.ReportingRep)

	// Services
	svcDist := district
	if svcDist == "" {
		svcDist = "all"
	}
	d.Services, _ = s.GetUsageServices(svcDist)
	d.Equipements, _ = s.GetUsageEquipements("all", svcDist)
	d.RH, _ = s.GetUsageRH(svcDist)
	d.RHSummary, _ = s.GetRHSummary(svcDist)
	d.Commodites, _ = s.GetUsageCommodites(svcDist)

	return d
}

func (s *Store) GetCompareData(districts []string) (*CompareResult, error) {
	result := &CompareResult{
		National: s.getDistrictCompareData(""),
	}
	for _, d := range districts {
		result.Districts = append(result.Districts, s.getDistrictCompareData(d))
	}
	return result, nil
}

func (s *Store) GetMapData() (*models.MapDistrictCollection, error) {
	// 1. Load district org units (level 3) with geometry
	type districtGeo struct {
		uid      string
		name     string
		geometry string
	}
	rows, err := s.db.Query(`SELECT uid, name, COALESCE(geometry,'') FROM org_unit WHERE level=3 AND geometry != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var districts []districtGeo
	for rows.Next() {
		var d districtGeo
		if err := rows.Scan(&d.uid, &d.name, &d.geometry); err != nil {
			return nil, err
		}
		districts = append(districts, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. Load reporting_rate by district
	type reportingData struct {
		expected, reported int
		pct                float64
	}
	reporting := map[string]*reportingData{}
	rRows, err := s.db.Query(`SELECT key, n_expected, n_reported, pct FROM reporting_rate WHERE dimension='district'`)
	if err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var key string
			var d reportingData
			if rRows.Scan(&key, &d.expected, &d.reported, &d.pct) == nil {
				reporting[key] = &d
			}
		}
	}

	// 3. Load quality_summary by district
	type qualityData struct {
		avgScore    float64
		nStructures int
	}
	quality := map[string]*qualityData{}
	qRows, err := s.db.Query(`SELECT key, avg_score, n_structures FROM quality_summary WHERE dimension='district'`)
	if err == nil {
		defer qRows.Close()
		for qRows.Next() {
			var key string
			var d qualityData
			if qRows.Scan(&key, &d.avgScore, &d.nStructures) == nil {
				quality[key] = &d
			}
		}
	}

	// 4. Load usage_service by district (exclude 'all')
	type svcData struct {
		label      string
		pct        float64
		nOui, nTot int
	}
	services := map[string]map[string]*svcData{} // district -> service_code -> data
	sRows, err := s.db.Query(`SELECT service_code, service_label, district, n_oui, n_total, pct_fonctionnel FROM usage_service WHERE district != 'all'`)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var code, label, dist string
			var d svcData
			if sRows.Scan(&code, &label, &dist, &d.nOui, &d.nTot, &d.pct) == nil {
				d.label = label
				if services[dist] == nil {
					services[dist] = map[string]*svcData{}
				}
				services[dist][code] = &d
			}
		}
	}

	// 5. Load usage_equipement by district (exclude 'all')
	type eqData struct {
		label    string
		category string
		sumTot   int
		sumFonc  int
	}
	equipements := map[string]map[string]*eqData{} // district -> equip_root -> data
	eRows, err := s.db.Query(`SELECT equip_root, label, district, sum_total, sum_fonct, category FROM usage_equipement WHERE district != 'all'`)
	if err == nil {
		defer eRows.Close()
		for eRows.Next() {
			var root, label, dist, cat string
			var sumTot, sumFonc int
			if eRows.Scan(&root, &label, &dist, &sumTot, &sumFonc, &cat) == nil {
				if equipements[dist] == nil {
					equipements[dist] = map[string]*eqData{}
				}
				equipements[dist][root] = &eqData{label: label, category: cat, sumTot: sumTot, sumFonc: sumFonc}
			}
		}
	}

	// 6. Load usage_commodite for WASH (source_eau_*) by district
	type washCounter struct {
		fmh, fme, reseau, total int
	}
	wash := map[string]*washCounter{} // district -> counts
	cRows, err := s.db.Query(`SELECT indicator, district, n_oui, n_total FROM usage_commodite WHERE district != 'all' AND indicator LIKE 'source_eau_%'`)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var ind, dist string
			var nOui, nTotal int
			if cRows.Scan(&ind, &dist, &nOui, &nTotal) == nil {
				if wash[dist] == nil {
					wash[dist] = &washCounter{}
				}
				w := wash[dist]
				switch ind {
				case "source_eau_FMH":
					w.fmh = nOui
				case "source_eau_FME", "source_eau_FEM":
					w.fme = nOui
				case "source_eau_réseau":
					w.reseau = nOui
				case "source_eau_total":
					w.total = nTotal
				}
			}
		}
	}

	// 6b. Load eau_pts_critiques by district
	type eauData struct {
		nOui, nTotal int
	}
	eauPts := map[string]*eauData{}
	epRows, err := s.db.Query(`SELECT district, n_oui, n_total FROM usage_commodite WHERE district != 'all' AND indicator='eau_pts_critiques'`)
	if err == nil {
		defer epRows.Close()
		for epRows.Next() {
			var dist string
			var nOui, nTotal int
			if epRows.Scan(&dist, &nOui, &nTotal) == nil {
				eauPts[dist] = &eauData{nOui: nOui, nTotal: nTotal}
			}
		}
	}

	// 7. Load RH medecin totals by district
	type rhData struct {
		medTotal    int
		nStructures int
	}
	rh := map[string]*rhData{}
	mRows, err := s.db.Query(`SELECT district, COALESCE(SUM(effectif_total),0) FROM usage_rh WHERE district != 'all' AND (profil_code LIKE '%MED_GEN%' OR profil_code LIKE '%MED_CHIR%' OR profil_code LIKE '%MED_GYNE%' OR profil_code LIKE '%MED_PED%' OR profil_code LIKE '%MED_ANESTH%' OR profil_code LIKE '%MED_AUTRE%' OR profil_code LIKE '%MED_SP_PUB%' OR profil_code LIKE '%MEDECIN_URGENTISTE%') GROUP BY district`)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var dist string
			var total int
			if mRows.Scan(&dist, &total) == nil {
				rh[dist] = &rhData{medTotal: total}
			}
		}
	}
	// Get structure counts per district
	nRows, err := s.db.Query(`SELECT district, COUNT(*) FROM event GROUP BY district`)
	if err == nil {
		defer nRows.Close()
		for nRows.Next() {
			var dist string
			var n int
			if nRows.Scan(&dist, &n) == nil {
				if rh[dist] == nil {
					rh[dist] = &rhData{}
				}
				rh[dist].nStructures = n
			}
		}
	}

	// 8. Assemble GeoJSON features
	features := make([]models.MapDistrictFeature, 0, len(districts))
	for _, d := range districts {
		props := models.MapDistrictProperties{
			DistrictUID:  d.uid,
			DistrictName: d.name,
			Services:     map[string]models.ServiceMapData{},
			Equipements:  map[string]models.EquipMapData{},
		}

		// Reporting
		if r, ok := reporting[d.name]; ok {
			pct := r.pct
			props.RapportagePct = &pct
			props.RapportageExpected = r.expected
			props.RapportageReported = r.reported
		}

		// Quality
		if q, ok := quality[d.name]; ok {
			score := q.avgScore
			props.QualiteAvgScore = &score
			props.QualiteNStruct = q.nStructures
		}

		// Services
		if svcMap, ok := services[d.name]; ok {
			for code, s := range svcMap {
				props.Services[code] = models.ServiceMapData{
					ServiceLabel:   s.label,
					PctFonctionnel: s.pct,
					NOui:           s.nOui,
					NTotal:         s.nTot,
				}
			}
		}

		// Equipements
		if eqMap, ok := equipements[d.name]; ok {
			for root, e := range eqMap {
				props.Equipements[root] = models.EquipMapData{
					Label:    e.label,
					Category: e.category,
					SumTotal: e.sumTot,
					SumFonct: e.sumFonc,
				}
			}
		}

		// WASH
		if w, ok := wash[d.name]; ok {
			n := w.fmh + w.fme + w.reseau
			props.WashForageOuReseauN = n
			props.WashTotal = w.total
			if w.total > 0 {
				pct := float64(n) / float64(w.total) * 100
				props.WashForageOuReseauPct = &pct
			}
		}
		if ep, ok := eauPts[d.name]; ok {
			props.WashEauPtsCritiquesN = ep.nOui
			if ep.nTotal > 0 {
				pct := float64(ep.nOui) / float64(ep.nTotal) * 100
				props.WashEauPtsCritiquesPct = &pct
			}
		}

		// RH
		if r, ok := rh[d.name]; ok {
			props.RhMedecinsTotal = r.medTotal
			props.RhNStructures = r.nStructures
			if r.nStructures > 0 {
				ratio := float64(r.medTotal) / float64(r.nStructures)
				props.RhMedecinsParStruct = &ratio
			}
		}

		features = append(features, models.MapDistrictFeature{
			Type:       "Feature",
			Geometry:   []byte(d.geometry),
			Properties: props,
		})
	}

	return &models.MapDistrictCollection{
		Type:     "FeatureCollection",
		Features: features,
	}, nil
}

func (s *Store) distinctCol(query string) []string {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err == nil && v != "" {
			out = append(out, v)
		}
	}
	return out
}
