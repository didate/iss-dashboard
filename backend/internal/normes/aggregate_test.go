package normes

import (
	"testing"

	"iss-dashboard-backend/internal/models"
)

func TestAggregate_SummaryAndGaps(t *testing.T) {
	sf := func(v float64) *float64 { return &v }
	cs1 := EventResult{
		Event: &models.Event{OrgUnitUID: "a", TypeCode: "CS", Region: "R", District: "D1", SousPrefecture: "SP1"},
		Items: []Item{
			{Kind: KindRH, Target: "ISS_RH_SAGEF", Label: "SF", Level: LevelEssentiel, Expected: 2, Observed: sf(0), Status: StatusManque},
			{Kind: KindService, Target: "ISS_SVC_CPN_DE", Label: "CPN", Level: LevelEssentiel, Expected: 1, Observed: sf(1), Status: StatusOK},
		},
		Summary: Summary{Score: sf(50), Conforme: false, NManqueEssentiel: 1},
	}
	cs2 := EventResult{
		Event: &models.Event{OrgUnitUID: "b", TypeCode: "CS", Region: "R", District: "D1"},
		Items: []Item{
			{Kind: KindRH, Target: "ISS_RH_SAGEF", Label: "SF", Level: LevelEssentiel, Expected: 2, Observed: sf(1), Status: StatusManque},
			{Kind: KindService, Target: "ISS_SVC_CPN_DE", Label: "CPN", Level: LevelEssentiel, Expected: 1, Observed: nil, Status: StatusInconnu},
		},
		Summary: Summary{Score: sf(0), Conforme: false, NManqueEssentiel: 1},
	}
	ps := EventResult{
		Event:   &models.Event{OrgUnitUID: "c", TypeCode: "PS", Region: "R", District: "D2"},
		Items:   []Item{{Kind: KindInfra, Target: "ISS_INFRA_LATRINES_DE", Label: "Latrines", Level: LevelEssentiel, Expected: 1, Observed: sf(1), Status: StatusOK}},
		Summary: Summary{Score: sf(100), Conforme: true},
	}
	noRule := EventResult{Event: &models.Event{OrgUnitUID: "d", TypeCode: "INDETERMINE", Region: "R", District: "D2"}}

	srows, grows := Aggregate([]EventResult{cs1, cs2, ps, noRule})

	find := func(dim, key, tc string) *SummaryRow {
		for i := range srows {
			if srows[i].Dimension == dim && srows[i].Key == key && srows[i].TypeCode == tc {
				return &srows[i]
			}
		}
		return nil
	}
	g := find("global", "all", "")
	if g == nil || g.NStructures != 4 || g.NEvaluees != 3 || g.NConformes != 1 || *g.AvgScore != 50 {
		t.Fatalf("global: %+v", g)
	}
	if d1 := find("district", "D1", "CS"); d1 == nil || d1.NStructures != 2 || *d1.AvgScore != 25 || *d1.PctConforme != 0 {
		t.Fatalf("district D1 CS: %+v", d1)
	}
	if ty := find("type", "PS", "PS"); ty == nil || ty.Label != "Poste de santé" || *ty.PctConforme != 100 {
		t.Fatalf("type PS: %+v", ty)
	}
	if find("type", "PS", "") != nil {
		t.Fatal("type dimension must not carry an all-types row")
	}

	var sfGap *GapRow
	for i := range grows {
		if grows[i].Dimension == "district" && grows[i].Key == "D1" && grows[i].Target == "ISS_RH_SAGEF" {
			sfGap = &grows[i]
		}
	}
	if sfGap == nil || sfGap.NConcernees != 2 || sfGap.NManque != 2 || sfGap.Deficit != 3 || sfGap.TypeCode != "CS" {
		t.Fatalf("SF gap D1: %+v", sfGap)
	}
	for _, gr := range grows {
		if gr.Dimension == "region" && gr.Target == "ISS_SVC_CPN_DE" && (gr.NInconnu != 1 || gr.NManque != 0 || gr.Deficit != 0) {
			t.Fatalf("CPN gap must count the unknown, not as a gap: %+v", gr)
		}
	}
}
