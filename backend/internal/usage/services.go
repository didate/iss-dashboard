package usage

import (
	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

// ComputeServices aggregates service availability.
func ComputeServices(events []*models.Event, ctx *quality.QualityContext) []models.UsageService {
	// Find all service DEs (section ISS_SVC or ISS_LAB with optionSet RGsTov6dBHH)
	type svcDE struct {
		uid   string
		code  string
		label string
	}
	var serviceDEs []svcDE
	for _, de := range ctx.MetadataByUID {
		if (de.SectionPrefix == "ISS_SVC" || de.SectionPrefix == "ISS_LAB") && de.OptionSetID == "RGsTov6dBHH" {
			serviceDEs = append(serviceDEs, svcDE{uid: de.UID, code: de.Code, label: de.DisplayName()})
		}
	}

	type counter struct {
		oui, ouiPasFonc, non, total int
	}
	// key: serviceCode + "|" + district
	accum := make(map[string]*counter)
	getKey := func(code, district string) string { return code + "|" + district }
	ensure := func(code, district string) *counter {
		k := getKey(code, district)
		if accum[k] == nil {
			accum[k] = &counter{}
		}
		return accum[k]
	}

	for _, evt := range events {
		for _, svc := range serviceDEs {
			v := quality.GetEventValue(evt, svc.uid)
			if v == "" {
				continue
			}

			inc := func(c *counter) {
				c.total++
				switch v {
				case "oui":
					c.oui++
				case "oui_pas_fonctionnel":
					c.ouiPasFonc++
				case "non":
					c.non++
				}
			}

			inc(ensure(svc.code, "all"))
			if evt.District != "" {
				inc(ensure(svc.code, evt.District))
			}
		}
	}

	// Build label map
	labelMap := make(map[string]string)
	for _, svc := range serviceDEs {
		labelMap[svc.code] = svc.label
	}

	var result []models.UsageService
	for key, c := range accum {
		parts := splitFirst(key, '|')
		code, district := parts[0], parts[1]
		pct := 0.0
		if c.total > 0 {
			pct = float64(c.oui) / float64(c.total) * 100
		}
		result = append(result, models.UsageService{
			ServiceCode:    code,
			ServiceLabel:   labelMap[code],
			District:       district,
			NOui:           c.oui,
			NOuiPasFonc:    c.ouiPasFonc,
			NNon:           c.non,
			NTotal:         c.total,
			PctFonctionnel: pct,
		})
	}
	return result
}

func splitFirst(s string, sep byte) [2]string {
	for i := range s {
		if s[i] == sep {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}
