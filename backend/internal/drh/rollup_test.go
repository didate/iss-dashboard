package drh

import "testing"

// jeu minimal : un district (Kankan) avec un hôpital et son bureau, un second
// district (Boké) avec un centre de santé et un agent non rattaché, et une
// administration centrale.
func fixtureFine(t *testing.T) ([]EffectifRow, []PyramideRow, RollupContext) {
	t.Helper()
	unites := []UniteOrg{
		{UID: "hr", Name: "HR Kankan", Level: 5, District: "DPS Kankan", Region: "IRS Kankan", TypeCode: "HR",
			SousPrefecture: "Kankan Centre", SousPrefectureUID: "sp-kankan"},
		{UID: "cs", Name: "CSR Kolaboui", Level: 5, District: "DPS Boké", Region: "IRS Boké", TypeCode: "CS",
			SousPrefecture: "Kolaboui", SousPrefectureUID: "sp-kolaboui"},
		{UID: "dis-kankan", Name: "DPS Kankan", Level: 3, District: "DPS Kankan", Region: "IRS Kankan"},
		{UID: "reg-boke", Name: "IRS Boké", Level: 2, Region: "IRS Boké"},
		{UID: "gn", Name: "Guinée", Level: 1},
	}
	rows := []AgentRow{
		{UIDDhis2: "hr", Prefecture: "Kankan", Profession: "Médécin Généraliste", Sexe: "H", AnneeNaissance: 1968},
		{UIDDhis2: "hr", Prefecture: "Kankan", Profession: "Sage-Femme", Sexe: "F", AnneeNaissance: 1990},
		{UIDDhis2: "dis-kankan", Prefecture: "Kankan", Profession: "ATS", Sexe: "F", AnneeNaissance: 1985},
		{UIDDhis2: "cs", Prefecture: "Boké", Profession: "Médécin Généraliste", Sexe: "F", AnneeNaissance: 1975},
		{UIDDhis2: "reg-boke", Prefecture: "Boké", Profession: "ATS", Sexe: "H", AnneeNaissance: 1995},
		{UIDDhis2: "gn", Prefecture: "Kaloum", Profession: "Administrateur Civil", Sexe: "H", AnneeNaissance: 1970},
	}
	affs, _ := ResolveAll(rows, NewResolver(unites))
	eff, pyr := Aggregate(rows, affs, Options{RefYear: 2026, RetirementAge: 60})

	byUID := map[string]UniteOrg{}
	for _, u := range unites {
		byUID[u.UID] = u
	}
	ctx := RollupContext{Unites: byUID, Population: map[string]float64{
		PopKey(DimGlobal, KeyNational):    13_000_000,
		PopKey(DimDistrict, "DPS Kankan"): 500_000,
	}}
	return eff, pyr, ctx
}

func findRollup(t *testing.T, rows []EffectifRow, dim, key, cat string) EffectifRow {
	t.Helper()
	for _, r := range rows {
		if r.Dimension == dim && r.Key == key && r.Categorie == cat {
			return r
		}
	}
	t.Fatalf("cellule %s/%s/%q absente", dim, key, cat)
	return EffectifRow{}
}

