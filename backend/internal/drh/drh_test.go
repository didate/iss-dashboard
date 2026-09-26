package drh

import (
	"strings"
	"testing"
)

const csvHeader = "region;prefecture;sous_prefecture;structure_affectation;structure_rattachement;profession;profession_oms;hierarchie;statut;sexe;annee_naissance;zone;niveau_structure\n"

func TestParseCSV(t *testing.T) {
	in := csvHeader +
		"BOKE;Boké;Commune Urbaine;IRS Boké;IRS Boké;Médécin Généraliste;Médecin;A2;Fonctionnaire;H;1981;urbaine;secondaire\n" +
		"\n" +
		"KANKAN;Kankan;;HRKkan;HR Kankan;Infirmiers (ères) d'Etat;;B2;Fonctionnaire;féminin;12/03/1990;rurale;\n"
	rows, errs := ParseCSV(strings.NewReader(in))
	if len(errs) != 0 {
		t.Fatalf("erreurs inattendues : %+v", errs)
	}
	if len(rows) != 2 {
		t.Fatalf("2 lignes attendues, %d", len(rows))
	}
	if rows[0].AnneeNaissance != 1981 || rows[0].Sexe != "H" {
		t.Errorf("ligne 1 mal lue : %+v", rows[0])
	}
	if rows[1].Sexe != "F" || rows[1].AnneeNaissance != 1990 {
		t.Errorf("sexe/année non normalisés : %+v", rows[1])
	}
	if rows[1].Libelle() != "HRKkan" {
		t.Errorf("libellé = %q", rows[1].Libelle())
	}
}

// Le rejet d'une colonne inconnue est la garantie qu'un fichier nominatif ne
// peut pas être importé par inadvertance.
func TestParseCSVRejetteColonneInconnue(t *testing.T) {
	in := "matricule;" + csvHeader[:len(csvHeader)-1] + "\n001;BOKE;Boké;;;;Médécin Généraliste;;;;H;1981;;\n"
	rows, errs := ParseCSV(strings.NewReader(in))
	if len(rows) != 0 {
		t.Fatalf("aucune ligne ne doit être importée, %d lues", len(rows))
	}
	if len(errs) == 0 || !strings.Contains(errs[0].Message, "matricule") {
		t.Fatalf("l'erreur doit nommer la colonne fautive : %+v", errs)
	}
}

func TestParseCSVColonneObligatoireVide(t *testing.T) {
	in := csvHeader + "BOKE;;;CS Kolaboui;;Sage-Femme;;;;F;1990;;\n"
	rows, errs := ParseCSV(strings.NewReader(in))
	if len(rows) != 0 || len(errs) != 1 {
		t.Fatalf("ligne sans préfecture : rows=%d errs=%+v", len(rows), errs)
	}
}

// Une profession vide ne fait pas perdre l'agent : il compte, en « Autre ».
func TestParseCSVProfessionVide(t *testing.T) {
	in := csvHeader + "BOKE;Boké;;CS Kolaboui;;;;;Fonctionnaire;F;1990;;\n"
	rows, errs := ParseCSV(strings.NewReader(in))
	if len(rows) != 1 || len(errs) != 0 {
		t.Fatalf("rows=%d errs=%+v", len(rows), errs)
	}
	if CategorieDe(rows[0].Profession) != "AUTRE" {
		t.Errorf("profession vide mal classée")
	}
}

func TestTranche(t *testing.T) {
	cases := map[int]string{-1: TrancheInconnue, 20: "<25", 25: "25-29", 43: "40-44", 59: "55-59", 60: "60+", 71: "60+"}
	for age, want := range cases {
		if got := Tranche(age); got != want {
			t.Errorf("Tranche(%d) = %q, attendu %q", age, got, want)
		}
	}
}

func TestDepart(t *testing.T) {
	opt := Options{RefYear: 2026, RetirementAge: 60}
	a := AgentRow{AnneeNaissance: 1970} // 56 ans
	if !a.PartDansMoinsDe(5, opt) {
		t.Error("56 ans doit partir d'ici 5 ans")
	}
	b := AgentRow{AnneeNaissance: 1980} // 46 ans
	if b.PartDansMoinsDe(5, opt) || !b.PartDansMoinsDe(15, opt) {
		t.Error("46 ans : pas d'ici 5 ans, oui d'ici 15")
	}
	c := AgentRow{} // année inconnue
	if c.PartDansMoinsDe(10, opt) {
		t.Error("une année inconnue ne doit pas compter dans les départs")
	}
}

