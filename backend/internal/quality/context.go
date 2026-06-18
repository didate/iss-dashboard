package quality

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"iss-dashboard-backend/internal/models"
)

// QualityContext carries metadata and precomputed stats needed by rules.
type QualityContext struct {
	// Metadata
	MetadataByUID map[string]models.DataElementMeta
	CodeToUID     map[string]string
	UIDToCode     map[string]string

	// Equipment pairs discovered dynamically
	EquipPairs []models.EquipPair

	// Option sets: option_set_id → []OptionEntry
	OptionsBySet map[string][]models.OptionEntry

	// Outlier stats: de_uid → MedianStat
	Medians map[string]MedianStat

	// Duplicate detection: org_unit_uid → count of events
	OrgUnitCounts map[string]int

	// Duplicate per year: "org_unit_uid|year" → count
	OrgUnitYearCounts map[string]int

	// Closed org units: org_unit_uid → closedDate
	OrgUnitClosedDate map[string]string

	// Service-support specs for R3
	ServiceSpecs []ServiceSupportSpec

	// All events (for global rules)
	AllEvents []*models.Event
}

type MedianStat struct {
	Median float64
	MAD    float64
}

// ServiceSupportSpec defines a service field and its required support fields.
type ServiceSupportSpec struct {
	ServiceUID    string
	SupportUIDs   []string
	Message       string
}

// BuildContext constructs a QualityContext from metadata, events and org units.
func BuildContext(
	metadata []models.DataElementMeta,
	options []models.OptionEntry,
	events []*models.Event,
	orgUnits []models.OrgUnit,
) *QualityContext {
	ctx := &QualityContext{
		MetadataByUID:     make(map[string]models.DataElementMeta),
		CodeToUID:         make(map[string]string),
		UIDToCode:         make(map[string]string),
		OptionsBySet:      make(map[string][]models.OptionEntry),
		Medians:           make(map[string]MedianStat),
		OrgUnitCounts:     make(map[string]int),
		OrgUnitYearCounts: make(map[string]int),
		OrgUnitClosedDate: make(map[string]string),
		AllEvents:         events,
	}

	// Build closed date map
	for _, ou := range orgUnits {
		if ou.ClosedDate != "" {
			ctx.OrgUnitClosedDate[ou.UID] = ou.ClosedDate
		}
	}

	for _, de := range metadata {
		ctx.MetadataByUID[de.UID] = de
		if de.Code != "" {
			ctx.CodeToUID[de.Code] = de.UID
			ctx.UIDToCode[de.UID] = de.Code
		}
	}

	for _, o := range options {
		ctx.OptionsBySet[o.OptionSetID] = append(ctx.OptionsBySet[o.OptionSetID], o)
	}

	ctx.EquipPairs = discoverEquipPairs(metadata, ctx.CodeToUID)
	ctx.ServiceSpecs = buildServiceSpecs(ctx)
	ctx.OrgUnitCounts = countOrgUnits(events)
	ctx.OrgUnitYearCounts = countOrgUnitsByYear(events)
	ctx.Medians = computeMedians(events, ctx)

	return ctx
}

// discoverEquipPairs dynamically pairs _TOTAL_DE/_FONC_DE data elements.
func discoverEquipPairs(metadata []models.DataElementMeta, codeToUID map[string]string) []models.EquipPair {
	totalCodes := make(map[string]models.DataElementMeta) // root → DE
	foncCodes := make(map[string]models.DataElementMeta)

	for _, de := range metadata {
		code := de.Code
		if code == "" {
			continue
		}
		if strings.HasSuffix(code, "_TOTAL_DE") {
			root := strings.TrimSuffix(code, "_TOTAL_DE")
			totalCodes[root] = de
		} else if strings.HasSuffix(code, "_TOTAL") && !strings.HasSuffix(code, "_DE") {
			root := strings.TrimSuffix(code, "_TOTAL")
			totalCodes[root] = de
		} else if strings.HasSuffix(code, "_FONC_DE") {
			root := strings.TrimSuffix(code, "_FONC_DE")
			foncCodes[root] = de
		} else if strings.HasSuffix(code, "_FONC") && !strings.HasSuffix(code, "_DE") {
			root := strings.TrimSuffix(code, "_FONC")
			foncCodes[root] = de
		}
		// Special: code is a total without suffix (like ISS_NB_PESE_BEBE)
		if de.SectionPrefix == "ISS_EQ" && !strings.Contains(code, "FONC") && !strings.Contains(code, "TOTAL") {
			totalCodes[code] = de
		}
	}

	var pairs []models.EquipPair
	matched := make(map[string]bool)

	for root, totalDE := range totalCodes {
		// Try exact match first
		if foncDE, ok := foncCodes[root]; ok {
			pairs = append(pairs, models.EquipPair{
				Root:     root,
				TotalUID: totalDE.UID,
				FoncUID:  foncDE.UID,
				Label:    cleanLabel(totalDE.DisplayName()),
			})
			matched[root] = true
			continue
		}

		// Fuzzy match: look for fonc codes that share most of the root
		for fRoot, foncDE := range foncCodes {
			if matched[fRoot] {
				continue
			}
			// Normalize both roots and check similarity
			normTotal := normalizeRoot(root)
			normFonc := normalizeRoot(fRoot)
			if normTotal == normFonc || strings.Contains(normFonc, normTotal) || strings.Contains(normTotal, normFonc) {
				pairs = append(pairs, models.EquipPair{
					Root:     root,
					TotalUID: totalDE.UID,
					FoncUID:  foncDE.UID,
					Label:    cleanLabel(totalDE.DisplayName()),
				})
				matched[fRoot] = true
				matched[root] = true
				break
			}
		}
	}

	return pairs
}

