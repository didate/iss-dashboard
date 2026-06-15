package usage

import (
	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

// ComputeCommodites aggregates WASH and energy indicators.
func ComputeCommodites(events []*models.Event, ctx *quality.QualityContext) []models.UsageCommodite {
	type indicator struct {
		uid  string
		name string
	}
	indicators := []indicator{
		{uid: "G6aAGwMfuOH", name: "energie"},
		{uid: "mr2SQNgReyd", name: "eau_pts_critiques"},
		{uid: "Z5G3epiH9hh", name: "solaire"},
	}

	type counter struct {
		oui, total int
	}
	accum := make(map[string]*counter) // name|district → counter
	ensure := func(name, district string) *counter {
		k := name + "|" + district
		if accum[k] == nil {
			accum[k] = &counter{}
		}
		return accum[k]
	}

	for _, evt := range events {
		for _, ind := range indicators {
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
	}

	var result []models.UsageCommodite
	for key, c := range accum {
		parts := splitFirst(key, '|')
		name, district := parts[0], parts[1]
		pct := 0.0
		if c.total > 0 {
			pct = float64(c.oui) / float64(c.total) * 100
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
