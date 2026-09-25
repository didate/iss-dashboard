package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"iss-dashboard-backend/internal/drh"
)

// DrhImport is one millésime of the civil-service personnel file.
type DrhImport struct {
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	Annee        int    `json:"annee"`
	Status       string `json:"status"` // active | archived
	AgeRetraite  int    `json:"age_retraite"`
	NAgents      int    `json:"n_agents"`
	NStructure   int    `json:"n_structure"`
	NBureau      int    `json:"n_bureau"`
	NCentrale    int    `json:"n_centrale"`
	NNonRattache int    `json:"n_non_rattache"`
	NStructures  int    `json:"n_structures"`
	ImportedAt   string `json:"imported_at"`
	ImportedBy   string `json:"imported_by"`
	SourceFile   string `json:"source_file"`
}

const drhImportCols = `id, label, annee, status, age_retraite, n_agents, n_structure, n_bureau,
	n_centrale, n_non_rattache, n_structures, imported_at, COALESCE(imported_by,''), COALESCE(source_file,'')`

func scanDrhImport(sc interface{ Scan(...any) error }) (*DrhImport, error) {
	var im DrhImport
	err := sc.Scan(&im.ID, &im.Label, &im.Annee, &im.Status, &im.AgeRetraite, &im.NAgents, &im.NStructure,
		&im.NBureau, &im.NCentrale, &im.NNonRattache, &im.NStructures, &im.ImportedAt, &im.ImportedBy, &im.SourceFile)
	if err != nil {
		return nil, err
	}
	return &im, nil
}

