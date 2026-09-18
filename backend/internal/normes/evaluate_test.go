package normes

import (
	"math"
	"testing"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

func evalCtx() *quality.QualityContext {
	meta := []models.DataElementMeta{
		{UID: "svcCpn", Code: "ISS_SVC_CPN_DE", Name: "CPN", SectionPrefix: "ISS_SVC", OptionSetID: "RGsTov6dBHH"},
		{UID: "sfFn", Code: "ISS_RH_SAGEF_FN_DE", Name: "SF fonct.", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
		{UID: "sfCt", Code: "ISS_RH_SAGEF_CT_DE", Name: "SF contr.", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
		{UID: "medGen", Code: "ISS_RH_MED_GEN_FN_DE", Name: "Méd. gén.", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
		{UID: "medChir", Code: "ISS_RH_MED_CHIR_FN_DE", Name: "Chirurgien", ValueType: "NUMBER", SectionPrefix: "ISS_RH_SPE"},
		{UID: "frigoTot", Code: "ISS_EQUI_FRIGO_TOTAL_DE", Name: "Frigo total", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "frigoFonc", Code: "ISS_EQUI_FRIGO_FONC_DE", Name: "Frigo fonc", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "latrines", Code: "ISS_INFRA_LATRINES_DE", Name: "Latrines", ValueType: "NUMBER", SectionPrefix: "ISS_INFRA"},
	}
	return quality.BuildContext(meta, nil, nil, nil)
}

func evalRules() []models.NormeRule {
	return []models.NormeRule{
		{ID: 1, TypeCode: "CS", Kind: KindService, Target: "ISS_SVC_CPN_DE", Label: "CPN", MinValue: 1, Level: LevelEssentiel},
		{ID: 2, TypeCode: "CS", Kind: KindRH, Target: "ISS_RH_SAGEF", Label: "Sage-femme", MinValue: 2, Level: LevelEssentiel},
		{ID: 3, TypeCode: "CS", Kind: KindRH, Target: "ISS_RH_MED_", Label: "Médecin", MinValue: 1, Level: LevelRecommande},
		{ID: 4, TypeCode: "CS", Kind: KindEquipement, Target: "ISS_EQUI_FRIGO", Label: "Frigo", MinValue: 1, Level: LevelEssentiel},
		{ID: 5, TypeCode: "*", Kind: KindInfra, Target: "ISS_INFRA_LATRINES_DE", Label: "Latrines", MinValue: 1, Level: LevelEssentiel},
		{ID: 6, TypeCode: "*", Kind: KindService, Target: "ISS_SVC_CPN_DE", Label: "CPN (tous)", MinValue: 1, Level: LevelRecommande},
	}
}

func evt(typeCode string, vals map[string]string) *models.Event {
	e := &models.Event{EventUID: "e", OrgUnitUID: "ou", TypeCode: typeCode}
	for k, v := range vals {
		e.DataValues = append(e.DataValues, models.DataValue{DataElement: k, Value: v})
	}
	return e
}

func byRule(items []Item) map[int64]Item {
	m := map[int64]Item{}
	for _, it := range items {
		m[it.RuleID] = it
	}
	return m
}

func TestRulesFor_SpecificOverridesStar(t *testing.T) {
	ev := NewEvaluator(evalRules(), evalCtx())
	cs := ev.RulesFor("CS")
	if len(cs) != 5 { // 4 CS + latrines ; la règle * CPN est masquée par la règle CS CPN
		t.Fatalf("CS rules: %+v", cs)
	}
	for _, r := range cs {
		if r.ID == 6 {
			t.Fatal("star rule must be overridden by the type-specific one")
		}
	}
	if got := ev.RulesFor("INDETERMINE"); len(got) != 2 || got[0].TypeCode != "*" {
		t.Fatalf("INDETERMINE gets only * rules: %+v", got)
	}
	if got := ev.RulesFor("HN"); len(got) != 2 {
		t.Fatalf("type without own rules gets * rules: %+v", got)
	}
}

func TestEvaluate_AllStatuses(t *testing.T) {
	ev := NewEvaluator(evalRules(), evalCtx())
	items := ev.Evaluate(evt("CS", map[string]string{
		"svcCpn":   "oui_pas_fonctionnel", // service prévu mais non fonctionnel → manque
		"sfFn":     "1",
		"sfCt":     "1", // 1 + 1 = 2 ≥ 2 → ok, tous statuts confondus
		"medChir":  "0", // préfixe MED_ : renseigné (0) → manque
		"frigoTot": "3", // total renseigné mais FONC vide → inconnu
		"latrines": "2",
	}))
	m := byRule(items)
	want := map[int64]string{1: StatusManque, 2: StatusOK, 3: StatusManque, 4: StatusInconnu, 5: StatusOK}
	for id, st := range want {
		if m[id].Status != st {
			t.Errorf("rule %d: got %s (observed %v) want %s", id, m[id].Status, m[id].Observed, st)
		}
	}
	if *m[2].Observed != 2 {
		t.Errorf("RH sum across employment statuses: %v", *m[2].Observed)
	}
}

func TestEvaluate_UnknownWhenNothingDeclared(t *testing.T) {
	ev := NewEvaluator(evalRules(), evalCtx())
	items := ev.Evaluate(evt("CS", map[string]string{}))
	for _, it := range items {
		if it.Status != StatusInconnu {
			t.Errorf("%s: %s, want inconnu", it.Label, it.Status)
		}
	}
	s := Summarize(items)
	if s.Score != nil || s.Conforme {
		t.Fatalf("all unknown → no score, not conforme: %+v", s)
	}
}

func TestSummarize_WeightsAndConformity(t *testing.T) {
	items := []Item{
		{Level: LevelEssentiel, Status: StatusOK},
		{Level: LevelEssentiel, Status: StatusOK},
		{Level: LevelRecommande, Status: StatusManque},
		{Level: LevelRecommande, Status: StatusInconnu},
	}
	s := Summarize(items)
	// ok = 2+2 = 4 ; total = 4 + 1 = 5 → 80 ; inconnu hors dénominateur
	if s.Score == nil || math.Abs(*s.Score-80) > 1e-9 || !s.Conforme || s.NInconnu != 1 {
		t.Fatalf("%+v", s)
	}
	items[0].Status = StatusManque
	s = Summarize(items)
	// ok = 2 ; total = 5 → 40 ; un manque essentiel → non conforme
	if math.Abs(*s.Score-40) > 1e-9 || s.Conforme || s.NManqueEssentiel != 1 {
		t.Fatalf("%+v", s)
	}
}

func TestEvaluate_NoRulesForType(t *testing.T) {
	ev := NewEvaluator([]models.NormeRule{{ID: 1, TypeCode: "HP", Kind: KindService, Target: "ISS_SVC_CPN_DE", MinValue: 1, Level: LevelEssentiel}}, evalCtx())
	if items := ev.Evaluate(evt("PS", map[string]string{"svcCpn": "oui"})); items != nil {
		t.Fatalf("PS has no rule: %+v", items)
	}
}