func normalizeRoot(root string) string {
	r := strings.ReplaceAll(root, "__", "_")
	r = strings.TrimPrefix(r, "ISS_EQUI_")
	r = strings.TrimPrefix(r, "ISS_NB_")
	r = strings.TrimSuffix(r, "_OP")
	r = strings.TrimSuffix(r, "_DE")
	return strings.ToLower(r)
}

func cleanLabel(name string) string {
	// Remove "total (carton inclus)" and ISS_ prefix
	name = strings.TrimPrefix(name, "ISS_EQ ")
	for _, suf := range []string{" total (carton inclus)", " total", " fonctionnel (en service)"} {
		name = strings.TrimSuffix(name, suf)
	}
	if idx := strings.Index(name, " total"); idx > 0 {
		name = name[:idx]
	}
	return name
}

func buildServiceSpecs(ctx *QualityContext) []ServiceSupportSpec {
	laboUID := "Zq34u53MgeI"
	microscopeUID := "bWGkmx4RfoE"    // microscope fonctionnel
	laboInfraUID := ctx.CodeToUID["ISS_INFRA_LABO_DE"]

	specs := []ServiceSupportSpec{
		{
			ServiceUID:  laboUID,
			SupportUIDs: []string{microscopeUID, laboInfraUID},
			Message:     "Service laboratoire déclaré fonctionnel mais aucun microscope fonctionnel et aucune salle de labo",
		},
	}

	// Maternité sans table d'accouchement
	materniteUID := ctx.CodeToUID["ISS_SVC_MATERNITE_DE"]
	tableAccFoncUID := ctx.CodeToUID["ISS_EQUI_TABLE_ACC_FONC_DE"]
	if materniteUID != "" && tableAccFoncUID != "" {
		specs = append(specs, ServiceSupportSpec{
			ServiceUID:  materniteUID,
			SupportUIDs: []string{tableAccFoncUID},
			Message:     "Maternité déclarée fonctionnelle mais aucune table d'accouchement fonctionnelle",
		})
	}

	// Chirurgie sans table opératoire
	chirUID := ctx.CodeToUID["ISS_SVC_CHIRURGIE_DE"]
	tableOpFoncUID := ctx.CodeToUID["ISS_EQUI_TABLE_OP_FONC_DE"]
	if chirUID != "" && tableOpFoncUID != "" {
		specs = append(specs, ServiceSupportSpec{
			ServiceUID:  chirUID,
			SupportUIDs: []string{tableOpFoncUID},
			Message:     "Chirurgie déclarée fonctionnelle mais aucune table opératoire fonctionnelle",
		})
	}

	return specs
}

func countOrgUnits(events []*models.Event) map[string]int {
	counts := make(map[string]int)
	for _, e := range events {
		counts[e.OrgUnitUID]++
	}
	return counts
}

func countOrgUnitsByYear(events []*models.Event) map[string]int {
	counts := make(map[string]int)
	for _, e := range events {
		year := ""
		if len(e.EventDate) >= 4 {
			year = e.EventDate[:4]
		}
		key := e.OrgUnitUID + "|" + year
		counts[key]++
	}
	return counts
}

func computeMedians(events []*models.Event, ctx *QualityContext) map[string]MedianStat {
	// Collect numeric values per equipment total DE
	valuesMap := make(map[string][]float64)
	for _, pair := range ctx.EquipPairs {
		valuesMap[pair.TotalUID] = nil
	}

	for _, evt := range events {
		vals := evt.Values()
		for uid := range valuesMap {
			if v, ok := vals[uid]; ok && v != "" {
				f, err := strconv.ParseFloat(v, 64)
				if err == nil && f >= 0 {
					valuesMap[uid] = append(valuesMap[uid], f)
				}
			}
		}
	}

	medians := make(map[string]MedianStat)
	for uid, values := range valuesMap {
		if len(values) < 5 {
			continue
		}
		sort.Float64s(values)
		med := median(values)
		mad := computeMAD(values, med)
		medians[uid] = MedianStat{Median: med, MAD: mad}
	}
	return medians
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func computeMAD(sorted []float64, med float64) float64 {
	deviations := make([]float64, len(sorted))
	for i, v := range sorted {
		deviations[i] = math.Abs(v - med)
	}
	sort.Float64s(deviations)
	return median(deviations)
}

// GetEventValue gets a value from an event by DE UID.
func GetEventValue(evt *models.Event, uid string) string {
	for _, dv := range evt.DataValues {
		if dv.DataElement == uid {
			return dv.Value
		}
	}
	return ""
}

// GetEventValueByCode gets a value by DE code.
func GetEventValueByCode(evt *models.Event, code string, ctx *QualityContext) string {
	uid, ok := ctx.CodeToUID[code]
	if !ok {
		return ""
	}
	return GetEventValue(evt, uid)
}

// ParseNum parses a string to float64, returns 0 if invalid.
func ParseNum(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// IsTruthy checks if a DHIS2 boolean value is true.
func IsTruthy(v string) bool {
	return v == "true" || v == "1" || v == "oui"
}
