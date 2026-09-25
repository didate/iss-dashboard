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

func TestDrhRollupsRoundTrip(t *testing.T) {
	st := testStore(t)
	if _, err := st.db.Exec(`INSERT INTO event (event_uid, org_unit_uid, org_unit_name, district, region, event_date, type_code, sous_prefecture, sous_prefecture_uid)
		VALUES ('e1','u1','HR Kankan','DPS Kankan','IRS Kankan','2026-01-01','HR','Kankan Centre','sp1'),
		       ('e2','u2','CSR Balandou','DPS Kankan','IRS Kankan','2026-01-01','CS','Balandou','sp2')`); err != nil {
		t.Fatal(err)
	}
	// Population et effectifs ISS, les deux entrées du recalcul qui viennent du snapshot DHIS2.
	if _, err := st.db.Exec(`INSERT INTO org_unit (uid, name, level) VALUES ('gn','Guinée',1),('d1','DPS Kankan',3);
		INSERT INTO population (ou_uid, indicator, period, value) VALUES ('gn','total','2026',1000000),('d1','total','2026',200000);
		INSERT INTO usage_rh (profil_code, label, district, effectif_total) VALUES ('ISS_RH_MED_GEN','Médecin','all',10),('ISS_RH_MED_GEN','Médecin','DPS Kankan',4)`); err != nil {
		t.Fatal(err)
	}

	structures, _ := st.ListDrhStructures()
	if len(structures) != 2 || structures[0].SousPrefectureUID == "" {
		t.Fatalf("structures incomplètes : %+v", structures)
	}
	rows := []drh.AgentRow{
		{Region: "KANKAN", Prefecture: "Kankan", StructureAffectation: "HR Kankan", Profession: "Médécin Généraliste", Sexe: "H", AnneeNaissance: 1968},
		{Region: "KANKAN", Prefecture: "Kankan", StructureAffectation: "HR Kankan", Profession: "Sage-Femme", Sexe: "F"},
		{Region: "KANKAN", Prefecture: "Kankan", StructureAffectation: "DPS Kankan", Profession: "ATS", Sexe: "F", AnneeNaissance: 1980},
	}
	affs, rep := drh.ResolveAll(rows, drh.NewResolver(structures, nil))
	eff, pyr := drh.Aggregate(rows, affs, drh.Options{RefYear: 2026, RetirementAge: 60})
	im, err := st.SaveDrhImport(DrhImport{Label: "test", Annee: 2026, AgeRetraite: 60, NAgents: rep.NAgents}, eff, pyr, nil)
	if err != nil {
		t.Fatal(err)
	}

	fineEff, finePyr, err := st.GetDrhFineCells(im.ID)
	if err != nil || len(fineEff) == 0 {
		t.Fatalf("relecture des cellules : %d lignes, err %v", len(fineEff), err)
	}
	pop, err := st.GetDrhPopulationIndex()
	if err != nil {
		t.Fatal(err)
	}
	if pop[drh.PopKey(drh.DimDistrict, "DPS Kankan")] != 200000 || pop[drh.PopKey(drh.DimGlobal, drh.KeyNational)] != 1000000 {
		t.Fatalf("index de population : %+v", pop)
	}
	iss, err := st.GetIssRH()
	if err != nil || len(iss) != 2 {
		t.Fatalf("effectifs ISS : %+v (err %v)", iss, err)
	}

	byUID := map[string]drh.Structure{}
	for _, s := range structures {
		byUID[s.UID] = s
	}
	rEff, rPyr := drh.Rollup(fineEff, finePyr, drh.RollupContext{Structures: byUID, Population: pop})
	comp := drh.Compare(rEff, iss)
	if err := st.ReplaceDrhRollups(im.ID, rEff, rPyr, comp); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetDrhEffectifs(im.ID, DrhEffectifParams{Dimension: drh.DimDistrict, Categorie: ""})
	if err != nil || len(got) != 1 {
		t.Fatalf("effectifs district : %+v (err %v)", got, err)
	}
	d := got[0]
	if d.NAgents != 3 || d.NAgeConnu != 2 || d.Ratio10k == nil || *d.Ratio10k != 0.15 {
		t.Errorf("district : %+v", d)
	}

	// Un recalcul ne doit jamais dupliquer ni toucher au grain fin.
	if err := st.ReplaceDrhRollups(im.ID, rEff, rPyr, comp); err != nil {
		t.Fatal(err)
	}
	again, _ := st.GetDrhEffectifs(im.ID, DrhEffectifParams{Dimension: drh.DimDistrict, Categorie: ""})
	if len(again) != 1 || again[0].NAgents != 3 {
		t.Errorf("recalcul non idempotent : %+v", again)
	}
	fineAfter, _, _ := st.GetDrhFineCells(im.ID)
	if len(fineAfter) != len(fineEff) {
		t.Errorf("le recalcul a touché au grain fin : %d cellules puis %d", len(fineEff), len(fineAfter))
	}

	cmp, err := st.GetDrhComparaison(im.ID, drh.DimDistrict, "DPS Kankan", "MED_GEN")
	if err != nil || len(cmp) != 1 || cmp[0].NIss == nil || *cmp[0].NIss != 4 || *cmp[0].Ratio != 4 {
		t.Errorf("comparaison : %+v (err %v)", cmp, err)
	}
	// Sans clé, la comparaison couvre toutes les zones de la dimension.
	if all, _ := st.GetDrhComparaison(im.ID, drh.DimDistrict, "", "MED_GEN"); len(all) != 1 {
		t.Errorf("comparaison sans clé : %d lignes", len(all))
	}
	if none, _ := st.GetDrhComparaison(im.ID, drh.DimDistrict, "DPS Boké", "MED_GEN"); len(none) != 0 {
		t.Errorf("une clé inconnue ne doit rien renvoyer : %+v", none)
	}

	// La pyramide renvoie toutes les tranches, y compris les vides.
	pyrRows, err := st.GetDrhPyramide(im.ID, drh.DimDistrict, "DPS Kankan", "")
	if err != nil || len(pyrRows) != len(drh.Tranches) {
		t.Fatalf("pyramide : %d tranches (err %v)", len(pyrRows), err)
	}
	var totalPyr int
	for _, r := range pyrRows {
		totalPyr += r.NAgents
	}
	if totalPyr != 3 {
		t.Errorf("pyramide district = %d agents, attendu 3", totalPyr)
	}

	// Les structures sans aucun agent de l'État doivent rester visibles.
	list, err := st.GetDrhStructuresList(im.ID, "DPS Kankan", "")
	if err != nil || len(list) != 2 {
		t.Fatalf("liste des structures : %+v (err %v)", list, err)
	}
	if list[0].Name != "HR Kankan" || list[0].NAgents != 2 || list[1].NAgents != 0 {
		t.Errorf("tri ou jointure incorrects : %+v", list)
	}
}
