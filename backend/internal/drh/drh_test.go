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

func testResolver(t *testing.T) *Resolver {
	t.Helper()
	structures := []Structure{
		{UID: "u-koule-csa", Name: "CSA de Koule", District: "DPS N'zérékoré", Region: "N'zérékoré", TypeCode: "CSA"},
		{UID: "u-koule-csr", Name: "CSR Koule", District: "DPS N'zérékoré", Region: "N'zérékoré", TypeCode: "CS"},
		{UID: "u-hp-gaoual", Name: "HP Gaoual", District: "DPS Gaoual", Region: "Boké", TypeCode: "HP"},
		{UID: "u-hr-kankan", Name: "HR Kankan", District: "DPS Kankan", Region: "Kankan", TypeCode: "HR"},
		{UID: "u-csr-bokaria", Name: "CSR Bokaria", District: "DPS Forécariah", Region: "Kindia", TypeCode: "CS"},
	}
	corr := []Correspondance{
		{LibelleNorm: Norm("HRKkan"), LibelleDRH: "HRKkan", OrgUnitUID: "u-hr-kankan", Statut: CorrOK},
		{LibelleNorm: Norm("Kérouane"), LibelleDRH: "Kérouane", Statut: CorrBureau},
		{LibelleNorm: Norm("Boffa Centre"), LibelleDRH: "Boffa Centre", Statut: CorrNonRattache},
	}
	return NewResolver(structures, corr)
}

func TestResolve(t *testing.T) {
	r := testResolver(t)
	cases := []struct {
		nom        string
		agent      AgentRow
		wantKind   string
		wantKey    string
		wantSource string
	}{
		{"table de correspondance", AgentRow{Prefecture: "Kankan", StructureAffectation: "HRKkan"}, AffStructure, "u-hr-kankan", SrcTable},
		{"nom exact", AgentRow{Prefecture: "Gaoual", StructureAffectation: "HP Gaoual"}, AffStructure, "u-hp-gaoual", SrcExact},
		{"type + nom propre", AgentRow{Prefecture: "N'Zérékoré", StructureAffectation: "CS KOULE"}, AffStructure, "u-koule-csr", SrcApprox},
		{"type voisin quand il n'y a qu'un centre", AgentRow{Prefecture: "Forécariah", StructureAffectation: "CMC Bokaria"}, AffStructure, "u-csr-bokaria", SrcApprox},
		{"homonymes de types voisins : non rattaché", AgentRow{Prefecture: "N'Zérékoré", StructureAffectation: "CMC  KOULE"}, AffNonRattache, "n zerekore", SrcInconnu},
		{"déduction du seul hôpital", AgentRow{Prefecture: "Gaoual", StructureAffectation: "Hôpital Préfectoral"}, AffStructure, "u-hp-gaoual", SrcDeduit},
		{"sigle de bureau de district", AgentRow{Prefecture: "Forécariah", StructureAffectation: "DPS Forécariah"}, AffBureau, "forecariah", SrcPrefixe},
		{"bureau écrit en toutes lettres", AgentRow{Prefecture: "Forécariah", StructureAffectation: "Direction Prefectoral de la santé"}, AffBureau, "forecariah", SrcPrefixe},
		{"inspection régionale", AgentRow{Prefecture: "Kankan", StructureAffectation: "Inspection Régionale de la santé"}, AffBureau, "kankan", SrcPrefixe},
		{"bureau via la table", AgentRow{Prefecture: "Kérouané", StructureAffectation: "Kérouane"}, AffBureau, "kerouane", SrcTable},
		{"administration centrale", AgentRow{Prefecture: "Conakry", StructureAffectation: "DRH Ministère"}, AffCentrale, "drh ministere", SrcPrefixe},
		{"programme national", AgentRow{Prefecture: "Conakry", StructureAffectation: "PNLP"}, AffCentrale, "pnlp", SrcPrefixe},
		{"sigle en casse mixte", AgentRow{Prefecture: "Conakry", StructureAffectation: "DSVCo"}, AffCentrale, "dsvco", SrcPrefixe},
		{"non rattachable déclaré", AgentRow{Prefecture: "Boffa", StructureAffectation: "Boffa Centre"}, AffNonRattache, "boffa", SrcTable},
		{"libellé inconnu", AgentRow{Prefecture: "Gaoual", StructureAffectation: "CS Youkounkoun"}, AffNonRattache, "gaoual", SrcInconnu},
		{"repli sur la structure de rattachement", AgentRow{Prefecture: "Kankan", StructureRattachement: "HR Kankan"}, AffStructure, "u-hr-kankan", SrcExact},
		{"affectation vide : le rattachement administratif fait foi", AgentRow{Prefecture: "Forécariah", StructureRattachement: "DPS Forécariah"}, AffBureau, "forecariah", SrcPrefixe},
		{"affectation non reconnue : pas de repli sur le bureau", AgentRow{Prefecture: "Forécariah", StructureAffectation: "CSU Inconnu", StructureRattachement: "DPS Forécariah"}, AffNonRattache, "forecariah", SrcInconnu},
		{"affectation non reconnue : repli accepté vers une structure", AgentRow{Prefecture: "Kankan", StructureAffectation: "Pneumologie", StructureRattachement: "HR Kankan"}, AffStructure, "u-hr-kankan", SrcExact},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			got := r.Resolve(c.agent)
			if got.Kind != c.wantKind || got.Key != c.wantKey || got.Source != c.wantSource {
				t.Errorf("= %s/%s/%s, attendu %s/%s/%s", got.Kind, got.Key, got.Source, c.wantKind, c.wantKey, c.wantSource)
			}
		})
	}
}