func TestCategorieDe(t *testing.T) {
	cases := map[string]string{
		"Médécin Généraliste":                   "MED_GEN",
		"Médecin Spécialiste en Chirurgie":      "MED_CHIR",
		"Médecin Spécialiste en Santé Publique": "MED_SP_PUB",
		"Médecin Autre Specialité":              "MED_AUTRE",
		"Médecin Légiste":                       "MED_AUTRE",
		"Chirurgien Dentiste Généraliste":       "DENT",
		"Infirmiers (ères) d'Etat":              "INF",
		"ATS":                                   "ATS",
		"Sage-Femme":                            "SAGEF",
		"Biologiste médicale":                   "BIO",
		"Technicien de Laboratoire":             "TECH_LAB",
		"Laborantin":                            "TECH_LAB",
		"Techniciens Biomédicaux":               "BIOMED",
		"Statisticien":                          "STAT",
		"CHAUFFEUR":                             "CHAUFFEUR",
		"Administrateur Civil":                  "ADMIN",
		"Technicien Santé Publique":             "AUTRE_SANTE",
		"Aide Santé":                            "AIDE_SOIN",
		"":                                      "AUTRE",
		"Lettre Moderme":                        "AUTRE",
	}
	for prof, want := range cases {
		if got := CategorieDe(prof); got != want {
			t.Errorf("CategorieDe(%q) = %q, attendu %q", prof, got, want)
		}
	}
	for _, c := range Categories {
		if c.Famille == "" {
			t.Errorf("catégorie %s sans famille", c.Code)
		}
	}
}

// --- Rattachement -----------------------------------------------------------

// --- Rattachement : le fichier, et rien d'autre ------------------------------

func unites() []UniteOrg {
	return []UniteOrg{
		{UID: "gn", Name: "Guinée", Level: 1},
		{UID: "reg", Name: "IRS Kankan", Level: 2, Region: "IRS Kankan"},
		{UID: "dis", Name: "DPS Kankan", Level: 3, District: "DPS Kankan", Region: "IRS Kankan"},
		{UID: "hr", Name: "HR Kankan", Level: 5, District: "DPS Kankan", Region: "IRS Kankan", TypeCode: "HR"},
		{UID: "ps", Name: "PS Balandou", Level: 6, District: "DPS Kankan", Region: "IRS Kankan"},
	}
}

// Le niveau de l'unité dans la hiérarchie DHIS2 décide de la nature du
// rattachement. Aucun libellé n'est lu.
func TestResolveParNiveau(t *testing.T) {
	r := NewResolver(unites())
	for _, c := range []struct{ nom, uid, kind, key string }{
		{"unité racine", "gn", AffCentrale, Norm("Guinée")},
		{"région", "reg", AffBureauRegional, "IRS Kankan"},
		{"district", "dis", AffBureau, "DPS Kankan"},
		{"hôpital", "hr", AffStructure, "hr"},
		{"poste de santé", "ps", AffStructure, "ps"},
	} {
		t.Run(c.nom, func(t *testing.T) {
			got := r.Resolve(AgentRow{UIDDhis2: c.uid, Prefecture: "Kankan"})
			if got.Kind != c.kind || got.Key != c.key || got.Source != SrcFichier {
				t.Errorf("= %s/%s/%s, attendu %s/%s/%s", got.Kind, got.Key, got.Source, c.kind, c.key, SrcFichier)
			}
		})
	}
}

// L'administration centrale porte un seul identifiant pour toutes ses entités :
// c'est le libellé de la ligne qui sépare les directions, instituts et
// programmes. Sans cela, ils formeraient un bloc indistinct.
func TestResolveCentraleParEntite(t *testing.T) {
	r := NewResolver(unites())
	pnlp := r.Resolve(AgentRow{UIDDhis2: "gn", StructureAffectation: "PNLP"})
	drh := r.Resolve(AgentRow{UIDDhis2: "gn", StructureAffectation: "DRH"})
	if pnlp.Kind != AffCentrale || drh.Kind != AffCentrale {
		t.Fatalf("les deux lignes relèvent de l'administration centrale, a %s et %s", pnlp.Kind, drh.Kind)
	}
	if pnlp.Key == drh.Key {
		t.Errorf("deux entités distinctes partagent la clé « %s »", pnlp.Key)
	}
	if pnlp.Label != "PNLP" {
		t.Errorf("libellé = %q, attendu « PNLP »", pnlp.Label)
	}
	// Ligne sans libellé : l'unité racine sert de dernier recours.
	if vide := r.Resolve(AgentRow{UIDDhis2: "gn"}); vide.Label != "Guinée" {
		t.Errorf("sans libellé, = %q, attendu « Guinée »", vide.Label)
	}
}

