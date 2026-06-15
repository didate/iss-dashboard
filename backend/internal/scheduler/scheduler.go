package scheduler

import (
	"log"

	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/store"
	syncer "iss-dashboard-backend/internal/sync"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron   *cron.Cron
	store  *store.Store
	client *dhis2.Client
}

func New(st *store.Store, client *dhis2.Client) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		store:  st,
		client: client,
	}
}

func (s *Scheduler) Start(cronExpr string) error {
	_, err := s.cron.AddFunc(cronExpr, func() {
		log.Println("[SCHEDULER] Starting scheduled sync...")
		sr, err := syncer.RunSync(s.store, s.client)
		if err != nil {
			log.Printf("[SCHEDULER] Sync failed: %v", err)
			return
		}
		log.Printf("[SCHEDULER] Sync completed: %d events, %d issues", sr.EventsPulled, sr.IssuesFound)
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Printf("[SCHEDULER] Started with cron: %s", cronExpr)
	return nil
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
