package typologie

import (
	"testing"

	"iss-dashboard-backend/internal/models"
)

const (
	setTypo = "01 TOUTES LES STRUCTURES"
	setHop  = "07 HÖPITAUX"
	setOwn  = "02 PUBLIC PRIVEE"
)

func member(set, group, ou string) models.OrgUnitGroup {
	return models.OrgUnitGroup{GroupUID: "g_" + group, GroupName: group, SetName: set, OrgUnit: ou}
}

func TestResolve_FromSingleGroup(t *testing.T) {
	idx := NewIndex([]models.OrgUnitGroup{member(setTypo, "06 PS", "ou1")}, setTypo, setHop, setOwn)
	r := idx.Resolve(models.OrgUnit{UID: "ou1", Name: "Centre X"}) // name says CS, group must win
	if r.Code != PS || r.Source != SourceGroup {
		t.Fatalf("got %+v, want PS/group", r)
	}
}

func TestResolve_HospitalRefinedBySubtypeSet(t *testing.T) {
	ms := []models.OrgUnitGroup{
		member(setTypo, "05 HOPITAUX", "hn"), member(setHop, "Hôpitaux nationaux", "hn"),
		member(setTypo, "05 HOPITAUX", "hr"), member(setHop, "Hopitaux regionaux", "hr"),
		member(setTypo, "05 HOPITAUX", "hp"), member(setHop, "Hôpitaux Préfectoraux", "hp"),
		member(setTypo, "05 HOPITAUX", "hx"), // no subtype → HP by default
	}
	idx := NewIndex(ms, setTypo, setHop, setOwn)
	for uid, want := range map[string]string{"hn": HN, "hr": HR, "hp": HP, "hx": HP} {
		if got := idx.Resolve(models.OrgUnit{UID: uid, Name: "Hopital"}).Code; got != want {
			t.Errorf("%s: got %s want %s", uid, got, want)
		}
	}
}

func TestResolve_MultipleGroups_PicksMostSpecific(t *testing.T) {
	ms := []models.OrgUnitGroup{member(setTypo, "06 PS", "ou1"), member(setTypo, "02 CS", "ou1")}
	idx := NewIndex(ms, setTypo, setHop, setOwn)
	r := idx.Resolve(models.OrgUnit{UID: "ou1", Name: "PS Foo"})
	if r.Code != CS || r.Source != SourceGroupMultiple {
		t.Fatalf("got %+v, want CS/group_multiple", r)
	}
	if len(r.Groups) != 2 {
		t.Fatalf("expected both groups reported, got %v", r.Groups)
	}
}

func TestResolve_FallbackOnNamePrefix(t *testing.T) {
	idx := NewIndex(nil, setTypo, setHop, setOwn)
	cases := map[string]string{
		"CSR Yombiro":              CS,
		"PS_Patagara":              PS,
		"HP Guéckedou":             HP,
		"Clinique Chinoise ADS":    Clinique,
		"CABINET MEDICAL LA GRACE": Cabinet,
		"CM Michel J. Benjelloun":  Clinique,
		"CHU Donka":                HN,
	}
	for name, want := range cases {
		r := idx.Resolve(models.OrgUnit{UID: "x", Name: name})
		if r.Code != want || r.Source != SourceName {
			t.Errorf("%q: got %+v want %s/name", name, r, want)
		}
	}
}

func TestResolve_UnknownPrefix(t *testing.T) {
	idx := NewIndex([]models.OrgUnitGroup{member(setOwn, "Privé", "priv")}, setTypo, setHop, setOwn)
	if r := idx.Resolve(models.OrgUnit{UID: "pub", Name: "CDT TUBERCULOSE"}); r.Code != Indetermine || r.Source != SourceNone {
		t.Errorf("public unknown: got %+v", r)
	}
	if r := idx.Resolve(models.OrgUnit{UID: "priv", Name: "AGBF Kaloum"}); r.Code != AutrePrive {
		t.Errorf("private unknown: got %+v want AUTRE_PRIVE", r)
	}
}

func TestOwnership(t *testing.T) {
	ms := []models.OrgUnitGroup{member(setOwn, "Public", "a"), member(setOwn, "Privé", "b"), member(setOwn, "01 ASC", "c")}
	idx := NewIndex(ms, setTypo, setHop, setOwn)
	if idx.Ownership("a") != "publique" || idx.Ownership("b") != "privée" || idx.Ownership("c") != "" || idx.Ownership("zz") != "" {
		t.Fatalf("ownership mapping wrong: %q %q %q", idx.Ownership("a"), idx.Ownership("b"), idx.Ownership("c"))
	}
}

func TestSetNamesAreCaseInsensitive(t *testing.T) {
	idx := NewIndex([]models.OrgUnitGroup{member("01 toutes les structures ", "02 CS", "ou1")}, setTypo, setHop, setOwn)
	if idx.Resolve(models.OrgUnit{UID: "ou1"}).Code != CS {
		t.Fatal("set name comparison should ignore case and spaces")
	}
}

func TestNilIndexIsSafe(t *testing.T) {
	var idx *Index
	if r := idx.Resolve(models.OrgUnit{Name: "PS Foo"}); r.Code != PS {
		t.Fatalf("nil index should still resolve by name, got %+v", r)
	}
	if idx.Ownership("x") != "" {
		t.Fatal("nil index ownership should be empty")
	}
}
