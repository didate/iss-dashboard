package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"iss-dashboard-backend/internal/api"
	"iss-dashboard-backend/internal/config"
	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/scheduler"
	"iss-dashboard-backend/internal/store"
)

func main() {
	cfg := config.Load()

	log.Println("[INIT] Opening database:", cfg.SQLitePath)
	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer st.Close()

	client := dhis2.NewClient(cfg.DHIS2BaseURL, cfg.DHIS2PAT, cfg.DHIS2Program)

	// Start scheduler
	sched := scheduler.New(st, client)
	if err := sched.Start(cfg.SyncCron); err != nil {
		log.Printf("[WARN] Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// Setup and start HTTP server
	router := api.SetupRouter(cfg, st, client)

	log.Printf("[INIT] Starting server on :%s", cfg.Port)
	go func() {
		if err := router.Run(":" + cfg.Port); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[SHUTDOWN] Shutting down...")
}
