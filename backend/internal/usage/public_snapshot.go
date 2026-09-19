package usage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
	"iss-dashboard-backend/internal/typologie"
)

// Snapshot blob keys served verbatim by the public API.
const (
	BlobPublicPoints  = "public_points_geojson"
	BlobPublicFilters = "public_filters"
)

// serviceOptionSet is the option set shared by every ISS service field (oui / oui_pas_fonctionnel / non).
const serviceOptionSet = "RGsTov6dBHH"

// ServiceDE describes one service data element usable in the public projection.
type ServiceDE struct {
	UID   string
	Code  string // full DE code, e.g. ISS_SVC_MATERNITE_DE
	Key   string // short public key, e.g. MATERNITE
	Label string
}

// ServiceKey shortens a service DE code for public payloads: ISS_SVC_MATERNITE_DE → MATERNITE.
func ServiceKey(code string) string {
	return strings.TrimSuffix(strings.TrimPrefix(code, "ISS_SVC_"), "_DE")
}

// ServiceLabel strips the section prefix DHIS2 puts in service names.
func ServiceLabel(de models.DataElementMeta) string {
	l := de.DisplayName()
	for _, p := range []string{"ISS_SVC ", "ISS_LAB "} {
		l = strings.TrimPrefix(l, p)
	}
	return l
}

