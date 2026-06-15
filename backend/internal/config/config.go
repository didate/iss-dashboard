package config

import (
	"os"
	"strings"
)

type Config struct {
	DHIS2BaseURL  string
	DHIS2PAT      string
	DHIS2Program  string
	SQLitePath    string
	SyncCron      string
	AdminToken    string
	DashboardPublic bool
	Port          string
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
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
