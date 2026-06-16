package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"iss-dashboard-backend/internal/models"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(migrationSQL); err != nil {
		return err
	}
	// Clean up orphan "running" sync_runs from previous crashes
	s.db.Exec(`UPDATE sync_run SET status='error', error_text='interrupted by restart' WHERE status='running'`)
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// --- Sync Run ---

func (s *Store) CreateSyncRun() (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO sync_run (started_at, status) VALUES (?, 'running')`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishSyncRun(id int64, status string, eventsPulled, issuesFound int, durationMs int64, errText string) error {
	_, err := s.db.Exec(
		`UPDATE sync_run SET finished_at=?, status=?, events_pulled=?, issues_found=?, duration_ms=?, error_text=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), status, eventsPulled, issuesFound, durationMs, errText, id,
	)
	return err
}

func (s *Store) GetLastSyncRun() (*models.SyncRun, error) {
	row := s.db.QueryRow(`SELECT id, started_at, COALESCE(finished_at,''), status, events_pulled, issues_found, duration_ms, COALESCE(error_text,'') FROM sync_run ORDER BY id DESC LIMIT 1`)
	sr, err := scanSyncRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sr, err
}

func (s *Store) GetSyncRunHistory(limit int) ([]models.SyncRun, error) {
	rows, err := s.db.Query(`SELECT id, started_at, COALESCE(finished_at,''), status, events_pulled, issues_found, duration_ms, COALESCE(error_text,'') FROM sync_run ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []models.SyncRun
	for rows.Next() {
		sr, err := scanSyncRunRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *sr)
	}
	return runs, rows.Err()
}

func (s *Store) GetRunningSyncRun() (*models.SyncRun, error) {
	row := s.db.QueryRow(`SELECT id, started_at, COALESCE(finished_at,''), status, events_pulled, issues_found, duration_ms, COALESCE(error_text,'') FROM sync_run WHERE status='running' ORDER BY id DESC LIMIT 1`)
	sr, err := scanSyncRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sr, err
}

func scanSyncRun(row *sql.Row) (*models.SyncRun, error) {
	var sr models.SyncRun
	var startedAt, finishedAt string
	err := row.Scan(&sr.ID, &startedAt, &finishedAt, &sr.Status, &sr.EventsPulled, &sr.IssuesFound, &sr.DurationMs, &sr.ErrorText)
	if err != nil {
		return nil, err
	}
	sr.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if finishedAt != "" {
		t, _ := time.Parse(time.RFC3339, finishedAt)
		sr.FinishedAt = &t
	}
	return &sr, nil
}

func scanSyncRunRows(rows *sql.Rows) (*models.SyncRun, error) {
	var sr models.SyncRun
	var startedAt, finishedAt string
	err := rows.Scan(&sr.ID, &startedAt, &finishedAt, &sr.Status, &sr.EventsPulled, &sr.IssuesFound, &sr.DurationMs, &sr.ErrorText)
	if err != nil {
		return nil, err
	}
	sr.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if finishedAt != "" {
		t, _ := time.Parse(time.RFC3339, finishedAt)
		sr.FinishedAt = &t
	}
	return &sr, nil
}

// --- Metadata ---

func (s *Store) UpsertMetadataDE(des []models.DataElementMeta) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO metadata_de (de_uid, code, name, value_type, option_set_id, section_prefix) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, de := range des {
		if _, err := stmt.Exec(de.UID, de.Code, de.Name, de.ValueType, de.OptionSetID, de.SectionPrefix); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertOptionSets(opts []models.OptionEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM option_set`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO option_set (option_set_id, option_code, option_name) VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, o := range opts {
		if _, err := stmt.Exec(o.OptionSetID, o.Code, o.Name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertOrgUnits(units []models.OrgUnit) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO org_unit (uid, name, level, parent_uid, parent_name) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ou := range units {
		if _, err := stmt.Exec(ou.UID, ou.Name, ou.Level, ou.ParentUID, ou.ParentName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetAllMetadataDE() ([]models.DataElementMeta, error) {
	rows, err := s.db.Query(`SELECT de_uid, COALESCE(code,''), name, COALESCE(value_type,''), COALESCE(option_set_id,''), COALESCE(section_prefix,'') FROM metadata_de`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DataElementMeta
	for rows.Next() {
		var de models.DataElementMeta
		if err := rows.Scan(&de.UID, &de.Code, &de.Name, &de.ValueType, &de.OptionSetID, &de.SectionPrefix); err != nil {
			return nil, err
		}
		out = append(out, de)
	}
	return out, rows.Err()
}

func (s *Store) GetAllOptionEntries() ([]models.OptionEntry, error) {
	rows, err := s.db.Query(`SELECT option_set_id, option_code, option_name FROM option_set`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OptionEntry
	for rows.Next() {
		var o models.OptionEntry
		if err := rows.Scan(&o.OptionSetID, &o.Code, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetAllOrgUnits() ([]models.OrgUnit, error) {
	rows, err := s.db.Query(`SELECT uid, name, level, COALESCE(parent_uid,''), COALESCE(parent_name,'') FROM org_unit`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OrgUnit
	for rows.Next() {
		var ou models.OrgUnit
		if err := rows.Scan(&ou.UID, &ou.Name, &ou.Level, &ou.ParentUID, &ou.ParentName); err != nil {
			return nil, err
		}
		out = append(out, ou)
	}
	return out, rows.Err()
}

// IssueWithEvent ties an issue to its event UID.
type IssueWithEvent struct {
	EventUID string
	Issue    models.Issue
}

// SyncData holds everything computed by one sync run, to be persisted atomically.
type SyncData struct {
	Events           []models.Event
	Issues           []IssueWithEvent
	EventQualities   []models.EventQuality
	QualitySummaries []models.QualitySummaryRow
	UsageRecensement []models.UsageRecensement
	UsageServices    []models.UsageService
	UsageEquipements []models.UsageEquipement
	UsageRH          []models.UsageRH
	UsageCommodites  []models.UsageCommodite
	ReportingRates   []models.ReportingRate
}

// PersistSyncData atomically replaces all derived data within a single transaction.
func (s *Store) PersistSyncData(syncRunID int64, data *SyncData) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear derived tables (order matters for FK)
	for _, table := range []string{"event_value", "quality_issue", "event_quality", "quality_summary", "usage_recensement", "usage_service", "usage_equipement", "usage_rh", "usage_commodite", "reporting_rate", "event"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	// Build code map from metadata
	codeMap := make(map[string]string)
	metaDes, _ := s.GetAllMetadataDE()
	for _, de := range metaDes {
		codeMap[de.UID] = de.Code
	}

	// Events + values
	evtStmt, err := tx.Prepare(`INSERT INTO event (event_uid, org_unit_uid, org_unit_name, district, region, event_date, status, raw_json, sync_run_id) VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer evtStmt.Close()

	valStmt, err := tx.Prepare(`INSERT OR REPLACE INTO event_value (event_uid, de_uid, de_code, value) VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer valStmt.Close()

	for _, evt := range data.Events {
		if _, err := evtStmt.Exec(evt.EventUID, evt.OrgUnitUID, evt.OrgUnitName, evt.District, evt.Region, evt.EventDate, evt.Status, evt.RawJSON, syncRunID); err != nil {
			log.Printf("WARN: insert event %s: %v", evt.EventUID, err)
			continue
		}
		for _, dv := range evt.DataValues {
			code := codeMap[dv.DataElement]
			if _, err := valStmt.Exec(evt.EventUID, dv.DataElement, code, dv.Value); err != nil {
				log.Printf("WARN: insert event_value %s/%s: %v", evt.EventUID, dv.DataElement, err)
			}
		}
	}

	// Quality issues
	qiStmt, err := tx.Prepare(`INSERT INTO quality_issue (event_uid, rule_code, severity, rule_name, message, sync_run_id) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer qiStmt.Close()
	for _, iwe := range data.Issues {
		if _, err := qiStmt.Exec(iwe.EventUID, iwe.Issue.RuleCode, iwe.Issue.Severity, iwe.Issue.RuleName, iwe.Issue.Message, syncRunID); err != nil {
			log.Printf("WARN: insert quality_issue: %v", err)
		}
	}

	// Event quality
	eqStmt, err := tx.Prepare(`INSERT INTO event_quality (event_uid, n_error, n_warning, n_info, worst_severity, score) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer eqStmt.Close()
	for _, eq := range data.EventQualities {
		if _, err := eqStmt.Exec(eq.EventUID, eq.NError, eq.NWarning, eq.NInfo, eq.WorstSeverity, eq.Score); err != nil {
			log.Printf("WARN: insert event_quality: %v", err)
		}
	}

	// Quality summary
	qsStmt, err := tx.Prepare(`INSERT INTO quality_summary (dimension, key, label, avg_score, n_error, n_warning, n_info, n_structures) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer qsStmt.Close()
	for _, qs := range data.QualitySummaries {
		if _, err := qsStmt.Exec(qs.Dimension, qs.Key, qs.Label, qs.AvgScore, qs.NError, qs.NWarning, qs.NInfo, qs.NStructures); err != nil {
			log.Printf("WARN: insert quality_summary: %v", err)
		}
	}

	// Usage recensement
	urStmt, err := tx.Prepare(`INSERT INTO usage_recensement (dimension, key, label, n_structures, n_operationnel, n_non_operationnel, n_ferme_temp) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer urStmt.Close()
	for _, ur := range data.UsageRecensement {
		if _, err := urStmt.Exec(ur.Dimension, ur.Key, ur.Label, ur.NStructures, ur.NOperationnel, ur.NNonOperationnel, ur.NFermeTemp); err != nil {
			log.Printf("WARN: insert usage_recensement: %v", err)
		}
	}

	// Usage services
	usStmt, err := tx.Prepare(`INSERT INTO usage_service (service_code, service_label, district, n_oui, n_oui_pas_fonc, n_non, n_total, pct_fonctionnel) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer usStmt.Close()
	for _, us := range data.UsageServices {
		if _, err := usStmt.Exec(us.ServiceCode, us.ServiceLabel, us.District, us.NOui, us.NOuiPasFonc, us.NNon, us.NTotal, us.PctFonctionnel); err != nil {
			log.Printf("WARN: insert usage_service: %v", err)
		}
	}

	// Usage equipements
	ueStmt, err := tx.Prepare(`INSERT INTO usage_equipement (equip_root, label, district, sum_total, sum_fonct, pct_fonct, category) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ueStmt.Close()
	for _, ue := range data.UsageEquipements {
		if _, err := ueStmt.Exec(ue.EquipRoot, ue.Label, ue.District, ue.SumTotal, ue.SumFonct, ue.PctFonct, ue.Category); err != nil {
			log.Printf("WARN: insert usage_equipement: %v", err)
		}
	}

	// Usage RH
	rhStmt, err := tx.Prepare(`INSERT INTO usage_rh (profil_code, label, district, effectif_fonc, effectif_contr, effectif_benev, effectif_total) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer rhStmt.Close()
	for _, rh := range data.UsageRH {
		if _, err := rhStmt.Exec(rh.ProfilCode, rh.Label, rh.District, rh.EffectifFonc, rh.EffectifContr, rh.EffectifBenev, rh.EffectifTotal); err != nil {
			log.Printf("WARN: insert usage_rh: %v", err)
		}
	}

	// Usage commodites
	ucStmt, err := tx.Prepare(`INSERT INTO usage_commodite (indicator, district, n_oui, n_total, pct) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ucStmt.Close()
	for _, uc := range data.UsageCommodites {
		if _, err := ucStmt.Exec(uc.Indicator, uc.District, uc.NOui, uc.NTotal, uc.Pct); err != nil {
			log.Printf("WARN: insert usage_commodite: %v", err)
		}
	}

	// Reporting rates
	rrStmt, err := tx.Prepare(`INSERT INTO reporting_rate (dimension, key, label, n_expected, n_reported, pct) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer rrStmt.Close()
	for _, rr := range data.ReportingRates {
		if _, err := rrStmt.Exec(rr.Dimension, rr.Key, rr.Label, rr.NExpected, rr.NReported, rr.Pct); err != nil {
			log.Printf("WARN: insert reporting_rate: %v", err)
		}
	}

	return tx.Commit()
}
