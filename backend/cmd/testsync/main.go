// testsync lance une synchronisation complète en ligne de commande, avec la
// même configuration (.env exporté) que le serveur. Utile pour vérifier une
// évolution du pipeline sans passer par l'API admin.
package main

import (
	"log"

	"iss-dashboard-backend/internal/config"
	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/store"
	syncpkg "iss-dashboard-backend/internal/sync"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	client := dhis2.NewClient(cfg.DHIS2BaseURL, cfg.DHIS2PAT, cfg.DHIS2Program)

	log.Println("Starting sync...")
	if _, err := syncpkg.RunSync(st, client, syncpkg.OptionsFromConfig(cfg)); err != nil {
		log.Fatalf("sync failed: %v", err)
	}
	log.Println("Sync complete.")
}
