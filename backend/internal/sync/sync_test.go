package sync

import (
	"testing"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/typologie"
)

func TestPointCoordinates(t *testing.T) {
	lat, lng := pointCoordinates(`{"type":"Point","coordinates":[-13.675516,9.533432]}`)
	if lat == nil || lng == nil || *lat != 9.533432 || *lng != -13.675516 {
		t.Fatalf("GeoJSON is [lng, lat]; got lat=%v lng=%v", lat, lng)
	}
	for name, g := range map[string]string{
		"empty":   "",
		"polygon": `{"type":"Polygon","coordinates":[[[0,0],[1,1],[1,0],[0,0]]]}`,
		"null":    `null`,
		"zero":    `{"type":"Point","coordinates":[0,0]}`,
		"range":   `{"type":"Point","coordinates":[200,9]}`,
	} {
		if la, lo := pointCoordinates(g); la != nil || lo != nil {
			t.Errorf("%s: expected nil coordinates", name)
		}
	}
}

func TestEnrichEvent_HierarchyGPSAndType(t *testing.T) {
	units := []models.OrgUnit{
		{UID: "gn", Name: "Guinée", Level: 1},
		{UID: "r1", Name: "Kankan", Level: 2, ParentUID: "gn"},
		{UID: "d1", Name: "Kankan D", Level: 3, ParentUID: "r1"},
		{UID: "sp1", Name: "Balandou", Level: 4, ParentUID: "d1"},
		{UID: "cs1", Name: "CSR Balandou", Level: 5, ParentUID: "sp1", Geometry: `{"type":"Point","coordinates":[-9.3,10.4]}`},
		{UID: "ps1", Name: "PS Foo", Level: 6, ParentUID: "cs1"},
	}
	groups := []models.OrgUnitGroup{{GroupUID: "g", GroupName: "02 CS", SetName: "01 TOUTES LES STRUCTURES", OrgUnit: "cs1"}}
	typo := typologie.NewIndex(groups, "01 TOUTES LES STRUCTURES", "07 HÖPITAUX", "02 PUBLIC PRIVEE")
	orgMap := buildOrgUnitMap(units)

	cs := &models.Event{EventUID: "e1", OrgUnitUID: "cs1"}
	enrichEvent(cs, orgMap, typo)
	if cs.Region != "Kankan" || cs.District != "Kankan D" || cs.DistrictUID != "d1" || cs.SousPrefecture != "Balandou" || cs.SousPrefectureUID != "sp1" {
		t.Fatalf("hierarchy: %+v", cs)
	}
	if !cs.HasGPS() || *cs.Lat != 10.4 || *cs.Lng != -9.3 {
		t.Fatalf("gps: lat=%v lng=%v", cs.Lat, cs.Lng)
	}
	if cs.TypeCode != typologie.CS || cs.TypeSource != typologie.SourceGroup {
		t.Fatalf("type: %s/%s", cs.TypeCode, cs.TypeSource)
	}

	ps := &models.Event{EventUID: "e2", OrgUnitUID: "ps1"}
	enrichEvent(ps, orgMap, typo)
	if ps.HasGPS() {
		t.Fatal("level-6 unit without geometry must not inherit its parent's point")
	}
	if ps.TypeCode != typologie.PS || ps.TypeSource != typologie.SourceName {
		t.Fatalf("type by name: %s/%s", ps.TypeCode, ps.TypeSource)
	}
	if ps.SousPrefecture != "Balandou" {
		t.Fatalf("level-6 unit must still find its sous-préfecture: %+v", ps)
	}

	// Nil typology index (groups not loaded) must not panic.
	orphan := &models.Event{EventUID: "e3", OrgUnitUID: "cs1"}
	enrichEvent(orphan, orgMap, nil)
	if orphan.TypeCode != typologie.CS || orphan.TypeSource != typologie.SourceName {
		t.Fatalf("without groups, name prefix must resolve: %s/%s", orphan.TypeCode, orphan.TypeSource)
	}
}
