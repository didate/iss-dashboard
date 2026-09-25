package store

import (
	"path/filepath"
	"testing"

	"iss-dashboard-backend/internal/drh"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("ouverture de la base : %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// La table de correspondances embarquée doit être valide : elle est chargée au
// démarrage d'une base neuve, un fichier cassé passerait autrement inaperçu.
func TestSeedDrhCorrespondances(t *testing.T) {
	st := testStore(t)
	corr, err := st.ListDrhCorrespondances()
	if err != nil {
		t.Fatal(err)
	}
	if len(corr) < 20 {
		t.Fatalf("%d correspondances chargées, attendu la table validée avec le MSHP", len(corr))
	}
	var ok, bureau int
	for _, c := range corr {
		switch c.Statut {
		case drh.CorrOK:
			ok++
			if c.OrgUnitUID == "" {
				t.Errorf("%q : statut ok sans UID", c.LibelleDRH)
			}
		case drh.CorrBureau:
			bureau++
		}
	}
	if ok == 0 || bureau == 0 {
		t.Errorf("seed incomplet : %d rattachées, %d bureaux", ok, bureau)
	}

	// Un seed ne doit jamais écraser le travail de l'admin.
	if err := st.ReplaceDrhCorrespondances([]drh.Correspondance{
		{LibelleNorm: "x", LibelleDRH: "X", Statut: drh.CorrNonRattache}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SeedDrhCorrespondances(); err != nil {
		t.Fatal(err)
	}
	if corr, _ := st.ListDrhCorrespondances(); len(corr) != 1 {
		t.Errorf("le seed a écrasé la table éditée : %d entrées", len(corr))
	}
}

func TestSaveDrhImport(t *testing.T) {
	st := testStore(t)
	if _, err := st.db.Exec(`INSERT INTO event (event_uid, org_unit_uid, org_unit_name, district, region, event_date, type_code)
		VALUES ('e1','u1','HR Kankan','DPS Kankan','Kankan','2026-01-01','HR')`); err != nil {
		t.Fatal(err)
	}
	structures, err := st.ListDrhStructures()
	if err != nil || len(structures) != 1 || structures[0].TypeCode != "HR" {
		t.Fatalf("structures = %+v (err %v)", structures, err)
	}

	rows := []drh.AgentRow{
		{Region: "Kankan", Prefecture: "Kankan", StructureAffectation: "HR Kankan", Profession: "Médécin Généraliste", Sexe: "H", AnneeNaissance: 1968},
		{Region: "Kankan", Prefecture: "Kankan", StructureAffectation: "HR Kankan", Profession: "Sage-Femme", Sexe: "F", AnneeNaissance: 1992},
		{Region: "Kankan", Prefecture: "Kankan", StructureAffectation: "DPS Kankan", Profession: "ATS", Sexe: "F", AnneeNaissance: 1980},
	}
	corr, _ := st.ListDrhCorrespondances()
	affs, rep := drh.ResolveAll(rows, drh.NewResolver(structures, corr))
	opt := drh.Options{RefYear: 2026, RetirementAge: 60}
	eff, pyr := drh.Aggregate(rows, affs, opt)

	im, err := st.SaveDrhImport(DrhImport{Label: "DRH/CNPS 2026", Annee: 2026, AgeRetraite: 60,
		NAgents: rep.NAgents, NStructure: rep.NStructure, NBureau: rep.NBureau,
		NCentrale: rep.NCentrale, NNonRattache: rep.NNonRattache, NStructures: rep.NStructuresVues,
		ImportedBy: "test"}, eff, pyr, rep.Inconnus)
	if err != nil {
		t.Fatalf("enregistrement : %v", err)
	}
	if im.Status != "active" || im.NAgents != 3 || im.NStructure != 2 || im.NBureau != 1 {
		t.Fatalf("millésime enregistré : %+v", im)
	}

	active, err := st.GetActiveDrhImport()
	if err != nil || active == nil || active.ID != im.ID {
		t.Fatalf("millésime actif = %+v (err %v)", active, err)
	}

	var nAgents, nDepart int
	if err := st.db.QueryRow(`SELECT n_agents, n_depart_5ans FROM drh_effectif
		WHERE import_id = ? AND dimension = 'structure' AND key = 'u1' AND categorie = ''`, im.ID).Scan(&nAgents, &nDepart); err != nil {
		t.Fatalf("lecture de l'effectif : %v", err)
	}
	if nAgents != 2 || nDepart != 1 {
		t.Errorf("HR Kankan : %d agents, %d départs à 5 ans", nAgents, nDepart)
	}

	// Un second import archive le premier sans le supprimer : les millésimes
	// restent comparables dans le temps.
	im2, err := st.SaveDrhImport(DrhImport{Label: "DRH/CNPS 2027", Annee: 2027, AgeRetraite: 60, NAgents: 3}, eff, pyr, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := st.GetDrhImport(im.ID)
	if first.Status != "archived" {
		t.Errorf("le millésime précédent doit être archivé, statut %q", first.Status)
	}
	if list, _ := st.ListDrhImports(); len(list) != 2 {
		t.Errorf("%d millésimes listés, attendu 2", len(list))
	}

	if err := st.ActivateDrhImport(im.ID); err != nil {
		t.Fatal(err)
	}
	if active, _ := st.GetActiveDrhImport(); active.ID != im.ID {
		t.Errorf("réactivation du millésime %d sans effet", im.ID)
	}

	if err := st.DeleteDrhImport(im2.ID); err != nil {
		t.Fatal(err)
	}
	var restant int
	st.db.QueryRow(`SELECT COUNT(*) FROM drh_effectif WHERE import_id = ?`, im2.ID).Scan(&restant)
	if restant != 0 {
		t.Errorf("%d cellules orphelines après suppression", restant)
	}
}
