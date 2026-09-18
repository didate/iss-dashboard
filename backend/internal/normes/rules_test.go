package normes

import (
	"bytes"
	"strings"
	"testing"

	"iss-dashboard-backend/internal/models"
)

func testCatalog() *Catalog {
	c := NewCatalog()
	c.Add(Target{Kind: KindService, Code: "ISS_SVC_CPN_DE", Label: "CPN"})
	c.Add(Target{Kind: KindRH, Code: "ISS_RH_SAGEF", Label: "Sage-femme"})
	c.Add(Target{Kind: KindRH, Code: "ISS_RH_MED_GEN", Label: "Médecin généraliste"})
	c.Add(Target{Kind: KindRH, Code: "ISS_RH_MED_CHIR", Label: "Chirurgien"})
	c.Add(Target{Kind: KindEquipement, Code: "ISS_EQUI_FRIGO", Label: "Réfrigérateur"})
	c.Add(Target{Kind: KindInfra, Code: "ISS_INFRA_LATRINES_DE", Label: "Latrines"})
	return c
}

func TestParseCSV_ValidAndInvalidLines(t *testing.T) {
	csv := `# commentaire
type_code;kind;target;label;min_value;level
CS;service;ISS_SVC_CPN_DE;;;essentiel
cs;RH;ISS_RH_SAGEF;Sage-femme;2;Recommandé
HP;rh;ISS_RH_MED_;;3;essentiel
*;infra;ISS_INFRA_LATRINES_DE;Latrines;1;essentiel
XX;service;ISS_SVC_CPN_DE;;;essentiel
CS;equipement;ISS_EQUI_INCONNU;;1;essentiel
CS;rh;ISS_RH_SAGEF;;abc;essentiel
CS;rh;ISS_RH_SAGEF;;0;essentiel
`
	rules, errs := ParseCSV(strings.NewReader(csv), testCatalog())
	if len(rules) != 4 {
		t.Fatalf("expected 4 valid rules, got %d: %+v", len(rules), rules)
	}
	if len(errs) != 4 {
		t.Fatalf("expected 4 errors, got %+v", errs)
	}
	if rules[0].Label != "CPN" || rules[0].MinValue != 1 || rules[0].Level != LevelEssentiel {
		t.Errorf("service rule must take label from catalog and min 1: %+v", rules[0])
	}
	if rules[1].TypeCode != "CS" || rules[1].Kind != KindRH || rules[1].Level != LevelRecommande || rules[1].MinValue != 2 {
		t.Errorf("normalisation: %+v", rules[1])
	}
	if !strings.HasPrefix(rules[2].Label, "Tout profil MED") {
		t.Errorf("prefix target must get a generated label: %q", rules[2].Label)
	}
	for _, e := range errs {
		if e.Line < 7 {
			t.Errorf("error reported on a valid line: %+v", e)
		}
	}
}

func TestValidate_ServiceMinForcedToOne(t *testing.T) {
	r := models.NormeRule{TypeCode: "ps", Kind: "service", Target: "ISS_SVC_CPN_DE", MinValue: 5}
	if msg := Validate(&r, testCatalog()); msg != "" {
		t.Fatal(msg)
	}
	if r.MinValue != 1 || r.TypeCode != "PS" {
		t.Fatalf("got %+v", r)
	}
}

func TestResolve_PrefixNeedsAMatch(t *testing.T) {
	c := testCatalog()
	if _, ok := c.Resolve(KindRH, "ISS_RH_MED_"); !ok {
		t.Fatal("prefix with matches must resolve")
	}
	if _, ok := c.Resolve(KindRH, "ISS_RH_DENT_"); ok {
		t.Fatal("prefix without match must fail")
	}
	if _, ok := c.Resolve(KindService, "ISS_SVC_"); ok {
		t.Fatal("prefixes are RH only")
	}
}

func TestWriteCSV_RoundTrip(t *testing.T) {
	in := []models.NormeRule{
		{TypeCode: "CS", Kind: KindRH, Target: "ISS_RH_SAGEF", Label: "Sage-femme", MinValue: 1.5, Level: LevelEssentiel},
		{TypeCode: "*", Kind: KindInfra, Target: "ISS_INFRA_LATRINES_DE", Label: "Latrines; oui", MinValue: 1, Level: LevelRecommande},
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, errs := ParseCSV(&buf, testCatalog())
	if len(errs) != 0 || len(out) != 2 {
		t.Fatalf("round trip failed: %+v %+v", out, errs)
	}
	if out[0].MinValue != 1.5 || out[1].Label != "Latrines; oui" {
		t.Fatalf("values altered: %+v", out)
	}
}

func TestExampleCSV_FormatIsValid(t *testing.T) {
	// The example referential must at least be well-formed (targets are checked
	// against the real catalog at import time).
	data, err := readExample()
	if err != nil {
		t.Skip(err)
	}
	rules, errs := ParseCSV(bytes.NewReader(data), nil)
	if len(errs) != 0 {
		t.Fatalf("docs/normes-exemple.csv has invalid lines: %+v", errs)
	}
	if len(rules) < 100 {
		t.Fatalf("expected a substantial example, got %d rules", len(rules))
	}
}
