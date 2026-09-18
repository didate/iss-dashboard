package sync

import (
	"fmt"
	"log"
	"time"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/normes"
	"iss-dashboard-backend/internal/quality"
	"iss-dashboard-backend/internal/store"
	"iss-dashboard-backend/internal/usage"
)

// RecomputeConformite evaluates every structure (latest event per org unit)
// against the active norms referential and replaces the conformite_* tables.
// It reads everything from SQLite — no DHIS2 call — so it can run after an
// admin edits or activates a referential, and at the end of RunSync.
// With no active set (or an empty one) the tables are simply emptied.
func RecomputeConformite(st *store.Store) error {
	start := time.Now()
	set, err := st.GetActiveNormeSet()
	if err != nil {
		return fmt.Errorf("active norme set: %w", err)
	}
	data := &store.ConformiteData{}
	if set == nil {
		log.Println("[NORMES] Aucun référentiel actif : conformité vidée")
		return st.PersistConformite(data)
	}
	rules, err := st.GetNormeRules(set.ID)
	if err != nil {
		return fmt.Errorf("norme rules: %w", err)
	}
	data.SetID, data.SetVersion, data.NRules = set.ID, set.Version, len(rules)

	events, err := st.LoadEvents()
	if err != nil {
		return fmt.Errorf("load events: %w", err)
	}
	metadata, _ := st.GetAllMetadataDE()
	options, _ := st.GetAllOptionEntries()
	ptrs := make([]*models.Event, len(events))
	for i := range events {
		ptrs[i] = &events[i]
	}
	ctx := quality.BuildContext(metadata, options, ptrs, nil)
	ev := normes.NewEvaluator(rules, ctx)

	for _, e := range usage.LatestPerOrgUnit(ptrs) {
		items := ev.Evaluate(e)
		data.Results = append(data.Results, normes.EventResult{Event: e, Items: items, Summary: normes.Summarize(items)})
	}
	data.Summaries, data.Gaps = normes.Aggregate(data.Results)

	if err := st.PersistConformite(data); err != nil {
		return fmt.Errorf("persist conformite: %w", err)
	}
	nEval, nConf := 0, 0
	for _, r := range data.Results {
		if r.Summary.Score != nil {
			nEval++
		}
		if r.Summary.Conforme {
			nConf++
		}
	}
	log.Printf("[NORMES] Conformité : référentiel v%d (%d règles), %d structures, %d évaluées, %d conformes, %d écarts — %dms",
		set.Version, len(rules), len(data.Results), nEval, nConf, len(data.Gaps), time.Since(start).Milliseconds())
	return nil
}
