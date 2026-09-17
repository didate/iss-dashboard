package api

import "testing"

func TestParseLatLng(t *testing.T) {
	lat, lng, ok := parseLatLng(" 9.535 , -13.68 ")
	if !ok || lat != 9.535 || lng != -13.68 {
		t.Fatalf("got %v %v %v", lat, lng, ok)
	}
	for _, bad := range []string{"", "9.5", "a,b", "91,0", "0,181", "9.5,-13.6,2"} {
		if _, _, ok := parseLatLng(bad); ok {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