func (s *Store) ListDrhImports() ([]DrhImport, error) {
	rows, err := s.db.Query(`SELECT ` + drhImportCols + ` FROM drh_import ORDER BY annee DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DrhImport{}
	for rows.Next() {
		im, err := scanDrhImport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *im)
	}
	return out, rows.Err()
}

// GetActiveDrhImport returns nil, nil when no personnel file has been imported.
func (s *Store) GetActiveDrhImport() (*DrhImport, error) {
	im, err := scanDrhImport(s.db.QueryRow(`SELECT ` + drhImportCols + ` FROM drh_import WHERE status = 'active' ORDER BY id DESC LIMIT 1`))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return im, err
}

// ListDrhStructures returns the ISS facilities the resolver matches against.
func (s *Store) ListDrhStructures() ([]drh.Structure, error) {
	rows, err := s.db.Query(`SELECT org_unit_uid, org_unit_name, COALESCE(district,''), COALESCE(region,''), COALESCE(type_code,'')
		FROM structure_latest`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []drh.Structure
	for rows.Next() {
		var st drh.Structure
		if err := rows.Scan(&st.UID, &st.Name, &st.District, &st.Region, &st.TypeCode); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// --- Correspondances --------------------------------------------------------

func (s *Store) ListDrhCorrespondances() ([]drh.Correspondance, error) {
	rows, err := s.db.Query(`SELECT libelle_norm, libelle_drh, COALESCE(org_unit_uid,''), statut, COALESCE(district,'')
		FROM drh_correspondance ORDER BY libelle_drh`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []drh.Correspondance{}
	for rows.Next() {
		var c drh.Correspondance
		if err := rows.Scan(&c.LibelleNorm, &c.LibelleDRH, &c.OrgUnitUID, &c.Statut, &c.District); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ReplaceDrhCorrespondances swaps the whole table atomically: it is edited as a
// file, reviewed with the ministry, and re-imported as a whole.
func (s *Store) ReplaceDrhCorrespondances(corr []drh.Correspondance) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM drh_correspondance`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_correspondance (libelle_norm, libelle_drh, org_unit_uid, statut, district) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range corr {
		if _, err := stmt.Exec(c.LibelleNorm, c.LibelleDRH, c.OrgUnitUID, c.Statut, c.District); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SeedDrhCorrespondances fills the correspondence table from the embedded
// reference the first time only: once an admin has edited it, it is theirs.
func (s *Store) SeedDrhCorrespondances() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM drh_correspondance`).Scan(&n); err != nil || n > 0 {
		return err
	}
	corr, errs := drh.SeedCorrespondances()
	if len(errs) > 0 {
		return fmt.Errorf("table de correspondances embarquée invalide : %s", errs[0].Message)
	}
	if err := s.ReplaceDrhCorrespondances(corr); err != nil {
		return err
	}
	log.Printf("[DRH] table de correspondances initialisée : %d entrées", len(corr))
	return nil
}

// --- Import -----------------------------------------------------------------

// SaveDrhImport persists one millésime and its aggregates in a single
// transaction, and activates it: a failed import never replaces the previous
// snapshot. Nothing individual is written — only the cells built by drh.Aggregate.
func (s *Store) SaveDrhImport(im DrhImport, eff []drh.EffectifRow, pyr []drh.PyramideRow, inconnus []drh.Inconnu) (*DrhImport, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if im.ImportedAt == "" {
		im.ImportedAt = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := tx.Exec(`INSERT INTO drh_import (label, annee, status, age_retraite, n_agents, n_structure, n_bureau,
		n_centrale, n_non_rattache, n_structures, imported_at, imported_by, source_file)
		VALUES (?,?,'active',?,?,?,?,?,?,?,?,?,?)`,
		im.Label, im.Annee, im.AgeRetraite, im.NAgents, im.NStructure, im.NBureau,
		im.NCentrale, im.NNonRattache, im.NStructures, im.ImportedAt, im.ImportedBy, im.SourceFile)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	effStmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_effectif (import_id, dimension, key, label, district, region,
		categorie, n_agents, n_femmes, n_depart_5ans, n_depart_10ans) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return nil, err
	}
	defer effStmt.Close()
	for _, r := range eff {
		if _, err := effStmt.Exec(id, r.Dimension, r.Key, r.Label, r.District, r.Region,
			r.Categorie, r.NAgents, r.NFemmes, r.NDepart5Ans, r.NDepart10Ans); err != nil {
			return nil, fmt.Errorf("effectif %s/%s: %w", r.Dimension, r.Key, err)
		}
	}

	pyrStmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_pyramide (import_id, dimension, key, categorie, tranche, n_agents, n_femmes) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return nil, err
	}
	defer pyrStmt.Close()
	for _, r := range pyr {
		if _, err := pyrStmt.Exec(id, r.Dimension, r.Key, r.Categorie, r.Tranche, r.NAgents, r.NFemmes); err != nil {
			return nil, fmt.Errorf("pyramide %s/%s: %w", r.Dimension, r.Key, err)
		}
	}

	incStmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_non_reconnu (import_id, libelle_drh, prefecture, n_agents) VALUES (?,?,?,?)`)
	if err != nil {
		return nil, err
	}
	defer incStmt.Close()
	for _, u := range inconnus {
		if _, err := incStmt.Exec(id, u.Libelle, u.Prefecture, u.NAgents); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(`UPDATE drh_import SET status = 'archived' WHERE id != ? AND status = 'active'`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetDrhImport(id)
}

func (s *Store) GetDrhImport(id int64) (*DrhImport, error) {
	im, err := scanDrhImport(s.db.QueryRow(`SELECT `+drhImportCols+` FROM drh_import WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return im, err
}

// ActivateDrhImport makes a millésime the one the screens read.
func (s *Store) ActivateDrhImport(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM drh_import WHERE id = ?`, id).Scan(&n); err != nil || n == 0 {
		return fmt.Errorf("import introuvable")
	}
	if _, err := tx.Exec(`UPDATE drh_import SET status = 'archived' WHERE status = 'active'`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE drh_import SET status = 'active' WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteDrhImport removes a millésime and everything derived from it.
func (s *Store) DeleteDrhImport(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"drh_effectif", "drh_pyramide", "drh_comparaison", "drh_non_reconnu", "drh_import"} {
		where := "import_id = ?"
		if t == "drh_import" {
			where = "id = ?"
		}
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE `+where, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetDrhNonReconnus lists the labels the import could not attach.
func (s *Store) GetDrhNonReconnus(importID int64) ([]drh.Inconnu, error) {
	rows, err := s.db.Query(`SELECT libelle_drh, COALESCE(prefecture,''), n_agents FROM drh_non_reconnu
		WHERE import_id = ? ORDER BY n_agents DESC, libelle_drh`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []drh.Inconnu{}
	for rows.Next() {
		var u drh.Inconnu
		if err := rows.Scan(&u.Libelle, &u.Prefecture, &u.NAgents); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
