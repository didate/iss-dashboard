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
	DECode string `json:"de_code"`
	DEName string `json:"de_name"`
	Value  string `json:"value"`
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
		SELECT ev.de_code, COALESCE(md.name, ev.de_uid), ev.value
		FROM event_value ev
		LEFT JOIN metadata_de md ON ev.de_uid = md.de_uid
		WHERE ev.event_uid=?
		ORDER BY COALESCE(md.name, ev.de_uid)
	`, eventUID)
	if err != nil {
		return nil, err
	}
	defer valRows.Close()
	var values []EventValueDisplay
	for valRows.Next() {
		var v EventValueDisplay
		if err := valRows.Scan(&v.DECode, &v.DEName, &v.Value); err != nil {
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

// --- Filters ---

type Filters struct {
	Districts []string `json:"districts"`
	Regions   []string `json:"regions"`
	Rules     []string `json:"rules"`
	Services  []string `json:"services"`
	Statuts   []string `json:"statuts"`
}

func (s *Store) GetFilters() (*Filters, error) {
	f := &Filters{}
	f.Districts = s.distinctCol(`SELECT DISTINCT district FROM event WHERE district != '' ORDER BY district`)
	f.Regions = s.distinctCol(`SELECT DISTINCT region FROM event WHERE region != '' ORDER BY region`)
	f.Rules = s.distinctCol(`SELECT DISTINCT rule_code FROM quality_issue ORDER BY rule_code`)
	f.Services = s.distinctCol(`SELECT DISTINCT service_code FROM usage_service WHERE district='all' ORDER BY service_code`)
	f.Statuts = []string{"publique", "privée"}
	return f, nil
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
