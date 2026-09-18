package usage

import (
	"math"
	"testing"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

func pf(v float64) *float64 { return &v }

func near(p *float64, want float64) bool { return p != nil && math.Abs(*p-want) < 1e-9 }

func geoFixture() ([]*models.Event, []models.OrgUnit) {
	orgUnits := []models.OrgUnit{
		{UID: "gn", Name: "Guinée", Level: 1},
		{UID: "r1", Name: "Kankan", Level: 2, ParentUID: "gn", ParentName: "Guinée"},
		{UID: "d1", Name: "Kankan D", Level: 3, ParentUID: "r1", ParentName: "Kankan"},
		{UID: "d2", Name: "Siguiri", Level: 3, ParentUID: "r1", ParentName: "Kankan"},
		{UID: "sp1", Name: "Balandou", Level: 4, ParentUID: "d1", ParentName: "Kankan D"},
	}
	events := []*models.Event{
		{EventUID: "e1", OrgUnitUID: "ou1", Region: "Kankan", District: "Kankan D", DistrictUID: "d1", SousPrefecture: "Balandou", SousPrefectureUID: "sp1", TypeCode: "CS", Lat: pf(1), Lng: pf(2)},
		{EventUID: "e2", OrgUnitUID: "ou2", Region: "Kankan", District: "Kankan D", DistrictUID: "d1", TypeCode: "PS"},
		{EventUID: "e3", OrgUnitUID: "ou2", Region: "Kankan", District: "Kankan D", DistrictUID: "d1", TypeCode: "PS"}, // doublon même OU
	}
	return events, orgUnits
}

func TestComputeGeo_CountsDistinctOrgUnits(t *testing.T) {
	events, orgUnits := geoFixture()
	qual := []models.EventQuality{{EventUID: "e1", Score: 100}, {EventUID: "e2", Score: 50}, {EventUID: "e3", Score: 50}}
	pop := PopulationIndex{"d1": 20000}

	rows := ComputeGeo(events, orgUnits, qual, pop)
	byUID := map[string]models.UsageGeo{}
	for _, r := range rows {
		byUID[r.OrgUnitUID] = r
	}

	d1 := byUID["d1"]
	if d1.NStructures != 2 || d1.NGPS != 1 || d1.PctGPS == nil || *d1.PctGPS != 50 {
		t.Fatalf("d1: %+v", d1)
	}
	if d1.NParType["CS"] != 1 || d1.NParType["PS"] != 1 {
		t.Fatalf("d1 types: %v", d1.NParType)
	}
	if d1.AvgScore == nil || *d1.AvgScore != (100+50+50)/3.0 {
		t.Fatalf("d1 avg score: %v", d1.AvgScore)
	}
	if d1.Population == nil || *d1.Population != 20000 {
		t.Fatalf("d1 population: %v", d1.Population)
	}
	d2 := byUID["d2"]
	if d2.NStructures != 0 || d2.PctGPS != nil || d2.Population != nil {
		t.Fatalf("empty district must still be listed with zero counts: %+v", d2)
	}
	if sp := byUID["sp1"]; sp.Level != 4 || sp.NStructures != 1 {
		t.Fatalf("sous-préfecture: %+v", sp)
	}
	if _, ok := byUID["r1"]; ok {
		t.Fatal("level 2 must not appear in usage_geo")
	}
}

func TestComputeCouverture_RatiosAndPopulationScope(t *testing.T) {
	events, orgUnits := geoFixture()
	meta := []models.DataElementMeta{
		{UID: "medGen", Code: "ISS_RH_MED_GEN_FN_DE", Name: "Médecin généraliste", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
		{UID: "medChir", Code: "ISS_RH_MED_CHIR_CT_DE", Name: "Chirurgien", ValueType: "NUMBER", SectionPrefix: "ISS_RH_SPE"},
		{UID: "sf", Code: "ISS_RH_SAGEF_FN_DE", Name: "Sage-femme", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
		{UID: "inf", Code: "ISS_RH_INF_FN_DE", Name: "Infirmier", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
		{UID: "litTot", Code: "ISS_EQUI_LIT_TOTAL_DE", Name: "Lits total", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "litFonc", Code: "ISS_EQUI_LIT_FONC_DE", Name: "Lits fonctionnels", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "frigo", Code: "ISS_EQUI_FRIGO_TOTAL_DE", Name: "Frigos", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "frigoF", Code: "ISS_EQUI_FRIGO_FONC_DE", Name: "Frigos fonc", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
	}
	// e1 (ou1, Balandou) : 4 médecins gén., 3 SF, 40 lits ; e2/e3 (ou2, doublon) : 2 chirurgiens, 7 infirmiers
	events[0].DataValues = []models.DataValue{{DataElement: "medGen", Value: "4"}, {DataElement: "sf", Value: "3"}, {DataElement: "litTot", Value: "40"}, {DataElement: "litFonc", Value: "30"}, {DataElement: "frigo", Value: "9"}}
	events[1].DataValues = []models.DataValue{{DataElement: "medChir", Value: "2"}, {DataElement: "inf", Value: "7"}}
	events[2].DataValues = events[1].DataValues
	ctx := quality.BuildContext(meta, nil, events, orgUnits)
	pop := PopulationIndex{"gn": 1_000_000, "d1": 20000, "sp1": 5000} // pas de population régionale

	rows := ComputeCouverture(events, orgUnits, ctx, pop)
	get := func(dim, key, ind string) *models.UsageCouverture {
		for i := range rows {
			if rows[i].Dimension == dim && rows[i].Key == key && rows[i].Indicator == ind {
				return &rows[i]
			}
		}
		return nil
	}

	if r := get("global", "all", "medecins"); r == nil || r.Numerator != 6 || !near(r.Ratio10k, 0.06) {
		t.Fatalf("global medecins (gén. + chirurgien, doublon compté une fois): %+v", r)
	}
	if r := get("district", "Kankan D", "structures"); r == nil || r.Numerator != 2 || !near(r.Ratio10k, 1) || r.OrgUnitUID != "d1" {
		t.Fatalf("district structures (distinct OU): %+v", r)
	}
	if r := get("district", "Kankan D", "lits"); r == nil || r.Numerator != 40 || !near(r.Ratio10k, 20) {
		t.Fatalf("district lits (total, pas fonctionnel): %+v", r)
	}
	if r := get("district", "Kankan D", "personnel_soignant"); r == nil || r.Numerator != 4+3+2+7 {
		t.Fatalf("personnel soignant = médecins + SF + infirmiers + ATS: %+v", r)
	}
	if r := get("region", "Kankan", "infirmiers"); r == nil || r.Numerator != 7 || r.Population != nil || r.Ratio10k != nil {
		t.Fatalf("region without population must sum numerators but leave ratio nil: %+v", r)
	}
	if r := get("sous_prefecture", "Balandou", "sages_femmes"); r == nil || r.Numerator != 3 || !near(r.Ratio10k, 6) || r.OrgUnitUID != "sp1" {
		t.Fatalf("sous-préfecture sages-femmes /10k: %+v", r)
	}
	if r := get("district", "Kankan D", "frigo"); r != nil {
		t.Fatal("unrelated equipment must not produce an indicator")
	}
}
