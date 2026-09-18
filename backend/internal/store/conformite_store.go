package store

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/normes"
)

// LoadEvents rebuilds the events (with their data values and enrichment
// columns) from SQLite, so conformity can be recomputed without DHIS2.
func (s *Store) LoadEvents() ([]models.Event, error) {
	rows, err := s.db.Query(`SELECT event_uid, org_unit_uid, org_unit_name, district, region, event_date, status,
		COALESCE(district_uid,''), COALESCE(sous_prefecture,''), COALESCE(sous_prefecture_uid,''), COALESCE(type_code,''), COALESCE(type_source,''), lat, lng
		FROM event ORDER BY event_uid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []models.Event
	index := map[string]int{}
	for rows.Next() {
		var e models.Event
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&e.EventUID, &e.OrgUnitUID, &e.OrgUnitName, &e.District, &e.Region, &e.EventDate, &e.Status,
			&e.DistrictUID, &e.SousPrefecture, &e.SousPrefectureUID, &e.TypeCode, &e.TypeSource, &lat, &lng); err != nil {
			return nil, err
		}
		if lat.Valid && lng.Valid {
			e.Lat, e.Lng = &lat.Float64, &lng.Float64
		}
		index[e.EventUID] = len(events)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	vrows, err := s.db.Query(`SELECT event_uid, de_uid, value FROM event_value`)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	for vrows.Next() {
		var uid, de, val string
		if err := vrows.Scan(&uid, &de, &val); err != nil {
			return nil, err
		}
		if i, ok := index[uid]; ok {
			events[i].DataValues = append(events[i].DataValues, models.DataValue{DataElement: de, Value: val})
		}
	}
	return events, vrows.Err()
}

// ConformiteData is everything one recompute produces.
type ConformiteData struct {
	SetID      int64
	SetVersion int
	NRules     int
	Results    []normes.EventResult
	Summaries  []normes.SummaryRow
	Gaps       []normes.GapRow
}

// PersistConformite replaces every conformity table in one transaction.
func (s *Store) PersistConformite(d *ConformiteData) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"conformite_item", "event_conformite", "conformite_summary", "conformite_gap"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("clear %s: %w", t, err)
		}
	}

	itemStmt, err := tx.Prepare(`INSERT INTO conformite_item (event_uid, rule_id, type_code, kind, target, label, level, expected, observed, status) VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer itemStmt.Close()
	ecStmt, err := tx.Prepare(`INSERT INTO event_conformite (event_uid, set_id, n_rules, n_ok, n_manque, n_inconnu, n_manque_essentiel, score, conforme) VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ecStmt.Close()
	nEval, nConf := 0, 0
	for _, r := range d.Results {
		for _, it := range r.Items {
			if _, err := itemStmt.Exec(r.Event.EventUID, it.RuleID, it.TypeCode, it.Kind, it.Target, it.Label, it.Level, it.Expected, it.Observed, it.Status); err != nil {
				log.Printf("WARN: insert conformite_item: %v", err)
			}
		}
		sm := r.Summary
		conf := 0
		if sm.Conforme {
			conf = 1
			nConf++
		}
		if sm.Score != nil {
			nEval++
		}
		if _, err := ecStmt.Exec(r.Event.EventUID, d.SetID, sm.NRules, sm.NOk, sm.NManque, sm.NInconnu, sm.NManqueEssentiel, sm.Score, conf); err != nil {
			log.Printf("WARN: insert event_conformite: %v", err)
		}
	}

	sumStmt, err := tx.Prepare(`INSERT INTO conformite_summary (dimension, key, label, type_code, n_structures, n_evaluees, avg_score, n_conformes, pct_conformes) VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer sumStmt.Close()
	for _, r := range d.Summaries {
		if _, err := sumStmt.Exec(r.Dimension, r.Key, r.Label, r.TypeCode, r.NStructures, r.NEvaluees, r.AvgScore, r.NConformes, r.PctConforme); err != nil {
			log.Printf("WARN: insert conformite_summary: %v", err)
		}
	}
	gapStmt, err := tx.Prepare(`INSERT INTO conformite_gap (dimension, key, type_code, kind, target, label, level, n_concernees, n_manque, n_inconnu, deficit) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer gapStmt.Close()
	for _, g := range d.Gaps {
		if _, err := gapStmt.Exec(g.Dimension, g.Key, g.TypeCode, g.Kind, g.Target, g.Label, g.Level, g.NConcernees, g.NManque, g.NInconnu, g.Deficit); err != nil {
			log.Printf("WARN: insert conformite_gap: %v", err)
		}
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO conformite_run (id, set_id, set_version, n_rules, n_structures, n_evaluees, n_conformes, computed_at) VALUES (1,?,?,?,?,?,?,?)`,
		d.SetID, d.SetVersion, d.NRules, len(d.Results), nEval, nConf, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Lecture ------------------------------------------------------------------

// ConformiteRun describes the last recompute (nil when none happened).
type ConformiteRun struct {
	SetID       int64  `json:"set_id"`
	SetVersion  int    `json:"set_version"`
	NRules      int    `json:"n_rules"`
	NStructures int    `json:"n_structures"`
	NEvaluees   int    `json:"n_evaluees"`
	NConformes  int    `json:"n_conformes"`
	ComputedAt  string `json:"computed_at"`
}

func (s *Store) GetConformiteRun() (*ConformiteRun, error) {
	var r ConformiteRun
	err := s.db.QueryRow(`SELECT set_id, set_version, n_rules, n_structures, n_evaluees, n_conformes, computed_at FROM conformite_run WHERE id = 1`).
		Scan(&r.SetID, &r.SetVersion, &r.NRules, &r.NStructures, &r.NEvaluees, &r.NConformes, &r.ComputedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

// GetConformiteSummary returns rows of one dimension; typeCode "" = all-types rows
// (for dimension "type", every row is per type).
func (s *Store) GetConformiteSummary(dimension, typeCode string) ([]normes.SummaryRow, error) {
	switch dimension {
	case "global", "region", "district", "sous_prefecture", "type":
	default:
		dimension = "district"
	}
	query := `SELECT dimension, key, label, type_code, n_structures, n_evaluees, avg_score, n_conformes, pct_conformes FROM conformite_summary WHERE dimension = ?`
	args := []any{dimension}
	if dimension != "type" {
		query += ` AND type_code = ?`
		args = append(args, typeCode)
	}
	query += ` ORDER BY key`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []normes.SummaryRow{}
	for rows.Next() {
		var r normes.SummaryRow
		var avg, pct sql.NullFloat64
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.TypeCode, &r.NStructures, &r.NEvaluees, &avg, &r.NConformes, &pct); err != nil {
			return nil, err
		}
		if avg.Valid {
			r.AvgScore = &avg.Float64
		}
		if pct.Valid {
			r.PctConforme = &pct.Float64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GapParams filters the gap table.
type GapParams struct {
	Dimension string // global | region | district | sous_prefecture
	Key       string // "" = every key of the dimension
	TypeCode  string
	Kind      string
	Level     string
	Limit     int
}

func (s *Store) GetConformiteGaps(p GapParams) ([]normes.GapRow, error) {
	if p.Dimension == "" {
		p.Dimension = "global"
	}
	if p.Dimension == "global" {
		p.Key = "all"
	}
	where := []string{"dimension = ?", "n_manque > 0"}
	args := []any{p.Dimension}
	if p.Key != "" {
		where = append(where, "key = ?")
		args = append(args, p.Key)
	}
	if p.TypeCode != "" {
		where = append(where, "type_code = ?")
		args = append(args, p.TypeCode)
	}
	if p.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, p.Kind)
	}
	if p.Level != "" {
		where = append(where, "level = ?")
		args = append(args, p.Level)
	}
	query := `SELECT dimension, key, type_code, kind, target, label, level, n_concernees, n_manque, n_inconnu, deficit
		FROM conformite_gap WHERE ` + strings.Join(where, " AND ") + ` ORDER BY deficit DESC, n_manque DESC, label`
	if p.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", p.Limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []normes.GapRow{}
	for rows.Next() {
		var g normes.GapRow
		if err := rows.Scan(&g.Dimension, &g.Key, &g.TypeCode, &g.Kind, &g.Target, &g.Label, &g.Level, &g.NConcernees, &g.NManque, &g.NInconnu, &g.Deficit); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ConformiteStructureParams filters the per-structure list.
type ConformiteStructureParams struct {
	Region, District, SousPrefecture, TypeCode string
	Status                                     string // conforme | non_conforme | non_evalue | ""
	Search                                     string
	Page, PageSize                             int
}

type ConformiteStructureItem struct {
	EventUID         string   `json:"event_uid"`
	OrgUnitUID       string   `json:"org_unit_uid"`
	Name             string   `json:"name"`
	TypeCode         string   `json:"type_code"`
	Region           string   `json:"region"`
	District         string   `json:"district"`
	SousPrefecture   string   `json:"sous_prefecture"`
	Score            *float64 `json:"score"`
	Conforme         bool     `json:"conforme"`
	NManque          int      `json:"n_manque"`
	NManqueEssentiel int      `json:"n_manque_essentiel"`
	NInconnu         int      `json:"n_inconnu"`
	NRules           int      `json:"n_rules"`
}

type ConformiteStructureResult struct {
	Data     []ConformiteStructureItem `json:"data"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

func (s *Store) GetConformiteStructures(p ConformiteStructureParams) (*ConformiteStructureResult, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > 200 {
		p.PageSize = 25
	}
	where := []string{"1=1"}
	args := []any{}
	add := func(cond string, v string) {
		if v != "" {
			where = append(where, cond)
			args = append(args, v)
		}
	}
	add("e.region = ?", p.Region)
	add("e.district = ?", p.District)
	add("e.sous_prefecture = ?", p.SousPrefecture)
	add("e.type_code = ?", p.TypeCode)
	if p.Search != "" {
		where = append(where, "e.org_unit_name LIKE ?")
		args = append(args, "%"+p.Search+"%")
	}
	switch p.Status {
	case "conforme":
		where = append(where, "c.conforme = 1")
	case "non_conforme":
		where = append(where, "c.score IS NOT NULL AND c.conforme = 0")
	case "non_evalue":
		where = append(where, "c.score IS NULL")
	}
	whereClause := strings.Join(where, " AND ")
	from := ` FROM structure_latest e LEFT JOIN event_conformite c ON c.event_uid = e.event_uid WHERE ` + whereClause

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT e.event_uid, e.org_unit_uid, e.org_unit_name, COALESCE(e.type_code,''), e.region, e.district, COALESCE(e.sous_prefecture,''),
		c.score, COALESCE(c.conforme,0), COALESCE(c.n_manque,0), COALESCE(c.n_manque_essentiel,0), COALESCE(c.n_inconnu,0), COALESCE(c.n_rules,0)`+from+
		` ORDER BY (c.score IS NULL), c.score ASC, e.org_unit_name LIMIT ? OFFSET ?`, append(args, p.PageSize, (p.Page-1)*p.PageSize)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &ConformiteStructureResult{Data: []ConformiteStructureItem{}, Total: total, Page: p.Page, PageSize: p.PageSize}
	for rows.Next() {
		var it ConformiteStructureItem
		var score sql.NullFloat64
		var conf int
		if err := rows.Scan(&it.EventUID, &it.OrgUnitUID, &it.Name, &it.TypeCode, &it.Region, &it.District, &it.SousPrefecture, &score, &conf, &it.NManque, &it.NManqueEssentiel, &it.NInconnu, &it.NRules); err != nil {
			return nil, err
		}
		if score.Valid {
			it.Score = &score.Float64
		}
		it.Conforme = conf == 1
		res.Data = append(res.Data, it)
	}
	return res, rows.Err()
}

// EventConformite is the conformity block of the structure detail.
type EventConformite struct {
	Summary normes.Summary `json:"summary"`
	Items   []normes.Item  `json:"items"`
}

func (s *Store) GetEventConformite(eventUID string) (*EventConformite, error) {
	var ec EventConformite
	var score sql.NullFloat64
	var conf int
	err := s.db.QueryRow(`SELECT n_rules, n_ok, n_manque, n_inconnu, n_manque_essentiel, score, conforme FROM event_conformite WHERE event_uid = ?`, eventUID).
		Scan(&ec.Summary.NRules, &ec.Summary.NOk, &ec.Summary.NManque, &ec.Summary.NInconnu, &ec.Summary.NManqueEssentiel, &score, &conf)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if score.Valid {
		ec.Summary.Score = &score.Float64
	}
	ec.Summary.Conforme = conf == 1
	rows, err := s.db.Query(`SELECT rule_id, type_code, kind, target, label, level, expected, observed, status FROM conformite_item WHERE event_uid = ?
		ORDER BY CASE status WHEN 'manque' THEN 0 WHEN 'inconnu' THEN 1 ELSE 2 END, level, kind, label`, eventUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ec.Items = []normes.Item{}
	for rows.Next() {
		var it normes.Item
		var obs sql.NullFloat64
		if err := rows.Scan(&it.RuleID, &it.TypeCode, &it.Kind, &it.Target, &it.Label, &it.Level, &it.Expected, &obs, &it.Status); err != nil {
			return nil, err
		}
		if obs.Valid {
			it.Observed = &obs.Float64
		}
		ec.Items = append(ec.Items, it)
	}
	return &ec, rows.Err()
}

// ConformiteByOrgUnit returns avg score / pct conformes per administrative unit
// name for one level (3 = district, 4 = sous-préfecture), for the map.
func (s *Store) ConformiteByOrgUnit(level int) (map[string]normes.SummaryRow, error) {
	dim := map[int]string{3: "district", 4: "sous_prefecture"}[level]
	rows, err := s.GetConformiteSummary(dim, "")
	if err != nil {
		return nil, err
	}
	out := make(map[string]normes.SummaryRow, len(rows))
	for _, r := range rows {
		out[r.Key] = r
	}
	return out, nil
}