// Les cadres d'une région restent au niveau région : ils ne doivent pas tomber
// dans un district, sinon ils en gonfleraient un au hasard.
func TestResolveRegionSansDistrict(t *testing.T) {
	got := NewResolver(unites()).Resolve(AgentRow{UIDDhis2: "reg", Prefecture: "Kankan"})
	if got.District != "" {
		t.Fatalf("un bureau régional ne doit porter aucun district, a « %s »", got.District)
	}
	if got.Region != "IRS Kankan" {
		t.Errorf("région = %q", got.Region)
	}
}

// Sans identifiant, ou avec un identifiant inconnu, rien n'est deviné : l'agent
// remonte dans le rapport pour être tranché à la source.
func TestResolveSansIdentifiant(t *testing.T) {
	r := NewResolver(unites())
	for _, c := range []struct {
		nom string
		a   AgentRow
	}{
		{"identifiant vide", AgentRow{Prefecture: "Kankan", StructureAffectation: "HR Kankan"}},
		{"identifiant inconnu", AgentRow{UIDDhis2: "zzz", Prefecture: "Kankan"}},
		{"nom exact mais pas d'identifiant", AgentRow{Prefecture: "Kankan", StructureAffectation: "HR Kankan", StructureRattachement: "HR Kankan"}},
	} {
		t.Run(c.nom, func(t *testing.T) {
			if got := r.Resolve(c.a); got.Kind != AffNonRattache || got.Source != SrcInconnu {
				t.Errorf("= %s/%s, attendu non_rattache/inconnu", got.Kind, got.Source)
			}
		})
	}
}

func TestResolveAllReport(t *testing.T) {
	rows := []AgentRow{
		{UIDDhis2: "hr", Prefecture: "Kankan", Profession: "Médécin Généraliste", Sexe: "H", AnneeNaissance: 1970},
		{UIDDhis2: "hr", Prefecture: "Kankan", Profession: "Sage-Femme", Sexe: "F", AnneeNaissance: 1990},
		{UIDDhis2: "dis", Prefecture: "Kankan", Profession: "ATS", Sexe: "F"},
		{UIDDhis2: "reg", Prefecture: "Kankan", Profession: "ATS", Sexe: "H", AnneeNaissance: 1980},
		{UIDDhis2: "gn", Prefecture: "Kaloum", Profession: "Administrateur Civil", Sexe: "H", AnneeNaissance: 1975},
		{Prefecture: "Gaoual", StructureAffectation: "CS Youkounkoun", Profession: "ATS", Sexe: "H", AnneeNaissance: 1985},
	}
	affs, rep := ResolveAll(rows, NewResolver(unites()))
	if rep.NAgents != 6 || rep.NStructure != 2 || rep.NBureau != 1 || rep.NBureauRegional != 1 || rep.NCentrale != 1 || rep.NNonRattache != 1 {
		t.Fatalf("rapport inattendu : %+v", rep)
	}
	if rep.NStructuresVues != 1 {
		t.Errorf("structures couvertes = %d, attendu 1", rep.NStructuresVues)
	}
	if len(rep.Inconnus) != 1 || rep.Inconnus[0].Libelle != "CS Youkounkoun" {
		t.Errorf("liste des inconnus : %+v", rep.Inconnus)
	}
	if got := rep.PctCategorise(); got < 83.3 || got > 83.4 {
		t.Errorf("part catégorisée = %.1f, attendu 83,3", got)
	}

	eff, _ := Aggregate(rows, affs, Options{RefYear: 2026, RetirementAge: 60})
	total := find(t, eff, AffStructure, "hr", CategorieToutes)
	if total.NAgents != 2 || total.NFemmes != 1 || total.NDepart5Ans != 1 {
		t.Errorf("total HR Kankan : %+v", total)
	}
	if total.District != "DPS Kankan" {
		t.Errorf("district non propagé : %q", total.District)
	}
}