// La DRH écrit ses propres noms de région ("BOKE") : les agrégats régionaux
// doivent porter le nom ISS, sinon chaque région compte double.
func TestResolveRegionISS(t *testing.T) {
	r := testResolver(t)
	for _, a := range []AgentRow{
		{Prefecture: "Gaoual", Region: "BOKE", StructureAffectation: "DPS Gaoual"},
		{Prefecture: "Gaoual", Region: "BOKE", StructureAffectation: "Libellé inconnu"},
		{Prefecture: "Gaoual", Region: "BOKE", StructureAffectation: "HP Gaoual"},
	} {
		if got := r.Resolve(a); got.Region != "Boké" {
			t.Errorf("%s → région %q, attendu la région ISS \"Boké\"", a.StructureAffectation, got.Region)
		}
	}
}

// Le CT-EPi est hébergé par l'hôpital du district : le même libellé doit se
// résoudre différemment selon la préfecture de l'agent, ce qu'une table de
// correspondance globale ne sait pas faire.
func TestResolveServiceDuDistrict(t *testing.T) {
	r := NewResolver([]Structure{
		{UID: "hp-siguiri", Name: "HP siguiri", District: "DPS Siguiri", Region: "IRS Kankan", TypeCode: "HP"},
		{UID: "hp-pita", Name: "HP Pita", District: "DPS Pita", Region: "IRS Mamou", TypeCode: "HP"},
		{UID: "cs-pita", Name: "CSU Pita", District: "DPS Pita", TypeCode: "CS"},
		{UID: "hp-a", Name: "HP Macenta", District: "DPS Macenta", TypeCode: "HP"},
		{UID: "hp-b", Name: "Centre Hospitalier Regional Specialise Macenta", District: "DPS Macenta", TypeCode: "HP"},
	}, nil)

	for _, c := range []struct{ libelle, prefecture, want string }{
		{"CT-EPi", "Siguiri", "hp-siguiri"},
		{"CTEPI", "SIGUIRI", "hp-siguiri"},
		{"CT-Epi", "Pita", "hp-pita"},
	} {
		got := r.Resolve(AgentRow{Prefecture: c.prefecture, StructureAffectation: c.libelle})
		if got.Kind != AffStructure || got.Key != c.want || got.Source != SrcService {
			t.Errorf("%s / %s = %s/%s/%s, attendu structure/%s/%s", c.libelle, c.prefecture, got.Kind, got.Key, got.Source, c.want, SrcService)
		}
	}

	// Deux hôpitaux du même type dans le district : pas de rattachement au hasard.
	got := r.Resolve(AgentRow{Prefecture: "Macenta", StructureAffectation: "CT-EPi"})
	if got.Kind == AffStructure {
		t.Errorf("district à deux hôpitaux : rattaché quand même à %s", got.Key)
	}
}