func TestRollup(t *testing.T) {
	eff, pyr, ctx := fixtureFine(t)
	rEff, rPyr := Rollup(eff, pyr, ctx)

	national := findRollup(t, rEff, DimGlobal, KeyNational, CategorieToutes)
	if national.NAgents != 6 {
		t.Fatalf("national = %d agents, attendu 6", national.NAgents)
	}
	if national.NStructure != 3 || national.NBureau != 1 || national.NBureauRegional != 1 || national.NCentrale != 1 {
		t.Errorf("répartition nationale : %+v", national)
	}

	// Le district compte ses structures, son bureau et ses non rattachés.
	kankan := findRollup(t, rEff, DimDistrict, "DPS Kankan", CategorieToutes)
	if kankan.NAgents != 3 || kankan.NStructure != 2 || kankan.NBureau != 1 {
		t.Errorf("DPS Kankan : %+v", kankan)
	}
	// Le cadre régional ne tombe dans aucun district : Boké n'a que sa structure.
	boke := findRollup(t, rEff, DimDistrict, "DPS Boké", CategorieToutes)
	if boke.NAgents != 1 || boke.NBureauRegional != 0 {
		t.Errorf("DPS Boké : %+v", boke)
	}
	regBoke := findRollup(t, rEff, DimRegion, "IRS Boké", CategorieToutes)
	if regBoke.NAgents != 2 || regBoke.NBureauRegional != 1 {
		t.Errorf("région Boké : %+v — le cadre régional doit y compter", regBoke)
	}

	// L'administration centrale ne tombe dans aucun district ni région.
	for _, r := range rEff {
		if (r.Dimension == DimDistrict || r.Dimension == DimRegion) && r.NCentrale > 0 {
			t.Errorf("l'administration centrale ne doit compter qu'au national : %+v", r)
		}
	}
	if national.NAgents != kankan.NAgents+boke.NAgents+2 {
		t.Errorf("le national = districts + centrale + bureau régional")
	}

	// Sous-préfecture et type : seulement les agents rattachés à une structure.
	sp := findRollup(t, rEff, DimSousPrefecture, "sp-kankan", CategorieToutes)
	if sp.NAgents != 2 || sp.Label != "Kankan Centre" || sp.District != "DPS Kankan" {
		t.Errorf("sous-préfecture : %+v", sp)
	}
	hr := findRollup(t, rEff, DimType, "HR", CategorieToutes)
	if hr.NAgents != 2 || hr.Label == "HR" {
		t.Errorf("type HR : %+v (libellé non résolu ?)", hr)
	}
	var spTotal int
	for _, r := range rEff {
		if r.Dimension == DimSousPrefecture && r.Categorie == CategorieToutes {
			spTotal += r.NAgents
		}
	}
	if spTotal != 3 {
		t.Errorf("les sous-préfectures totalisent %d agents, attendu les 3 en structure", spTotal)
	}

	// Densité : 3 agents pour 500 000 habitants = 0,06 / 10 000.
	if kankan.Ratio10k == nil || *kankan.Ratio10k < 0.0599 || *kankan.Ratio10k > 0.0601 {
		t.Errorf("densité de Kankan : %v", kankan.Ratio10k)
	}
	if boke.Ratio10k != nil {
		t.Errorf("sans population connue, la densité doit rester nulle : %v", *boke.Ratio10k)
	}

	// Départs : seul le médecin né en 1968 (58 ans) part d'ici 5 ans.
	if national.NDepart5Ans != 2 || national.NDepart10Ans != 3 {
		t.Errorf("départs nationaux : %d à 5 ans, %d à 10 ans", national.NDepart5Ans, national.NDepart10Ans)
	}
	med := findRollup(t, rEff, DimGlobal, KeyNational, "MED_GEN")
	if med.NAgents != 2 || med.NFemmes != 1 {
		t.Errorf("médecins généralistes : %+v", med)
	}

	// Pyramide : cumulée au national, pas à la sous-préfecture.
	var pyrNational, pyrSP int
	for _, r := range rPyr {
		switch {
		case r.Dimension == DimGlobal && r.Categorie == CategorieToutes:
			pyrNational += r.NAgents
		case r.Dimension == DimSousPrefecture:
			pyrSP++
		}
	}
	if pyrNational != 6 {
		t.Errorf("pyramide nationale = %d agents, attendu 6", pyrNational)
	}
	if pyrSP != 0 {
		t.Errorf("pas de pyramide à la sous-préfecture, %d lignes trouvées", pyrSP)
	}
}