func find(t *testing.T, rows []EffectifRow, dim, key, cat string) EffectifRow {
	t.Helper()
	for _, r := range rows {
		if r.Dimension == dim && r.Key == key && r.Categorie == cat {
			return r
		}
	}
	t.Fatalf("cellule %s/%s/%q absente", dim, key, cat)
	return EffectifRow{}
}

func TestParseCorrespondancesCSVUneRegleParDistrict(t *testing.T) {
	in := "libelle_drh;structure_iss;uid_dhis2;district;type;statut\n" +
		"HRK;HR Kankan;uid-kankan;DPS Kankan;HR;OK\n" +
		"HRK;HR Kindia;uid-kindia;DPS Kindia;HR;OK\n"
	corr, errs := ParseCorrespondancesCSV(strings.NewReader(in))
	if len(errs) != 0 || len(corr) != 2 {
		t.Fatalf("deux règles attendues, une par district : %+v (err %+v)", corr, errs)
	}
}

// Une règle écrite après coup corrige la précédente : la table s'édite en
// ajoutant à la fin, et une correction ne doit pas être ignorée en silence.
func TestParseCorrespondancesCSVDerniereGagne(t *testing.T) {
	in := "libelle_drh;structure_iss;uid_dhis2;district;type;statut\n" +
		"CSA Kountia;PS Kountia;uid-ps;;;OK\n" +
		"CSA Kountia;CSA Kountya;uid-csa;;;OK\n"
	corr, errs := ParseCorrespondancesCSV(strings.NewReader(in))
	if len(errs) != 0 || len(corr) != 1 {
		t.Fatalf("une seule règle attendue : %+v (err %+v)", corr, errs)
	}
	if corr[0].OrgUnitUID != "uid-csa" {
		t.Fatalf("la dernière ligne doit gagner, cible retenue : %s", corr[0].OrgUnitUID)
	}
}

// Une ligne commentée permet de proposer deux cibles pour un même libellé et
// de n'en activer qu'une, sans perdre l'autre.
func TestParseCorrespondancesCSVCommentaires(t *testing.T) {
	in := "libelle_drh;structure_iss;uid_dhis2;district;type;statut\n" +
		"# ---- à trancher ----\n" +
		"Boffa Centre;HP Boffa;uid-hp;DPS Boffa;HP;OK\n" +
		"# Boffa Centre;CSU Boffa;uid-csu;DPS Boffa;CS;OK\n"
	corr, errs := ParseCorrespondancesCSV(strings.NewReader(in))
	if len(errs) != 0 {
		t.Fatalf("les commentaires ne doivent pas produire d'erreur : %+v", errs)
	}
	if len(corr) != 1 || corr[0].OrgUnitUID != "uid-hp" {
		t.Fatalf("une seule règle active attendue : %+v", corr)
	}
}

func TestParseCorrespondancesCSV(t *testing.T) {
	in := "libelle_drh;structure_iss;uid_dhis2;district;type;statut\n" +
		"HASIGUI;Hopital Amitie Sino-Guinéen;qoNflHBihaD;DCS Ratoma;HN;OK\n" +
		"CS Gbessia port1;CSU Gbessia Port 1;R21rlk5ynyV;DCS Matoto;CS;Ok\n" +
		"CS Gbessia Port1;CSU Gbessia Port 1;R21rlk5ynyV;DCS Matoto;CS;OK\n" +
		"Kérouane;;;DPS Kérouané;;BUREAU DE DISTRICT\n" +
		"Boffa Centre;;;DPS Boffa;;NON RATTACHE\n" +
		"Bidon;;;;;PEUT ETRE\n" +
		"SansUID;Quelque chose;;DPS Boffa;CS;OK\n"
	corr, errs := ParseCorrespondancesCSV(strings.NewReader(in))
	if len(corr) != 4 {
		t.Fatalf("4 correspondances attendues (doublon de casse fusionné), %d : %+v", len(corr), corr)
	}
	if len(errs) != 2 {
		t.Fatalf("2 erreurs attendues (statut inconnu, ok sans uid), %+v", errs)
	}
	if corr[0].Statut != CorrOK || corr[3].Statut != CorrNonRattache {
		t.Errorf("statuts mal normalisés : %+v", corr)
	}
}
