package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// PopulationSpec maps a named population indicator to the DHIS2 data elements
// that compose it (their values are summed for the same org unit and period).
type PopulationSpec struct {
	Indicator string
	UIDs      []string
}

type Config struct {
	DHIS2BaseURL    string
	DHIS2PAT        string
	DHIS2Program    string
	SQLitePath      string
	SyncCron        string
	AdminToken      string
	DashboardPublic bool
	Port            string

	// Carte sanitaire : population et typologie (UIDs / noms propres à l'instance)
	PopulationDX      []PopulationSpec
	PopulationLevels  []int
	PopulationFactor  float64 // multiplie la dernière valeur mensuelle (12 si valeur mensuelle = annuel/12)
	TypologyGroupSet  string
	HospitalGroupSet  string
	OwnershipGroupSet string
}

func Load() *Config {
	return &Config{
		DHIS2BaseURL:    strings.TrimRight(getEnv("DHIS2_BASE_URL", "https://dhis2.example.com"), "/"),
		DHIS2PAT:        getEnv("DHIS2_PAT", ""),
		DHIS2Program:    getEnv("DHIS2_PROGRAM_ID", "AJy1cnAA50U"),
		SQLitePath:      getEnv("SQLITE_PATH", "./iss.db"),
		SyncCron:        getEnv("SYNC_CRON", "0 */6 * * *"),
		AdminToken:      getEnv("ADMIN_TOKEN", ""),
		DashboardPublic: getEnv("DASHBOARD_PUBLIC", "true") == "true",
		Port:            getEnv("PORT", "8080"),

		PopulationDX:      ParsePopulationDX(getEnv("DHIS2_POPULATION_DX", "")),
		PopulationLevels:  parseIntList(getEnv("DHIS2_POPULATION_LEVELS", "1,2,3,4")),
		PopulationFactor:  parseFloat(getEnv("DHIS2_POPULATION_FACTOR", "12"), 12),
		TypologyGroupSet:  getEnv("DHIS2_TYPOLOGY_GROUPSET", "01 TOUTES LES STRUCTURES"),
		HospitalGroupSet:  getEnv("DHIS2_HOSPITAL_GROUPSET", "07 HÖPITAUX"),
		OwnershipGroupSet: getEnv("DHIS2_OWNERSHIP_GROUPSET", "02 PUBLIC PRIVEE"),
	}
}

// ParsePopulationDX parses "total:uid1+uid2,moins5:uid3" into specs.
// Malformed entries are skipped with a log line rather than aborting startup.
func ParsePopulationDX(raw string) []PopulationSpec {
	var specs []PopulationSpec
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, uids, ok := strings.Cut(entry, ":")
		if !ok || strings.TrimSpace(name) == "" {
			log.Printf("WARN: DHIS2_POPULATION_DX: entrée ignorée (format attendu indicateur:uid[+uid]) : %q", entry)
			continue
		}
		var list []string
		for _, u := range strings.Split(uids, "+") {
			if u = strings.TrimSpace(u); u != "" {
				list = append(list, u)
			}
		}
		if len(list) == 0 {
			log.Printf("WARN: DHIS2_POPULATION_DX: aucun UID pour %q", name)
			continue
		}
		specs = append(specs, PopulationSpec{Indicator: strings.TrimSpace(name), UIDs: list})
	}
	return specs
}

func parseIntList(raw string) []int {
	var out []int
	for _, p := range strings.Split(raw, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func parseFloat(raw string, fallback float64) float64 {
	if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && f > 0 {
		return f
	}
	log.Printf("WARN: valeur numérique invalide %q, utilisation de %v", raw, fallback)
	return fallback
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
