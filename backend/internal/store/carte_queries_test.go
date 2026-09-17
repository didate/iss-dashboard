package store

import (
	"math"
	"testing"
)

func TestHaversineKm(t *testing.T) {
	// Donka (Conakry) → HR Kankan : ~ 490 km à vol d'oiseau
	d := haversineKm(9.5359, -13.6837, 10.3846, -9.3059)
	if d < 480 || d > 500 {
		t.Fatalf("Conakry-Kankan distance out of range: %.1f km", d)
	}
	if haversineKm(1, 2, 1, 2) != 0 {
		t.Fatal("same point must be 0")
	}
	if math.Abs(haversineKm(0, 0, 0, 1)-111.19) > 0.2 {
		t.Fatalf("1° of longitude at the equator ≈ 111.19 km, got %.2f", haversineKm(0, 0, 0, 1))
	}
}
