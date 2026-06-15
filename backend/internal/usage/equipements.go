package usage

import (
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

// ComputeEquipements aggregates equipment functionality rates.
func ComputeEquipements(events []*models.Event, ctx *quality.QualityContext) []models.UsageEquipement {
	type counter struct {
		sumTotal, sumFonct int
	}
	accum := make(map[string]*counter) // root|district → counter
	ensure := func(root, district string) *counter {
		k := root + "|" + district
		if accum[k] == nil {
			accum[k] = &counter{}
		}
		return accum[k]
	}

	for _, evt := range events {
		vals := evt.Values()
		for _, pair := range ctx.EquipPairs {
			t := quality.ParseNum(vals[pair.TotalUID])
			f := quality.ParseNum(vals[pair.FoncUID])

			if t > 0 || f > 0 {
				c := ensure(pair.Root, "all")
				c.sumTotal += int(t)
				c.sumFonct += int(f)

				if evt.District != "" {
					c2 := ensure(pair.Root, evt.District)
					c2.sumTotal += int(t)
					c2.sumFonct += int(f)
				}
			}
		}
	}

	// Build label + category maps
	labelMap := make(map[string]string)
	for _, pair := range ctx.EquipPairs {
		labelMap[pair.Root] = pair.Label
	}

	var result []models.UsageEquipement
	for key, c := range accum {
		parts := splitFirst(key, '|')
		root, district := parts[0], parts[1]
		pct := 0.0
		if c.sumTotal > 0 {
			pct = float64(c.sumFonct) / float64(c.sumTotal) * 100
		}
		result = append(result, models.UsageEquipement{
			EquipRoot: root,
			Label:     labelMap[root],
			District:  district,
			SumTotal:  c.sumTotal,
			SumFonct:  c.sumFonct,
			PctFonct:  pct,
			Category:  classifyEquipment(root),
		})
	}
	return result
}

func classifyEquipment(root string) string {
	r := strings.ToLower(root)
	switch {
	case strings.Contains(r, "frigo") || strings.Contains(r, "congelateur") || strings.Contains(r, "porte_vaccin") || strings.Contains(r, "glaciere"):
		return "chaine_froid"
	case strings.Contains(r, "echo") || strings.Contains(r, "radio") || strings.Contains(r, "scanner") || strings.Contains(r, "irm"):
		return "imagerie"
	case strings.Contains(r, "ambulance") || strings.Contains(r, "moto") || strings.Contains(r, "voiture") || strings.Contains(r, "tricycle"):
		return "transport"
	case strings.Contains(r, "lit") || strings.Contains(r, "hosp"):
		return "hospitalisation"
	case strings.Contains(r, "microscope"):
		return "laboratoire"
	default:
		return "autre"
	}
}
