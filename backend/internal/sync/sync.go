package sync

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"encoding/json"

	"iss-dashboard-backend/internal/config"
	"iss-dashboard-backend/internal/dhis2"
	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
	"iss-dashboard-backend/internal/store"
	"iss-dashboard-backend/internal/typologie"
	"iss-dashboard-backend/internal/usage"
)

// Options carries the instance-specific settings the pipeline needs (population
// data elements, org unit group set names). Zero value = no population, default set names.
type Options struct {
	Population        []dhis2.PopulationSpec
	PopulationLevels  []int
	PopulationFactor  float64
	TypologyGroupSet  string
	HospitalGroupSet  string
	OwnershipGroupSet string
}

// OptionsFromConfig maps the loaded configuration to sync options.
func OptionsFromConfig(cfg *config.Config) Options {
	opts := Options{
		PopulationLevels:  cfg.PopulationLevels,
		PopulationFactor:  cfg.PopulationFactor,
		TypologyGroupSet:  cfg.TypologyGroupSet,
		HospitalGroupSet:  cfg.HospitalGroupSet,
		OwnershipGroupSet: cfg.OwnershipGroupSet,
	}
	for _, sp := range cfg.PopulationDX {
		opts.Population = append(opts.Population, dhis2.PopulationSpec{Indicator: sp.Indicator, UIDs: sp.UIDs})
	}
	return opts
}

var running int32

// IsRunning returns true if a sync is currently in progress.
func IsRunning() bool {
	return atomic.LoadInt32(&running) == 1
}

