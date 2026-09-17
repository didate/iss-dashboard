package config

import "testing"

func TestParsePopulationDX(t *testing.T) {
	specs := ParsePopulationDX("total:ksBi2JIApqW+oVYNP4fGnTo, moins5:hLcbHlNiRqP ,bad,:x,empty:")
	if len(specs) != 2 {
		t.Fatalf("expected 2 valid specs, got %+v", specs)
	}
	if specs[0].Indicator != "total" || len(specs[0].UIDs) != 2 || specs[0].UIDs[1] != "oVYNP4fGnTo" {
		t.Fatalf("total: %+v", specs[0])
	}
	if specs[1].Indicator != "moins5" || len(specs[1].UIDs) != 1 {
		t.Fatalf("moins5: %+v", specs[1])
	}
	if ParsePopulationDX("") != nil {
		t.Fatal("empty string must yield nil")
	}
}

func TestParseIntList(t *testing.T) {
	got := parseIntList("1, 2,x,4,-1")
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 4 {
		t.Fatalf("got %v", got)
	}
}
