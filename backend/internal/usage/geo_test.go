package usage

import (
	"math"
	"testing"

	"iss-dashboard-backend/internal/models"
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
	rh := []models.UsageRH{
		{ProfilCode: "ISS_RH_MED_GEN", District: "all", EffectifTotal: 10},
		{ProfilCode: "ISS_RH_MED_CHIR", District: "all", EffectifTotal: 2},
		{ProfilCode: "ISS_RH_MED_GEN", District: "Kankan D", EffectifTotal: 4},
		{ProfilCode: "ISS_RH_SAGEF", District: "Kankan D", EffectifTotal: 3},
		{ProfilCode: "ISS_RH_INF", District: "Siguiri", EffectifTotal: 7},
	}
	eq := []models.UsageEquipement{
		{EquipRoot: "ISS_EQUI_LIT", District: "all", SumTotal: 100},
		{EquipRoot: "ISS_EQUI_LIT", District: "Kankan D", SumTotal: 40},
		{EquipRoot: "ISS_EQUI_FRIGO", District: "Kankan D", SumTotal: 9}, // ignoré
	}
	pop := PopulationIndex{"gn": 1_000_000, "d1": 20000} // pas de population régionale

	rows := ComputeCouverture(events, orgUnits, rh, eq, pop)
	get := func(dim, key, ind string) *models.UsageCouverture {
		for i := range rows {
			if rows[i].Dimension == dim && rows[i].Key == key && rows[i].Indicator == ind {
				return &rows[i]
			}
		}
		return nil
	}

	if r := get("global", "all", "medecins"); r == nil || r.Numerator != 12 || !near(r.Ratio10k, 0.12) {
		t.Fatalf("global medecins: %+v", r)
	}
	if r := get("district", "Kankan D", "structures"); r == nil || r.Numerator != 2 || !near(r.Ratio10k, 1) {
		t.Fatalf("district structures (distinct OU): %+v", r)
	}
	if r := get("district", "Kankan D", "lits"); r == nil || r.Numerator != 40 || !near(r.Ratio10k, 20) {
		t.Fatalf("district lits: %+v", r)
	}
	if r := get("region", "Kankan", "infirmiers"); r == nil || r.Numerator != 7 || r.Population != nil || r.Ratio10k != nil {
		t.Fatalf("region without population must sum numerators but leave ratio nil: %+v", r)
	}
	if r := get("sous_prefecture", "Balandou", "structures"); r == nil || r.Numerator != 1 {
		t.Fatalf("sous-préfecture structures: %+v", r)
	}
	if r := get("district", "Kankan D", "frigo"); r != nil {
		t.Fatal("unrelated equipment must not produce an indicator")
	}
}