// Un rollup ne doit jamais se cumuler lui-même si on le repasse en entrée.
func TestRollupIdempotent(t *testing.T) {
	eff, pyr, ctx := fixtureFine(t)
	first, firstPyr := Rollup(eff, pyr, ctx)
	second, _ := Rollup(append(append([]EffectifRow{}, eff...), first...), append(append([]PyramideRow{}, pyr...), firstPyr...), ctx)
	a := findRollup(t, first, DimGlobal, KeyNational, CategorieToutes)
	b := findRollup(t, second, DimGlobal, KeyNational, CategorieToutes)
	if a.NAgents != b.NAgents {
		t.Fatalf("recalcul non idempotent : %d puis %d", a.NAgents, b.NAgents)
	}
}

func TestCompare(t *testing.T) {
	eff, pyr, ctx := fixtureFine(t)
	rEff, _ := Rollup(eff, pyr, ctx)
	iss := []ISSRH{
		{District: "all", ProfilCode: "ISS_RH_MED_GEN", Effectif: 6},
		{District: "all", ProfilCode: "ISS_RH_SAGEF", Effectif: 4},
		{District: "all", ProfilCode: "ISS_RH_ATS", Effectif: 1}, // moins que la DRH : incohérence
		{District: "DPS Kankan", ProfilCode: "ISS_RH_MED_GEN", Effectif: 3},
	}
	rows := Compare(rEff, iss)

	get := func(dim, key, cat string) ComparaisonRow {
		t.Helper()
		for _, r := range rows {
			if r.Dimension == dim && r.Key == key && r.Categorie == cat {
				return r
			}
		}
		t.Fatalf("comparaison %s/%s/%q absente", dim, key, cat)
		return ComparaisonRow{}
	}

	med := get(DimGlobal, KeyNational, "MED_GEN")
	if med.NDrh != 2 || med.NIss == nil || *med.NIss != 6 || *med.Ratio != 3 || *med.Ecart != 4 {
		t.Errorf("médecins : %+v", med)
	}

	if med.PartEtat == nil || *med.PartEtat < 33.2 || *med.PartEtat > 33.4 {
		t.Errorf("part payée par l'État pour les médecins : %v, attendu 2/6 = 33,3 %%", med.PartEtat)
	}

	// L'État paie plus d'ATS que les structures n'en déclarent : les deux
	// nomenclatures ne se recouvrent pas, le rapport n'est donc pas une part et
	// la catégorie sort du périmètre comparable.
	ats := get(DimGlobal, KeyNational, "ATS")
	if ats.NDrh != 2 || *ats.Ratio >= 1 {
		t.Errorf("ATS : %+v — le ratio doit signaler l'incohérence", ats)
	}
	if ats.Aligne {
		t.Errorf("ATS ne doit pas être marquée alignée : %+v", ats)
	}
	if ats.PartEtat != nil {
		t.Errorf("une catégorie non alignée ne doit pas porter de part : %v %%", *ats.PartEtat)
	}

	// La ligne « toutes catégories » ne somme que le périmètre comparable :
	// médecins (2/6) et sages-femmes (1/4), sans les ATS ni l'administratif.
	tot := get(DimGlobal, KeyNational, CategorieToutes)
	if tot.NDrh != 3 || tot.NIss == nil || *tot.NIss != 10 {
		t.Errorf("toutes catégories : %+v (attendu 3 DRH / 10 ISS)", tot)
	}
	if tot.PartEtat == nil || *tot.PartEtat < 29.9 || *tot.PartEtat > 30.1 {
		t.Errorf("part nationale : %v, attendu 3/10 = 30 %%", tot.PartEtat)
	}

	// Un district sans effectif ISS pour la catégorie n'invente pas de zéro.
	sagef := get(DimDistrict, "DPS Kankan", "SAGEF")
	if sagef.NIss != nil || sagef.Ecart != nil || sagef.Ratio != nil {
		t.Errorf("sans donnée ISS, l'écart doit rester nul : %+v", sagef)
	}

	for _, r := range rows {
		if r.Dimension != DimGlobal && r.Dimension != DimDistrict {
			t.Fatalf("dimension non comparable : %s", r.Dimension)
		}
	}
}
