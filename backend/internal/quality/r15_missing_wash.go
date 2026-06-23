package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckMissingWaterSource flags operational structures with no water source (R15).
func CheckMissingWaterSource(event *models.Event, ctx *QualityContext) []models.Issue {
	vals := event.Values()

	statutUID := ctx.CodeToUID["ISS_STATUT_OP_DE"]
	if vals[statutUID] != "operationnel" {
		return nil
	}

	eauUID := ctx.CodeToUID["ISS_SOURCE_EAU_DE"]
	if vals[eauUID] != "" {
		return nil
	}

	return []models.Issue{{
		RuleCode: "R15",
		Severity: "info",
		RuleName: "Source d'eau non renseignée",
		Message:  "Structure opérationnelle sans source d'eau renseignée",
	}}
}

// CheckMissingEnergy flags operational structures with no energy source info (R16).
func CheckMissingEnergy(event *models.Event, ctx *QualityContext) []models.Issue {
	vals := event.Values()

	statutUID := ctx.CodeToUID["ISS_STATUT_OP_DE"]
	if vals[statutUID] != "operationnel" {
		return nil
	}

	energieUID := ctx.CodeToUID["ISS_ENERGIE_OUI_NON_DE"]
	if vals[energieUID] != "" {
		return nil
	}

	return []models.Issue{{
		RuleCode: "R16",
		Severity: "info",
		RuleName: "Source d'énergie non renseignée",
		Message:  "Structure opérationnelle sans information sur l'énergie",
	}}
}