// Un même libellé peut désigner une structure différente selon le district :
// « HOPITAL » à Fria n'est pas celui de Boffa, et aucune déduction ne peut les
// départager quand le district compte deux hôpitaux.
func TestResolveCorrespondanceParDistrict(t *testing.T) {
	r := NewResolver([]Structure{
		{UID: "hp-fria", Name: "HP Fria", District: "DPS Fria", TypeCode: "HP"},
		{UID: "rusal", Name: "Hôpital RUSAL", District: "DPS Fria", TypeCode: "HP"},
		{UID: "hp-boffa", Name: "HP Boffa", District: "DPS Boffa", TypeCode: "HP"},
		{UID: "hr-kankan", Name: "HR Kankan", District: "DPS Kankan", TypeCode: "HR"},
	}, []Correspondance{
		{LibelleNorm: Norm("HOPITAL"), LibelleDRH: "HOPITAL", OrgUnitUID: "hp-fria", Statut: CorrOK, District: "DPS Fria"},
		{LibelleNorm: Norm("HOPITAL"), LibelleDRH: "HOPITAL", OrgUnitUID: "hp-boffa", Statut: CorrOK, District: "DPS Boffa"},
		{LibelleNorm: Norm("HRKkan"), LibelleDRH: "HRKkan", OrgUnitUID: "hr-kankan", Statut: CorrOK},
	})

	for _, c := range []struct{ prefecture, want string }{{"Fria", "hp-fria"}, {"Boffa", "hp-boffa"}} {
		got := r.Resolve(AgentRow{Prefecture: c.prefecture, StructureAffectation: "HOPITAL"})
		if got.Key != c.want {
			t.Errorf("HOPITAL à %s → %s, attendu %s", c.prefecture, got.Key, c.want)
		}
	}
	// Hors des districts couverts par une règle, le libellé ne pioche pas dans
	// les règles des autres : il repasse par la déduction ordinaire, qui ne
	// tranche que si le district n'a qu'un seul hôpital.
	if got := r.Resolve(AgentRow{Prefecture: "Kankan", StructureAffectation: "HOPITAL"}); got.Key != "hr-kankan" || got.Source != SrcDeduit {
		t.Errorf("HOPITAL à Kankan = %s/%s, attendu hr-kankan par déduction", got.Key, got.Source)
	}
	// Une règle sans district vaut partout.
	if got := r.Resolve(AgentRow{Prefecture: "Boffa", StructureAffectation: "HRKkan"}); got.Key != "hr-kankan" {
		t.Errorf("règle sans district non appliquée : %+v", got)
	}
}

// « Pneumologie » est un service hospitalier, pas un programme national : le
// motif des sigles ne doit pas happer les mots ordinaires.
func TestResolveMotOrdinaireNestPasUnSigle(t *testing.T) {
	r := testResolver(t)
	got := r.Resolve(AgentRow{Prefecture: "Gaoual", StructureAffectation: "Pneumologie"})
	if got.Kind == AffCentrale {
		t.Fatalf("« Pneumologie » classé en administration centrale : %+v", got)
	}
	for _, sigle := range []string{"PNLP", "DNELM", "CT-EPi", "DSVCo", "IGS"} {
		if !estSigle(sigle) {
			t.Errorf("%q devrait être reconnu comme un sigle", sigle)
		}
	}
	for _, mot := range []string{"Pneumologie", "Maternite", "Chirurgie", "a"} {
		if estSigle(mot) {
			t.Errorf("%q ne devrait pas être reconnu comme un sigle", mot)
		}
	}
}

// Une préfecture que la hiérarchie ISS ne connaît pas ne doit pas fabriquer
// une région fantôme à côté des régions réelles.
func TestResolvePrefectureInconnue(t *testing.T) {
	r := testResolver(t)
	got := r.Resolve(AgentRow{Prefecture: "Kassa", Region: "CONAKRY", StructureAffectation: "Bureau"})
	if got.Kind != AffNonRattache || got.District != "Kassa" || got.Region != "" {
		t.Fatalf("= %+v, attendu un district visible « Kassa » sans région", got)
	}
}