// DiscoverServices lists service data elements sorted by label.
func DiscoverServices(ctx *quality.QualityContext) []ServiceDE {
	var out []ServiceDE
	for _, de := range ctx.MetadataByUID {
		if (de.SectionPrefix == "ISS_SVC" || de.SectionPrefix == "ISS_LAB") && de.OptionSetID == serviceOptionSet && de.Code != "" {
			out = append(out, ServiceDE{UID: de.UID, Code: de.Code, Key: ServiceKey(de.Code), Label: ServiceLabel(de)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// PublicCoreServices is the basket of « services principaux » counted in the
// public popup score (x / 7) : the primary-care minimum package.
var PublicCoreServices = []string{
	"ISS_SVC_CURATIF_DE", "ISS_SVC_CPN_DE", "ISS_SVC_ACCOUCHEMENT_DE", "ISS_SVC_PEV_DE",
	"ISS_SVC_PTME_DE", "ISS_SVC_LABO_DE", "ISS_SVC_PHARMACIE_DE",
}

// rhSoignantsPrefixes are the profile roots counted as « personnel soignant ».
var rhSoignantsPrefixes = []string{"ISS_RH_MED_", "ISS_RH_SAGEF", "ISS_RH_INF", "ISS_RH_ATS"}

// ExtrasComputer derives PublicExtras from an event; built once per sync.
type ExtrasComputer struct {
	rhDEs     []RHDataElement
	levelOf   map[string]int
	codeToUID map[string]string
}

func NewExtrasComputer(ctx *quality.QualityContext, orgUnits []models.OrgUnit) *ExtrasComputer {
	ec := &ExtrasComputer{levelOf: make(map[string]int, len(orgUnits)), codeToUID: ctx.CodeToUID}
	ec.rhDEs, _ = DiscoverRHProfiles(ctx)
	for _, ou := range orgUnits {
		ec.levelOf[ou.UID] = ou.Level
	}
	return ec
}

func (ec *ExtrasComputer) Compute(e *models.Event) models.PublicExtras {
	vals := e.Values()
	x := models.PublicExtras{Niveau: ec.levelOf[e.OrgUnitUID], ScoreServicesN: len(PublicCoreServices)}

	total, med, soign, any := 0, 0, 0, false
	for _, de := range ec.rhDEs {
		v := strings.TrimSpace(vals[de.UID])
		if v == "" {
			continue
		}
		any = true
		n := int(quality.ParseNum(v))
		total += n
		if strings.HasPrefix(de.Root, "ISS_RH_MED_") {
			med += n
		}
		for _, p := range rhSoignantsPrefixes {
			if strings.HasPrefix(de.Root, p) {
				soign += n
				break
			}
		}
	}
	if any {
		x.RhTotal, x.RhMedecins, x.RhSoignants = &total, &med, &soign
	}

	boolOf := func(code string) *bool {
		v := strings.TrimSpace(vals[ec.codeToUID[code]])
		if v == "" {
			return nil
		}
		b := quality.IsTruthy(v)
		return &b
	}
	x.Eau = boolOf("ISS_EAU_DISPO_PTS_CRITIQUES")
	x.Energie = boolOf("ISS_ENERGIE_OUI_NON_DE")

	score, answered := 0, false
	for _, code := range PublicCoreServices {
		v := vals[ec.codeToUID[code]]
		if v != "" {
			answered = true
		}
		if v == "oui" {
			score++
		}
	}
	if answered {
		x.ScoreServices = &score
	}
	return x
}

// LatestPerOrgUnit keeps the most recent event of each org unit (a structure may
// have been censused several times; the public projection shows one record).
func LatestPerOrgUnit(events []*models.Event) []*models.Event {
	latest := make(map[string]*models.Event, len(events))
	for _, e := range events {
		cur, ok := latest[e.OrgUnitUID]
		if !ok || e.EventDate > cur.EventDate || (e.EventDate == cur.EventDate && e.EventUID > cur.EventUID) {
			latest[e.OrgUnitUID] = e
		}
	}
	out := make([]*models.Event, 0, len(latest))
	for _, e := range latest {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrgUnitName < out[j].OrgUnitName })
	return out
}

// BuildPublicSnapshot pre-computes the public map payloads: the GeoJSON of every
// geolocated structure with reduced properties, and the filter lists. Returned
// as serialized JSON keyed by blob name, ready to be stored and served with an ETag.
func BuildPublicSnapshot(events []*models.Event, ctx *quality.QualityContext, orgUnits []models.OrgUnit) (map[string][]byte, []models.PublicExtrasRow, error) {
	services := DiscoverServices(ctx)
	extras := NewExtrasComputer(ctx, orgUnits)
	var extraRows []models.PublicExtrasRow
	structUID := ctx.CodeToUID["ISS_STATUT_STRUCT_DE"]
	opUID := ctx.CodeToUID["ISS_STATUT_OP_DE"]

	latest := LatestPerOrgUnit(events)
	typeCounts := map[string]int{}
	regions := map[string]bool{}
	districts := map[string]string{}
	features := make([]models.PublicPointFeature, 0, len(latest))

	for _, e := range latest {
		code := e.TypeCode
		if code == "" {
			code = typologie.Indetermine
		}
		typeCounts[code]++
		if e.Region != "" {
			regions[e.Region] = true
		}
		if e.District != "" {
			districts[e.District] = e.Region
		}
		ex := extras.Compute(e)
		extraRows = append(extraRows, models.PublicExtrasRow{OrgUnitUID: e.OrgUnitUID, PublicExtras: ex})
		if !e.HasGPS() {
			continue
		}
		vals := e.Values()
		var svc []string
		for _, s := range services {
			if vals[s.UID] == "oui" {
				svc = append(svc, s.Key)
			}
		}
		geom, _ := json.Marshal(map[string]any{"type": "Point", "coordinates": []float64{*e.Lng, *e.Lat}})
		features = append(features, models.PublicPointFeature{
			Type:     "Feature",
			Geometry: geom,
			Properties: models.PublicPointProperties{
				UID:            e.OrgUnitUID,
				Name:           e.OrgUnitName,
				TypeCode:       code,
				TypeLabel:      typologie.Label(code),
				StatutJuri:     vals[structUID],
				StatutOp:       vals[opUID],
				Region:         e.Region,
				District:       e.District,
				SousPrefecture: e.SousPrefecture,
				Services:       svc,
				PublicExtras:   ex,
			},
		})
	}

	points, err := json.Marshal(models.PublicPointCollection{Type: "FeatureCollection", Features: features})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal public points: %w", err)
	}

	filters := models.PublicFilters{}
	for code, n := range typeCounts {
		filters.Types = append(filters.Types, models.PublicFilterType{Code: code, Label: typologie.Label(code), N: n})
	}
	sort.Slice(filters.Types, func(i, j int) bool { return filters.Types[i].N > filters.Types[j].N })
	for _, s := range services {
		filters.Services = append(filters.Services, models.PublicService{Code: s.Key, Label: s.Label})
	}
	for r := range regions {
		filters.Regions = append(filters.Regions, r)
	}
	sort.Strings(filters.Regions)
	for d, r := range districts {
		filters.Districts = append(filters.Districts, models.PublicFilterDistrict{Name: d, Region: r})
	}
	sort.Slice(filters.Districts, func(i, j int) bool { return filters.Districts[i].Name < filters.Districts[j].Name })
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal public filters: %w", err)
	}

	return map[string][]byte{BlobPublicPoints: points, BlobPublicFilters: filtersJSON}, extraRows, nil
}
