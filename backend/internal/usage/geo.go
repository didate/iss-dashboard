package usage

import (
	"sort"
	"strings"

	"iss-dashboard-backend/internal/models"
)

// Indicateurs de couverture démographique, et leur libellé.
var couvertureLabels = map[string]string{
	"structures":   "Structures sanitaires",
	"lits":         "Lits d'hospitalisation",
	"medecins":     "Médecins (toutes spécialités)",
	"sages_femmes": "Sages-femmes",
	"infirmiers":   "Infirmiers",
	"ats":          "ATS",
}

// couvertureRHProfiles maps an indicator to the usage_rh profile roots it sums.
// A prefix ending in "_" matches every profile under it (ISS_RH_MED_ → tous les médecins).
var couvertureRHProfiles = map[string][]string{
	"medecins":     {"ISS_RH_MED_"},
	"sages_femmes": {"ISS_RH_SAGEF"},
	"infirmiers":   {"ISS_RH_INF"},
	"ats":          {"ISS_RH_ATS"},
}

const litsEquipRoot = "ISS_EQUI_LIT"

// PopulationIndex answers "population totale de cette org unit ?" for the geo computations.
type PopulationIndex map[string]float64 // ou uid → population totale

// BuildPopulationIndex keeps only the "total" indicator, keyed by org unit.
func BuildPopulationIndex(rows []models.PopulationRow) PopulationIndex {
	idx := make(PopulationIndex)
	for _, r := range rows {
		if r.Indicator == "total" {
			idx[r.OrgUnitUID] = r.Value
		}
	}
	return idx
}

func (p PopulationIndex) get(uid string) *float64 {
	if v, ok := p[uid]; ok && v > 0 {
		return &v
	}
	return nil
}

