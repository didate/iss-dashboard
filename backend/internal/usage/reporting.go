package usage

import (
	"iss-dashboard-backend/internal/models"
)

// ComputeReportingRate calculates reporting completeness.
// Expected = org units assigned to the ISS program that are NOT closed.
// Reported = org units with at least one submitted event.
func ComputeReportingRate(events []*models.Event, programOrgUnits []string, orgUnits []models.OrgUnit) []models.ReportingRate {
	// Build org unit lookup
	ouMap := make(map[string]models.OrgUnit, len(orgUnits))
	for _, ou := range orgUnits {
		ouMap[ou.UID] = ou
	}

	// Build set of closed org units
	closedOUs := make(map[string]bool)
	for _, ou := range orgUnits {
		if ou.ClosedDate != "" {
			closedOUs[ou.UID] = true
		}
	}

	// Also mark as closed org units that submitted with non-operational status
	for _, evt := range events {
		for _, dv := range evt.DataValues {
			if dv.DataElement == "HpjvSNCEWM0" { // ISS_STATUT_OP_DE
				if dv.Value == "non_operationnel" || dv.Value == "ferme_temporairement" {
					closedOUs[evt.OrgUnitUID] = true
				}
			}
		}
	}

	// Resolve district/region by walking up hierarchy
	type location struct{ district, region string }
	locCache := make(map[string]location)
	resolveLocation := func(uid string) location {
		if loc, ok := locCache[uid]; ok {
			return loc
		}
		var loc location
		current, ok := ouMap[uid]
		if !ok {
			return loc
		}
		visited := map[string]bool{uid: true}
		ancestors := []models.OrgUnit{current}
		for current.ParentUID != "" && !visited[current.ParentUID] {
			parent, ok := ouMap[current.ParentUID]
			if !ok {
				break
			}
			ancestors = append(ancestors, parent)
			visited[parent.UID] = true
			current = parent
		}
		for _, a := range ancestors {
			switch a.Level {
			case 2:
				loc.region = a.Name
			case 3:
				loc.district = a.Name
			}
		}
		locCache[uid] = loc
		return loc
	}

	type counter struct {
		expected int
		reported map[string]bool
	}
	dims := map[string]map[string]*counter{
		"global":   {"all": {reported: make(map[string]bool)}},
		"district": {},
		"region":   {},
	}
	ensure := func(dim, key string) *counter {
		if dims[dim][key] == nil {
			dims[dim][key] = &counter{reported: make(map[string]bool)}
		}
		return dims[dim][key]
	}

	// Count expected from program org units (excluding closed ones)
	for _, uid := range programOrgUnits {
		if closedOUs[uid] {
			continue
		}
		loc := resolveLocation(uid)
		ensure("global", "all").expected++
		if loc.district != "" {
			ensure("district", loc.district).expected++
		}
		if loc.region != "" {
			ensure("region", loc.region).expected++
		}
	}

	// Count reported (distinct org units with events, excluding closed/non-operational)
	for _, evt := range events {
		if closedOUs[evt.OrgUnitUID] {
			continue
		}
		ensure("global", "all").reported[evt.OrgUnitUID] = true
		if evt.District != "" {
			ensure("district", evt.District).reported[evt.OrgUnitUID] = true
		}
		if evt.Region != "" {
			ensure("region", evt.Region).reported[evt.OrgUnitUID] = true
		}
	}

	var result []models.ReportingRate
	for dim, m := range dims {
		for key, c := range m {
			pct := 0.0
			if c.expected > 0 {
				pct = float64(len(c.reported)) / float64(c.expected) * 100
			}
			result = append(result, models.ReportingRate{
				Dimension: dim,
				Key:       key,
				Label:     key,
				NExpected: c.expected,
				NReported: len(c.reported),
				Pct:       pct,
			})
		}
	}
	return result
}
