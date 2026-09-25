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
	rows, err := s.db.Query(`SELECT org_unit_uid, org_unit_name, COALESCE(district,''), COALESCE(region,''), COALESCE(type_code,''),
		COALESCE(sous_prefecture,''), COALESCE(sous_prefecture_uid,'') FROM structure_latest`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []drh.Structure
	for rows.Next() {
		var st drh.Structure
		if err := rows.Scan(&st.UID, &st.Name, &st.District, &st.Region, &st.TypeCode, &st.SousPrefecture, &st.SousPrefectureUID); err != nil {
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
		categorie, n_agents, n_femmes, n_depart_5ans, n_depart_10ans, n_age_connu) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return nil, err
	}
	defer effStmt.Close()
	for _, r := range eff {
		if _, err := effStmt.Exec(id, r.Dimension, r.Key, r.Label, r.District, r.Region,
			r.Categorie, r.NAgents, r.NFemmes, r.NDepart5Ans, r.NDepart10Ans, r.NAgeConnu); err != nil {
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

// --- Rollups : entrées du recalcul ------------------------------------------

// GetDrhFineCells returns the cells written at import time — the only ones the
// rollups may be rebuilt from.
func (s *Store) GetDrhFineCells(importID int64) ([]drh.EffectifRow, []drh.PyramideRow, error) {
	rows, err := s.db.Query(`SELECT dimension, key, label, district, region, categorie,
		n_agents, n_femmes, n_depart_5ans, n_depart_10ans, n_age_connu
		FROM drh_effectif WHERE import_id = ? AND dimension IN ('structure','bureau','centrale','non_rattache')`, importID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var eff []drh.EffectifRow
	for rows.Next() {
		var r drh.EffectifRow
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.District, &r.Region, &r.Categorie,
			&r.NAgents, &r.NFemmes, &r.NDepart5Ans, &r.NDepart10Ans, &r.NAgeConnu); err != nil {
			return nil, nil, err
		}
		eff = append(eff, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	prows, err := s.db.Query(`SELECT dimension, key, categorie, tranche, n_agents, n_femmes
		FROM drh_pyramide WHERE import_id = ? AND dimension IN ('structure','bureau','centrale','non_rattache')`, importID)
	if err != nil {
		return nil, nil, err
	}
	defer prows.Close()
	var pyr []drh.PyramideRow
	for prows.Next() {
		var r drh.PyramideRow
		if err := prows.Scan(&r.Dimension, &r.Key, &r.Categorie, &r.Tranche, &r.NAgents, &r.NFemmes); err != nil {
			return nil, nil, err
		}
		pyr = append(pyr, r)
	}
	return eff, pyr, prows.Err()
}

// GetDrhPopulationIndex keys the total population the way the rollups do:
// national by level, regions and districts by name (as the ISS hierarchy names
// them), sous-préfectures by org unit UID since their names repeat.
func (s *Store) GetDrhPopulationIndex() (map[string]float64, error) {
	rows, err := s.db.Query(`SELECT o.level, o.uid, o.name, p.value FROM population p
		JOIN org_unit o ON o.uid = p.ou_uid WHERE p.indicator = 'total' AND p.value > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var level int
		var uid, name string
		var value float64
		if err := rows.Scan(&level, &uid, &name, &value); err != nil {
			return nil, err
		}
		switch level {
		case 1:
			out[drh.PopKey(drh.DimGlobal, drh.KeyNational)] = value
		case 2:
			out[drh.PopKey(drh.DimRegion, name)] = value
		case 3:
			out[drh.PopKey(drh.DimDistrict, name)] = value
		case 4:
			out[drh.PopKey(drh.DimSousPrefecture, uid)] = value
		}
	}
	return out, rows.Err()
}

// GetIssRH returns the headcounts declared by the facilities in ISS, the other
// side of the comparison.
func (s *Store) GetIssRH() ([]drh.ISSRH, error) {
	rows, err := s.db.Query(`SELECT district, profil_code, effectif_total FROM usage_rh`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []drh.ISSRH
	for rows.Next() {
		var r drh.ISSRH
		if err := rows.Scan(&r.District, &r.ProfilCode, &r.Effectif); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplaceDrhRollups swaps the derived cells of one millésime in a single
// transaction, leaving the import-time cells untouched.
func (s *Store) ReplaceDrhRollups(importID int64, eff []drh.EffectifRow, pyr []drh.PyramideRow, comp []drh.ComparaisonRow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM drh_effectif WHERE import_id = ?
		AND dimension NOT IN ('structure','bureau','centrale','non_rattache')`, importID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM drh_pyramide WHERE import_id = ?
		AND dimension NOT IN ('structure','bureau','centrale','non_rattache')`, importID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM drh_comparaison WHERE import_id = ?`, importID); err != nil {
		return err
	}

	effStmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_effectif (import_id, dimension, key, label, district, region,
		categorie, n_agents, n_femmes, n_structure, n_bureau, n_centrale, n_non_rattache,
		n_depart_5ans, n_depart_10ans, n_age_connu, population, ratio_10k) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer effStmt.Close()
	for _, r := range eff {
		if _, err := effStmt.Exec(importID, r.Dimension, r.Key, r.Label, r.District, r.Region, r.Categorie,
			r.NAgents, r.NFemmes, r.NStructure, r.NBureau, r.NCentrale, r.NNonRattache,
			r.NDepart5Ans, r.NDepart10Ans, r.NAgeConnu, r.Population, r.Ratio10k); err != nil {
			return fmt.Errorf("rollup %s/%s: %w", r.Dimension, r.Key, err)
		}
	}

	pyrStmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_pyramide (import_id, dimension, key, categorie, tranche, n_agents, n_femmes) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer pyrStmt.Close()
	for _, r := range pyr {
		if _, err := pyrStmt.Exec(importID, r.Dimension, r.Key, r.Categorie, r.Tranche, r.NAgents, r.NFemmes); err != nil {
			return err
		}
	}

	compStmt, err := tx.Prepare(`INSERT OR REPLACE INTO drh_comparaison (import_id, dimension, key, label, categorie,
		n_drh, n_iss, ecart, ratio) VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer compStmt.Close()
	for _, r := range comp {
		if _, err := compStmt.Exec(importID, r.Dimension, r.Key, r.Label, r.Categorie,
			r.NDrh, r.NIss, r.Ecart, r.Ratio); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Lectures (espace planification) ----------------------------------------

// DrhEffectifParams filters the pre-computed headcount cells.
type DrhEffectifParams struct {
	Dimension string // global | region | district | sous_prefecture | type
	Key       string // une seule zone, vide = toutes
	Categorie string // "" = toutes professions, "*" = toutes les catégories détaillées
	District  string // restreint les sous-préfectures à un district
}

const drhEffectifCols = `dimension, key, label, district, region, categorie, n_agents, n_femmes,
	n_structure, n_bureau, n_centrale, n_non_rattache, n_depart_5ans, n_depart_10ans, n_age_connu, population, ratio_10k`

func scanDrhEffectifs(rows *sql.Rows) ([]drh.EffectifRow, error) {
	defer rows.Close()
	out := []drh.EffectifRow{}
	for rows.Next() {
		var r drh.EffectifRow
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.District, &r.Region, &r.Categorie,
			&r.NAgents, &r.NFemmes, &r.NStructure, &r.NBureau, &r.NCentrale, &r.NNonRattache,
			&r.NDepart5Ans, &r.NDepart10Ans, &r.NAgeConnu, &r.Population, &r.Ratio10k); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDrhEffectifs serves the cells as they were computed, sorted by headcount.
func (s *Store) GetDrhEffectifs(importID int64, p DrhEffectifParams) ([]drh.EffectifRow, error) {
	q := `SELECT ` + drhEffectifCols + ` FROM drh_effectif WHERE import_id = ? AND dimension = ?`
	args := []any{importID, p.Dimension}
	switch p.Categorie {
	case "*":
		q += ` AND categorie != ''`
	default:
		q += ` AND categorie = ?`
		args = append(args, p.Categorie)
	}
	if p.Key != "" {
		q += ` AND key = ?`
		args = append(args, p.Key)
	}
	if p.District != "" {
		q += ` AND district = ?`
		args = append(args, p.District)
	}
	q += ` ORDER BY n_agents DESC, label`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	return scanDrhEffectifs(rows)
}

// GetDrhPyramide returns the age brackets of one place, in display order.
func (s *Store) GetDrhPyramide(importID int64, dimension, key, categorie string) ([]drh.PyramideRow, error) {
	rows, err := s.db.Query(`SELECT dimension, key, categorie, tranche, n_agents, n_femmes FROM drh_pyramide
		WHERE import_id = ? AND dimension = ? AND key = ? AND categorie = ?`, importID, dimension, key, categorie)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byTranche := map[string]drh.PyramideRow{}
	for rows.Next() {
		var r drh.PyramideRow
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Categorie, &r.Tranche, &r.NAgents, &r.NFemmes); err != nil {
			return nil, err
		}
		byTranche[r.Tranche] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Les tranches vides sont renvoyées à zéro : un histogramme troué se lit mal.
	out := make([]drh.PyramideRow, 0, len(drh.Tranches))
	for _, t := range drh.Tranches {
		if r, ok := byTranche[t]; ok {
			out = append(out, r)
			continue
		}
		out = append(out, drh.PyramideRow{Dimension: dimension, Key: key, Categorie: categorie, Tranche: t})
	}
	return out, nil
}

// GetDrhComparaison serves the DRH ↔ ISS confrontation, worst gaps first.
// An empty key spans every zone of the dimension.
func (s *Store) GetDrhComparaison(importID int64, dimension, key, categorie string) ([]drh.ComparaisonRow, error) {
	q := `SELECT dimension, key, label, categorie, n_drh, n_iss, ecart, ratio FROM drh_comparaison
		WHERE import_id = ? AND dimension = ?`
	args := []any{importID, dimension}
	if key != "" {
		q += ` AND key = ?`
		args = append(args, key)
	}
	if categorie == "*" {
		q += ` AND categorie != ''`
	} else {
		q += ` AND categorie = ?`
		args = append(args, categorie)
	}
	q += ` ORDER BY ratio IS NULL, ratio ASC, n_drh DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []drh.ComparaisonRow{}
	for rows.Next() {
		var r drh.ComparaisonRow
		if err := rows.Scan(&r.Dimension, &r.Key, &r.Label, &r.Categorie, &r.NDrh, &r.NIss, &r.Ecart, &r.Ratio); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DrhStructureRow is one facility with the state agents posted to it.
type DrhStructureRow struct {
	OrgUnitUID  string `json:"org_unit_uid"`
	Name        string `json:"name"`
	TypeCode    string `json:"type_code"`
	District    string `json:"district"`
	Region      string `json:"region"`
	NAgents     int    `json:"n_agents"`
	NFemmes     int    `json:"n_femmes"`
	NDepart5Ans int    `json:"n_depart_5ans"`
}

// GetDrhStructuresList lists the facilities of a district with their state
// headcount, including those with none: a facility without a single paid agent
// is exactly what a planner is looking for.
func (s *Store) GetDrhStructuresList(importID int64, district, search string) ([]DrhStructureRow, error) {
	q := `SELECT s.org_unit_uid, s.org_unit_name, COALESCE(s.type_code,''), COALESCE(s.district,''), COALESCE(s.region,''),
		COALESCE(e.n_agents,0), COALESCE(e.n_femmes,0), COALESCE(e.n_depart_5ans,0)
		FROM structure_latest s
		LEFT JOIN drh_effectif e ON e.import_id = ? AND e.dimension = 'structure' AND e.categorie = '' AND e.key = s.org_unit_uid
		WHERE 1 = 1`
	args := []any{importID}
	if district != "" {
		q += ` AND s.district = ?`
		args = append(args, district)
	}
	if search != "" {
		q += ` AND s.org_unit_name LIKE ?`
		args = append(args, "%"+search+"%")
	}
	q += ` ORDER BY COALESCE(e.n_agents,0) DESC, s.org_unit_name LIMIT 2000`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DrhStructureRow{}
	for rows.Next() {
		var r DrhStructureRow
		if err := rows.Scan(&r.OrgUnitUID, &r.Name, &r.TypeCode, &r.District, &r.Region,
			&r.NAgents, &r.NFemmes, &r.NDepart5Ans); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDrhStructureCategories returns the state headcount of one facility, by
// category — the block shown on a facility's page.
func (s *Store) GetDrhStructureCategories(importID int64, ouUID string) ([]drh.EffectifRow, error) {
	rows, err := s.db.Query(`SELECT `+drhEffectifCols+` FROM drh_effectif
		WHERE import_id = ? AND dimension = 'structure' AND key = ? AND categorie != ''
		ORDER BY n_agents DESC`, importID, ouUID)
	if err != nil {
		return nil, err
	}
	return scanDrhEffectifs(rows)
}