// RunSync executes the full sync pipeline:
// 1. Pull metadata from DHIS2
// 2. Pull events (paginated)
// 3. Enrich events with org unit hierarchy (district/region/sous-préfecture), GPS and type
// 4. Run quality rules
// 5. Compute usage aggregates
// 6. Persist everything atomically
func RunSync(st *store.Store, client *dhis2.Client, opts Options) (*models.SyncRun, error) {
	if !atomic.CompareAndSwapInt32(&running, 0, 1) {
		return nil, fmt.Errorf("sync already in progress")
	}
	defer atomic.StoreInt32(&running, 0)

	start := time.Now()

	syncRunID, err := st.CreateSyncRun()
	if err != nil {
		return nil, fmt.Errorf("create sync run: %w", err)
	}

	finishErr := func(errMsg string) (*models.SyncRun, error) {
		duration := time.Since(start).Milliseconds()
		st.FinishSyncRun(syncRunID, "error", 0, 0, duration, errMsg)
		sr, _ := st.GetLastSyncRun()
		return sr, fmt.Errorf("%s", errMsg)
	}

	// Step 1: Pull metadata
	log.Println("[SYNC] Pulling metadata...")

	dataElements, err := client.FetchDataElements()
	if err != nil {
		return finishErr(fmt.Sprintf("fetch data elements: %v", err))
	}
	log.Printf("[SYNC] Got %d data elements", len(dataElements))
	if err := st.UpsertMetadataDE(dataElements); err != nil {
		return finishErr(fmt.Sprintf("upsert metadata: %v", err))
	}

	optionEntries, err := client.FetchOptionSets()
	if err != nil {
		return finishErr(fmt.Sprintf("fetch option sets: %v", err))
	}
	log.Printf("[SYNC] Got %d option entries", len(optionEntries))
	if err := st.UpsertOptionSets(optionEntries); err != nil {
		return finishErr(fmt.Sprintf("upsert option sets: %v", err))
	}

	orgUnits, err := client.FetchOrgUnits()
	if err != nil {
		return finishErr(fmt.Sprintf("fetch org units: %v", err))
	}
	log.Printf("[SYNC] Got %d org units", len(orgUnits))
	if err := st.UpsertOrgUnits(orgUnits); err != nil {
		return finishErr(fmt.Sprintf("upsert org units: %v", err))
	}

	programOrgUnits, err := client.FetchProgramOrgUnits()
	if err != nil {
		log.Printf("[SYNC] WARN: fetch program org units: %v (reporting rate will be incomplete)", err)
	}
	log.Printf("[SYNC] Program has %d assigned org units", len(programOrgUnits))

	// Org unit groups → typology and ownership. Failure is not fatal: types fall
	// back to name prefixes and R15/R16 will flag it.
	var typoIndex *typologie.Index
	if groups, err := client.FetchOrgUnitGroups(); err != nil {
		log.Printf("[SYNC] WARN: fetch org unit groups: %v (typologie déduite des noms uniquement)", err)
	} else {
		if err := st.UpsertOrgUnitGroups(groups); err != nil {
			return finishErr(fmt.Sprintf("upsert org unit groups: %v", err))
		}
		typoIndex = typologie.NewIndex(groups, opts.TypologyGroupSet, opts.HospitalGroupSet, opts.OwnershipGroupSet)
	}

	// Population (analytics). Not fatal either: ratios stay NULL without it.
	var population []models.PopulationRow
	if len(opts.Population) == 0 {
		log.Println("[SYNC] Population: DHIS2_POPULATION_DX non configuré, ratios démographiques désactivés")
	} else {
		rows, err := client.FetchPopulation(opts.Population, opts.PopulationLevels, rootOrgUnit(orgUnits), opts.PopulationFactor)
		if err != nil {
			log.Printf("[SYNC] WARN: fetch population: %v (ratios démographiques indisponibles)", err)
		} else {
			population = rows
			log.Printf("[SYNC] Population: %d valeurs (org unit × indicateur)", len(population))
		}
	}

	// Step 2: Pull events
	log.Println("[SYNC] Pulling events...")
	events, err := client.FetchAllEvents()
	if err != nil {
		return finishErr(fmt.Sprintf("fetch events: %v", err))
	}
	log.Printf("[SYNC] Got %d events", len(events))

	// Step 3: Enrich events with district/region/sous-préfecture, GPS and type
	orgMap := buildOrgUnitMap(orgUnits)
	nGPS, typeSources := 0, map[string]int{}
	for i := range events {
		enrichEvent(&events[i], orgMap, typoIndex)
		if events[i].HasGPS() {
			nGPS++
		}
		typeSources[events[i].TypeSource]++
	}
	log.Printf("[SYNC] Geo: %d/%d events avec GPS ; typologie par source : %v", nGPS, len(events), typeSources)

	// Build event pointers for quality context
	eventPtrs := make([]*models.Event, len(events))
	for i := range events {
		eventPtrs[i] = &events[i]
	}

	// Step 4: Build quality context and run rules
	log.Println("[SYNC] Running quality rules...")
	metadata, _ := st.GetAllMetadataDE()
	options, _ := st.GetAllOptionEntries()
	ctx := quality.BuildContext(metadata, options, eventPtrs, orgUnits)
	ctx.Typologie = typoIndex

	log.Printf("[SYNC] Discovered %d equipment pairs", len(ctx.EquipPairs))

	var allIssues []store.IssueWithEvent
	var eventQualities []models.EventQuality
	totalIssues := 0

	for _, evt := range eventPtrs {
		issues := quality.RunAll(evt, ctx)
		for _, iss := range issues {
			allIssues = append(allIssues, store.IssueWithEvent{
				EventUID: evt.EventUID,
				Issue:    iss,
			})
		}
		eq := quality.ComputeScore(issues)
		eq.EventUID = evt.EventUID
		eventQualities = append(eventQualities, eq)
		totalIssues += len(issues)
	}
	log.Printf("[SYNC] Found %d issues across %d events", totalIssues, len(events))

	// Step 5: Compute quality summaries
	qualitySummaries := computeQualitySummaries(eventPtrs, eventQualities, ctx)

	// Step 6: Compute usage aggregates
	log.Println("[SYNC] Computing usage aggregates...")
	usageRecensement := usage.ComputeRecensement(eventPtrs, ctx)
	usageServices := usage.ComputeServices(eventPtrs, ctx)
	usageEquipements := usage.ComputeEquipements(eventPtrs, ctx)
	usageRH := usage.ComputeRH(eventPtrs, ctx)
	usageCommodites := usage.ComputeCommodites(eventPtrs, ctx)
	reportingRates := usage.ComputeReportingRate(eventPtrs, programOrgUnits, orgUnits)
	popIndex := usage.BuildPopulationIndex(population)
	usageGeo := usage.ComputeGeo(eventPtrs, orgUnits, eventQualities, popIndex)
	usageCouverture := usage.ComputeCouverture(eventPtrs, orgUnits, ctx, popIndex)
	blobs, err := usage.BuildPublicSnapshot(eventPtrs, ctx)
	if err != nil {
		return finishErr(fmt.Sprintf("build public snapshot: %v", err))
	}
	log.Printf("[SYNC] Public snapshot: %d octets de GeoJSON", len(blobs[usage.BlobPublicPoints]))

	// Step 7: Persist atomically
	log.Println("[SYNC] Persisting data...")
	data := &store.SyncData{
		Events:           events,
		Issues:           allIssues,
		EventQualities:   eventQualities,
		QualitySummaries: qualitySummaries,
		UsageRecensement: usageRecensement,
		UsageServices:    usageServices,
		UsageEquipements: usageEquipements,
		UsageRH:          usageRH,
		UsageCommodites:  usageCommodites,
		ReportingRates:   reportingRates,
		Population:       population,
		UsageGeo:         usageGeo,
		UsageCouverture:  usageCouverture,
		Blobs:            blobs,
	}

	if err := st.PersistSyncData(syncRunID, data); err != nil {
		return finishErr(fmt.Sprintf("persist data: %v", err))
	}

	duration := time.Since(start).Milliseconds()
	if err := st.FinishSyncRun(syncRunID, "success", len(events), totalIssues, duration, ""); err != nil {
		log.Printf("[SYNC] WARN: finish sync run: %v", err)
	}

	log.Printf("[SYNC] Completed in %dms: %d events, %d issues", duration, len(events), totalIssues)
	sr, _ := st.GetLastSyncRun()
	return sr, nil
}