// Une correspondance qui vise une structure absente du recensement ne doit pas
// être ignorée en silence : l'agent reste non rattaché, mais le rapport dit
// pourquoi — la structure existe, ISS ne l'a jamais recensée.
func TestResolveCorrespondanceVersStructureNonRecensee(t *testing.T) {
	r := NewResolver(
		[]Structure{
			{UID: "hp", Name: "HP Boké", District: "DPS Boké", TypeCode: "HP"},
			{UID: "kassapo", Name: "CSR Kassapo", District: "DPS Boké", HorsRecensement: true},
		},
		[]Correspondance{{LibelleNorm: Norm("CSR KASSOPO"), LibelleDRH: "CSR KASSOPO", OrgUnitUID: "kassapo", Statut: CorrOK}},
	)
	got := r.Resolve(AgentRow{Prefecture: "Boké", StructureAffectation: "CSR KASSOPO"})
	if got.Kind != AffStructure || got.Key != "kassapo" || got.Source != SrcNonRecensee {
		t.Fatalf("= %s/%s/%s, attendu structure/kassapo/%s", got.Kind, got.Key, got.Source, SrcNonRecensee)
	}
	if got.District != "DPS Boké" {
		t.Errorf("le district de l'unité doit être repris : %q", got.District)
	}
	// Hors recensement, la structure ne doit pas servir à l'appariement
	// automatique : personne n'a vérifié la correspondance de son nom.
	if got := r.Resolve(AgentRow{Prefecture: "Boké", StructureAffectation: "CSR Kassapo bis"}); got.Kind == AffStructure {
		t.Errorf("apparié automatiquement sur une structure hors recensement : %+v", got)
	}
}

// Un libellé ambigu ne doit surtout pas être rattaché au hasard : deux
// structures « Koule » du même type ne se départagent pas.
func TestResolveAmbiguNeRattachePas(t *testing.T) {
	r := NewResolver([]Structure{
		{UID: "a", Name: "CSR Koule", District: "DPS N'zérékoré", TypeCode: "CS"},
		{UID: "b", Name: "CSU Koule", District: "DPS N'zérékoré", TypeCode: "CS"},
	}, nil)
	got := r.Resolve(AgentRow{Prefecture: "N'zérékoré", StructureAffectation: "CS Koule"})
	if got.Kind != AffNonRattache {
		t.Fatalf("ambigu rattaché à %s/%s", got.Kind, got.Key)
	}
}

func TestResolveAllReport(t *testing.T) {
	r := testResolver(t)
	rows := []AgentRow{
		{Prefecture: "Kankan", StructureAffectation: "HRKkan", Profession: "Médécin Généraliste", Sexe: "H", AnneeNaissance: 1970},
		{Prefecture: "Kankan", StructureAffectation: "HRKkan", Profession: "Sage-Femme", Sexe: "F", AnneeNaissance: 1990},
		{Prefecture: "Forécariah", StructureAffectation: "DPS Forécariah", Profession: "ATS", Sexe: "F"},
		{Prefecture: "Gaoual", StructureAffectation: "CS Youkounkoun", Profession: "ATS", Sexe: "H", AnneeNaissance: 1985},
	}
	affs, rep := ResolveAll(rows, r)
	if rep.NAgents != 4 || rep.NStructure != 2 || rep.NBureau != 1 || rep.NNonRattache != 1 {
		t.Fatalf("rapport inattendu : %+v", rep)
	}
	if rep.NStructuresVues != 1 {
		t.Errorf("structures couvertes = %d, attendu 1", rep.NStructuresVues)
	}
	if len(rep.Inconnus) != 1 || rep.Inconnus[0].NAgents != 1 || rep.Inconnus[0].Libelle != "CS Youkounkoun" {
		t.Errorf("liste des inconnus : %+v", rep.Inconnus)
	}
	if got := rep.PctCategorise(); got != 75 {
		t.Errorf("part catégorisée = %.1f, attendu 75", got)
	}

	eff, pyr := Aggregate(rows, affs, Options{RefYear: 2026, RetirementAge: 60})
	total := find(t, eff, AffStructure, "u-hr-kankan", CategorieToutes)
	if total.NAgents != 2 || total.NFemmes != 1 || total.NDepart5Ans != 1 {
		t.Errorf("total HR Kankan : %+v", total)
	}
	if total.District != "DPS Kankan" {
		t.Errorf("district non propagé : %q", total.District)
	}
	med := find(t, eff, AffStructure, "u-hr-kankan", "MED_GEN")
	if med.NAgents != 1 || med.NDepart10Ans != 1 {
		t.Errorf("médecins HR Kankan : %+v", med)
	}
	// L'agent sans année de naissance est compté, mais dans la tranche inconnue.
	bureau := find(t, eff, AffBureau, "forecariah", CategorieToutes)
	if bureau.NAgents != 1 || bureau.NDepart5Ans != 0 {
		t.Errorf("bureau de district : %+v", bureau)
	}
	var vu bool
	for _, p := range pyr {
		if p.Dimension == AffBureau && p.Categorie == CategorieToutes && p.Tranche == TrancheInconnue && p.NAgents == 1 {
			vu = true
		}
	}
	if !vu {
		t.Error("l'agent sans année de naissance doit apparaître en tranche inconnue")
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
