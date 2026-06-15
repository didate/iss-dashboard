package usage

import (
	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

// ComputeCommodites aggregates WASH and energy indicators.
func ComputeCommodites(events []*models.Event, ctx *quality.QualityContext) []models.UsageCommodite {
	// Boolean indicators (has/doesn't have)
	type boolIndicator struct {
		uid  string
		name string
	}
	boolIndicators := []boolIndicator{
		{uid: "G6aAGwMfuOH", name: "energie"},
		{uid: "mr2SQNgReyd", name: "eau_pts_critiques"},
		{uid: "Z5G3epiH9hh", name: "energie_solaire"},
		{uid: "ZKd23M3NVu0", name: "energie_reseau"},
		{uid: "Ff1uAvJbxXm", name: "energie_generateur"},
	}

	type counter struct {
		oui, total int
	}
	accum := make(map[string]*counter)
	ensure := func(name, district string) *counter {
		k := name + "|" + district
		if accum[k] == nil {
			accum[k] = &counter{}
		}
		return accum[k]
	}

	for _, evt := range events {
		for _, ind := range boolIndicators {
			v := quality.GetEventValue(evt, ind.uid)
			inc := func(c *counter) {
				c.total++
				if quality.IsTruthy(v) {
					c.oui++
				}
			}
			inc(ensure(ind.name, "all"))
			if evt.District != "" {
				inc(ensure(ind.name, evt.District))
			}
		}

		// Source d'eau (optionSet: réseau, puit, FMH, FEM, aucune)
		sourceEau := quality.GetEventValue(evt, "IzfXJ0Zrfxh")
		if sourceEau != "" {
			key := "source_eau_" + sourceEau
			c := ensure(key, "all")
			c.oui++
			c.total++ // for source_eau_* rows, n_oui = count of this type, n_total = count of this type
			if evt.District != "" {
				c2 := ensure(key, evt.District)
				c2.oui++
				c2.total++
			}
		}
		// Also count total structures for source_eau denominator
		ensure("source_eau_total", "all").total++
		if evt.District != "" {
			ensure("source_eau_total", evt.District).total++
		}
	}

	var result []models.UsageCommodite
	for key, c := range accum {
		parts := splitFirst(key, '|')
		name, district := parts[0], parts[1]
		pct := 0.0
		if c.total > 0 {
			pct = float64(c.oui) / float64(c.total) * 100
		}
		// For source_eau_* types, compute pct against total structures
		if len(name) > 11 && name[:11] == "source_eau_" && name != "source_eau_total" {
			totalKey := "source_eau_total" + "|" + district
			if tot, ok := accum[totalKey]; ok && tot.total > 0 {
				pct = float64(c.oui) / float64(tot.total) * 100
			}
		}
		result = append(result, models.UsageCommodite{
			Indicator: name,
			District:  district,
			NOui:      c.oui,
			NTotal:    c.total,
			Pct:       pct,
		})
	}
	return result
}