type orgNode struct {
	uid        string
	name       string
	level      int
	parentUID  string
	parentName string
	geometry   string
}

func buildOrgUnitMap(units []models.OrgUnit) map[string]orgNode {
	m := make(map[string]orgNode, len(units))
	for _, ou := range units {
		m[ou.UID] = orgNode{
			uid:        ou.UID,
			name:       ou.Name,
			level:      ou.Level,
			parentUID:  ou.ParentUID,
			parentName: ou.ParentName,
			geometry:   ou.Geometry,
		}
	}
	return m
}

// rootOrgUnit returns the UID of the level-1 org unit (scopes analytics LEVEL-n queries).
func rootOrgUnit(units []models.OrgUnit) string {
	for _, ou := range units {
		if ou.Level == 1 {
			return ou.UID
		}
	}
	return ""
}

// pointCoordinates extracts lat/lng from a GeoJSON Point geometry ([lng, lat] order).
func pointCoordinates(geometry string) (lat, lng *float64) {
	if geometry == "" {
		return nil, nil
	}
	var g struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(geometry), &g); err != nil || g.Type != "Point" || len(g.Coordinates) < 2 {
		return nil, nil
	}
	x, y := g.Coordinates[0], g.Coordinates[1]
	if x < -180 || x > 180 || y < -90 || y > 90 || (x == 0 && y == 0) {
		return nil, nil
	}
	return &y, &x
}

// enrichEvent sets district, region and sous-préfecture by walking up the org
// unit hierarchy, then copies the structure's GPS point and resolves its type.
// DHIS2 Guinea hierarchy: 1 = national, 2 = région, 3 = préfecture (district),
// 4 = sous-préfecture, 5-6 = structures.
func enrichEvent(evt *models.Event, orgMap map[string]orgNode, typo *typologie.Index) {
	// Walk up from the event's org unit to find district (level 3) and region (level 2)
	current, ok := orgMap[evt.OrgUnitUID]
	if !ok {
		return
	}
	evt.Lat, evt.Lng = pointCoordinates(current.geometry)
	res := typo.Resolve(models.OrgUnit{UID: current.uid, Name: current.name})
	evt.TypeCode, evt.TypeSource = res.Code, res.Source

	// Collect ancestors
	ancestors := []orgNode{current}
	visited := map[string]bool{current.uid: true}
	for current.parentUID != "" && !visited[current.parentUID] {
		parent, ok := orgMap[current.parentUID]
		if !ok {
			break
		}
		ancestors = append(ancestors, parent)
		visited[parent.uid] = true
		current = parent
	}

	for _, a := range ancestors {
		switch a.level {
		case 2:
			evt.Region = a.name
		case 3:
			evt.District = a.name
			evt.DistrictUID = a.uid
		case 4:
			evt.SousPrefecture = a.name
			evt.SousPrefectureUID = a.uid
		}
	}
}

func computeQualitySummaries(events []*models.Event, qualities []models.EventQuality, ctx *quality.QualityContext) []models.QualitySummaryRow {
	type accum struct {
		sumScore    float64
		nError      int
		nWarning    int
		nInfo       int
		nStructures int
	}

	dims := map[string]map[string]*accum{
		"global":   {},
		"district": {},
		"region":   {},
		"statut":   {},
		"type":     {},
	}

	ensure := func(dim, key string) *accum {
		if dims[dim][key] == nil {
			dims[dim][key] = &accum{}
		}
		return dims[dim][key]
	}

	eqMap := make(map[string]models.EventQuality)
	for _, eq := range qualities {
		eqMap[eq.EventUID] = eq
	}

	statutUID := ""
	if uid, ok := ctx.CodeToUID["ISS_STATUT_STRUCT_DE"]; ok {
		statutUID = uid
	}

	for _, evt := range events {
		eq := eqMap[evt.EventUID]

		add := func(a *accum) {
			a.sumScore += float64(eq.Score)
			a.nError += eq.NError
			a.nWarning += eq.NWarning
			a.nInfo += eq.NInfo
			a.nStructures++
		}

		add(ensure("global", "all"))

		if evt.District != "" {
			add(ensure("district", evt.District))
		}
		if evt.Region != "" {
			add(ensure("region", evt.Region))
		}

		if statutUID != "" {
			statut := quality.GetEventValue(evt, statutUID)
			if statut != "" {
				add(ensure("statut", statut))
			}
		}
		if evt.TypeCode != "" {
			add(ensure("type", evt.TypeCode))
		}
	}

	var result []models.QualitySummaryRow
	for dim, m := range dims {
		for key, a := range m {
			avg := 0.0
			if a.nStructures > 0 {
				avg = a.sumScore / float64(a.nStructures)
			}
			label := key
			if dim == "type" {
				label = typologie.Label(key)
			}
			result = append(result, models.QualitySummaryRow{
				Dimension:   dim,
				Key:         key,
				Label:       label,
				AvgScore:    avg,
				NError:      a.nError,
				NWarning:    a.nWarning,
				NInfo:       a.nInfo,
				NStructures: a.nStructures,
			})
		}
	}
	return result
}