// ComputeGeo builds one row per administrative unit of level 3 (district) and
// 4 (sous-préfecture), including units without any structure so maps can show
// them as empty rather than missing. Structures are counted per distinct org unit.
func ComputeGeo(events []*models.Event, orgUnits []models.OrgUnit, qualities []models.EventQuality, pop PopulationIndex) []models.UsageGeo {
	scoreOf := make(map[string]int, len(qualities))
	for _, q := range qualities {
		scoreOf[q.EventUID] = q.Score
	}

	type acc struct {
		structures map[string]bool
		gps        map[string]bool
		sumScore   float64
		nScore     int
		parType    map[string]int
	}
	newAcc := func() *acc {
		return &acc{structures: map[string]bool{}, gps: map[string]bool{}, parType: map[string]int{}}
	}
	byUnit := make(map[string]*acc) // ou uid (level 3 or 4) → acc

	add := func(unitUID string, evt *models.Event) {
		if unitUID == "" {
			return
		}
		a := byUnit[unitUID]
		if a == nil {
			a = newAcc()
			byUnit[unitUID] = a
		}
		if !a.structures[evt.OrgUnitUID] {
			a.structures[evt.OrgUnitUID] = true
			if evt.HasGPS() {
				a.gps[evt.OrgUnitUID] = true
			}
			code := evt.TypeCode
			if code == "" {
				code = "INDETERMINE"
			}
			a.parType[code]++
		}
		if s, ok := scoreOf[evt.EventUID]; ok {
			a.sumScore += float64(s)
			a.nScore++
		}
	}

	for _, evt := range events {
		add(evt.DistrictUID, evt)
		add(evt.SousPrefectureUID, evt)
	}

	var out []models.UsageGeo
	for _, ou := range orgUnits {
		if ou.Level != 3 && ou.Level != 4 {
			continue
		}
		row := models.UsageGeo{
			Level:      ou.Level,
			OrgUnitUID: ou.UID,
			Name:       ou.Name,
			ParentName: ou.ParentName,
			Population: pop.get(ou.UID),
			NParType:   map[string]int{},
		}
		if a := byUnit[ou.UID]; a != nil {
			row.NStructures = len(a.structures)
			row.NGPS = len(a.gps)
			row.NParType = a.parType
			if row.NStructures > 0 {
				pct := 100 * float64(row.NGPS) / float64(row.NStructures)
				row.PctGPS = &pct
			}
			if a.nScore > 0 {
				avg := a.sumScore / float64(a.nScore)
				row.AvgScore = &avg
			}
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ComputeCouverture derives "per 10 000 inhabitants" ratios. Numerators reuse the
// already computed RH and equipment aggregates (so the figures match the RH and
// equipment tabs) ; structures are counted per distinct org unit from the events.
// Population comes straight from the org unit of each dimension key (level 1
// for global, 2 for region, 3 for district, 4 for sous-préfecture) — no summing
// across children, which would silently produce partial totals.
func ComputeCouverture(events []*models.Event, orgUnits []models.OrgUnit, usageRH []models.UsageRH, usageEquip []models.UsageEquipement, pop PopulationIndex) []models.UsageCouverture {
	// Dimension key → org unit uid, to look population up.
	uidOf := map[string]map[string]string{"global": {}, "region": {}, "district": {}, "sous_prefecture": {}}
	districtRegion := make(map[string]string) // district name → region name
	for _, ou := range orgUnits {
		switch ou.Level {
		case 1:
			uidOf["global"]["all"] = ou.UID
		case 2:
			uidOf["region"][ou.Name] = ou.UID
		case 3:
			uidOf["district"][ou.Name] = ou.UID
			districtRegion[ou.Name] = ou.ParentName
		case 4:
			uidOf["sous_prefecture"][ou.Name] = ou.UID
		}
	}

	// numerators[dimension][key][indicator]
	num := map[string]map[string]map[string]float64{}
	addNum := func(dim, key, ind string, v float64) {
		if key == "" {
			return
		}
		if num[dim] == nil {
			num[dim] = map[string]map[string]float64{}
		}
		if num[dim][key] == nil {
			num[dim][key] = map[string]float64{}
		}
		num[dim][key][ind] += v
	}

	// Structures : distinct org units per dimension.
	seen := map[string]bool{}
	for _, evt := range events {
		if seen[evt.OrgUnitUID] {
			continue
		}
		seen[evt.OrgUnitUID] = true
		addNum("global", "all", "structures", 1)
		addNum("region", evt.Region, "structures", 1)
		addNum("district", evt.District, "structures", 1)
		addNum("sous_prefecture", evt.SousPrefecture, "structures", 1)
	}

	// RH : usage_rh rows are keyed by district name or "all".
	for _, rh := range usageRH {
		for ind, prefixes := range couvertureRHProfiles {
			if !matchesProfile(rh.ProfilCode, prefixes) {
				continue
			}
			v := float64(rh.EffectifTotal)
			if rh.District == "all" {
				addNum("global", "all", ind, v)
			} else {
				addNum("district", rh.District, ind, v)
				addNum("region", districtRegion[rh.District], ind, v)
			}
		}
	}

	// Lits : usage_equipement, same keying.
	for _, eq := range usageEquip {
		if eq.EquipRoot != litsEquipRoot {
			continue
		}
		v := float64(eq.SumTotal)
		if eq.District == "all" {
			addNum("global", "all", "lits", v)
		} else {
			addNum("district", eq.District, "lits", v)
			addNum("region", districtRegion[eq.District], "lits", v)
		}
	}

	var out []models.UsageCouverture
	for dim, byKey := range num {
		for key, byInd := range byKey {
			population := pop.get(uidOf[dim][key])
			for ind, v := range byInd {
				row := models.UsageCouverture{
					Dimension:  dim,
					Key:        key,
					Label:      key,
					Indicator:  ind,
					Numerator:  v,
					Population: population,
				}
				if dim == "global" {
					row.Label = "National"
				}
				if population != nil {
					r := v / *population * 10000
					row.Ratio10k = &r
				}
				out = append(out, row)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dimension != out[j].Dimension {
			return out[i].Dimension < out[j].Dimension
		}
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].Indicator < out[j].Indicator
	})
	return out
}

// CouvertureLabel returns the display label of a coverage indicator.
func CouvertureLabel(indicator string) string {
	if l, ok := couvertureLabels[indicator]; ok {
		return l
	}
	return indicator
}

func matchesProfile(code string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasSuffix(p, "_") {
			if strings.HasPrefix(code, p) {
				return true
			}
		} else if code == p {
			return true
		}
	}
	return false
}
