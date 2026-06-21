package usage

import (
	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

// ComputeRecensement aggregates structure counts by dimension.
func ComputeRecensement(events []*models.Event, ctx *quality.QualityContext) []models.UsageRecensement {
	type counter struct {
		total, oper, nonOper, ferme int
	}

	// Accumulators per dimension
	dims := map[string]map[string]*counter{
		"global":          {"all": {}},
		"district":        {},
		"region":          {},
		"statut_juridique": {},
	}

	ensure := func(dim, key string) *counter {
		m := dims[dim]
		if m[key] == nil {
			m[key] = &counter{}
		}
		return m[key]
	}

	statutUID := ctx.CodeToUID["ISS_STATUT_OP_DE"]
	structUID := ctx.CodeToUID["ISS_STATUT_STRUCT_DE"]
	pubUID := ctx.CodeToUID["ISS_STATUT_PUB_DE"]
	privUID := ctx.CodeToUID["ISS_STATUT_PRIV_DE"]

	for _, evt := range events {
		statut := quality.GetEventValue(evt, statutUID)
		isOper := statut == "operationnel"
		isNonOper := statut == "non_operationnel"
		isFerme := statut == "ferme_temporairement"

		inc := func(c *counter) {
			c.total++
			if isOper {
				c.oper++
			} else if isNonOper {
				c.nonOper++
			} else if isFerme {
				c.ferme++
			}
		}

		inc(ensure("global", "all"))

		if evt.District != "" {
			inc(ensure("district", evt.District))
		}
		if evt.Region != "" {
			inc(ensure("region", evt.Region))
		}

		// Statut de la structure (niveau 1 : publique/privée)
		statutStruct := quality.GetEventValue(evt, structUID)
		if statutStruct != "" {
			inc(ensure("statut_structure", statutStruct))
		}

		// Statut juridique (niveau 2 : sous-type)
		var juridique string
		if statutStruct == "publique" {
			juridique = quality.GetEventValue(evt, pubUID)
			if juridique == "" {
				juridique = "publique"
			}
		} else if statutStruct == "privée" {
			juridique = quality.GetEventValue(evt, privUID)
			if juridique == "" || juridique == "privélucratif" {
				juridique = "Privé"
			}
		}
		if juridique != "" {
			inc(ensure("statut_juridique", juridique))
		}
	}

	var result []models.UsageRecensement
	for dim, m := range dims {
		for key, c := range m {
			result = append(result, models.UsageRecensement{
				Dimension:        dim,
				Key:              key,
				Label:            key,
				NStructures:      c.total,
				NOperationnel:    c.oper,
				NNonOperationnel: c.nonOper,
				NFermeTemp:       c.ferme,
			})
		}
	}
	return result
}
