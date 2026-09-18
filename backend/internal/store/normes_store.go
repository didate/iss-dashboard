package store

import (
	"database/sql"
	"fmt"
	"time"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/normes"
)

// --- Sets -------------------------------------------------------------------

const normeSetCols = `s.id, s.name, s.version, s.status, COALESCE(s.notes,''), s.created_at, COALESCE(s.created_by,''), COALESCE(s.activated_at,''),
	(SELECT COUNT(*) FROM norme_rule r WHERE r.set_id = s.id)`

func scanNormeSet(sc interface{ Scan(...any) error }) (*models.NormeSet, error) {
	var ns models.NormeSet
	if err := sc.Scan(&ns.ID, &ns.Name, &ns.Version, &ns.Status, &ns.Notes, &ns.CreatedAt, &ns.CreatedBy, &ns.ActivatedAt, &ns.NRules); err != nil {
		return nil, err
	}
	return &ns, nil
}

func (s *Store) ListNormeSets() ([]models.NormeSet, error) {
	rows, err := s.db.Query(`SELECT ` + normeSetCols + ` FROM norme_set s ORDER BY s.version DESC, s.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.NormeSet{}
	for rows.Next() {
		ns, err := scanNormeSet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ns)
	}
	return out, rows.Err()
}

func (s *Store) GetNormeSet(id int64) (*models.NormeSet, error) {
	ns, err := scanNormeSet(s.db.QueryRow(`SELECT `+normeSetCols+` FROM norme_set s WHERE s.id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ns, err
}

// GetActiveNormeSet returns nil, nil when no referential is active.
func (s *Store) GetActiveNormeSet() (*models.NormeSet, error) {
	ns, err := scanNormeSet(s.db.QueryRow(`SELECT ` + normeSetCols + ` FROM norme_set s WHERE s.status = 'active' ORDER BY s.id DESC LIMIT 1`))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return ns, err
}

// CreateNormeSet creates an empty draft with the next version number.
func (s *Store) CreateNormeSet(name, notes, createdBy string) (*models.NormeSet, error) {
	var next int
	s.db.QueryRow(`SELECT COALESCE(MAX(version),0)+1 FROM norme_set`).Scan(&next)
	res, err := s.db.Exec(`INSERT INTO norme_set (name, version, status, notes, created_at, created_by) VALUES (?,?,'draft',?,?,?)`,
		name, next, notes, time.Now().UTC().Format(time.RFC3339), createdBy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetNormeSet(id)
}

func (s *Store) UpdateNormeSet(id int64, name, notes string) error {
	_, err := s.db.Exec(`UPDATE norme_set SET name = ?, notes = ? WHERE id = ?`, name, notes, id)
	return err
}

// DuplicateNormeSet copies a set (and its rules) into a new draft version.
func (s *Store) DuplicateNormeSet(id int64, createdBy string) (*models.NormeSet, error) {
	src, err := s.GetNormeSet(id)
	if err != nil || src == nil {
		return nil, err
	}
	dup, err := s.CreateNormeSet(src.Name, src.Notes, createdBy)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`INSERT INTO norme_rule (set_id, type_code, kind, target, label, min_value, level)
		SELECT ?, type_code, kind, target, label, min_value, level FROM norme_rule WHERE set_id = ?`, dup.ID, id); err != nil {
		return nil, err
	}
	return s.GetNormeSet(dup.ID)
}

// DeleteNormeSet removes a draft; active/archived sets are kept for traceability.
func (s *Store) DeleteNormeSet(id int64) error {
	res, err := s.db.Exec(`DELETE FROM norme_set WHERE id = ? AND status = 'draft'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("seul un brouillon peut être supprimé")
	}
	_, err = s.db.Exec(`DELETE FROM norme_rule WHERE set_id = ?`, id) // FK cascade is off by default in SQLite
	return err
}

// ActivateNormeSet archives the current active set and activates the given one.
func (s *Store) ActivateNormeSet(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRow(`SELECT status FROM norme_set WHERE id = ?`, id).Scan(&status); err != nil {
		return fmt.Errorf("référentiel introuvable")
	}
	if status == "active" {
		return nil
	}
	if _, err := tx.Exec(`UPDATE norme_set SET status = 'archived' WHERE status = 'active'`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE norme_set SET status = 'active', activated_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Rules ------------------------------------------------------------------

func (s *Store) GetNormeRules(setID int64) ([]models.NormeRule, error) {
	rows, err := s.db.Query(`SELECT id, set_id, type_code, kind, target, label, min_value, level FROM norme_rule WHERE set_id = ?
		ORDER BY CASE type_code WHEN '*' THEN 0 WHEN 'PS' THEN 1 WHEN 'CS' THEN 2 WHEN 'CSA' THEN 3 WHEN 'CMC' THEN 4 WHEN 'HP' THEN 5 WHEN 'HR' THEN 6 WHEN 'HN' THEN 7 ELSE 8 END, kind, label`, setID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.NormeRule{}
	for rows.Next() {
		var r models.NormeRule
		if err := rows.Scan(&r.ID, &r.SetID, &r.TypeCode, &r.Kind, &r.Target, &r.Label, &r.MinValue, &r.Level); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplaceNormeRules replaces every rule of a set (grid edit / CSV import), atomically.
// Duplicate (type, kind, target) keep the last occurrence.
func (s *Store) ReplaceNormeRules(setID int64, rules []models.NormeRule) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM norme_rule WHERE set_id = ?`, setID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO norme_rule (set_id, type_code, kind, target, label, min_value, level) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rules {
		if _, err := stmt.Exec(setID, r.TypeCode, r.Kind, r.Target, r.Label, r.MinValue, r.Level); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Catalogue des cibles ---------------------------------------------------

// BuildNormeCatalog lists what a rule may target, from the metadata and
// aggregates already in SQLite (so it reflects the real ISS form).
func (s *Store) BuildNormeCatalog() (*normes.Catalog, error) {
	c := normes.NewCatalog()

	add := func(query, kind string, strip []string) error {
		rows, err := s.db.Query(query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var code, label string
			if err := rows.Scan(&code, &label); err != nil {
				return err
			}
			for _, p := range strip {
				if len(label) > len(p) && label[:len(p)] == p {
					label = label[len(p):]
				}
			}
			c.Add(normes.Target{Kind: kind, Code: code, Label: label})
		}
		return rows.Err()
	}

	if err := add(`SELECT code, COALESCE(NULLIF(form_name,''), name) FROM metadata_de
		WHERE section_prefix IN ('ISS_SVC','ISS_LAB') AND option_set_id = 'RGsTov6dBHH' AND code != ''`, normes.KindService, []string{"ISS_SVC ", "ISS_LAB "}); err != nil {
		return nil, err
	}
	if err := add(`SELECT code, COALESCE(NULLIF(form_name,''), name) FROM metadata_de
		WHERE section_prefix = 'ISS_INFRA' AND code != ''`, normes.KindInfra, []string{"ISS_INFRA "}); err != nil {
		return nil, err
	}
	if err := add(`SELECT DISTINCT profil_code, label FROM usage_rh WHERE district = 'all'`, normes.KindRH, nil); err != nil {
		return nil, err
	}
	if err := add(`SELECT DISTINCT equip_root, label FROM usage_equipement WHERE district = 'all'`, normes.KindEquipement, nil); err != nil {
		return nil, err
	}
	return c, nil
}
