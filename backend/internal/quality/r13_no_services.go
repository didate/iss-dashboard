package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckNoServices flags operational structures that have zero functional services (R13).
func CheckNoServices(event *models.Event, ctx *QualityContext) []models.Issue {
	vals := event.Values()

	// Only check operational structures
	statutUID := ctx.CodeToUID["ISS_STATUT_OP_DE"]
	if vals[statutUID] != "operationnel" {
		return nil
	}

	// Check all service DEs (section ISS_SVC)
	for uid, de := range ctx.MetadataByUID {
		if de.SectionPrefix != "ISS_SVC" {
			continue
		}
		if vals[uid] == "oui" {
			return nil // At least one service is functional
		}
	}

	return []models.Issue{{
		RuleCode: "R13",
		Severity: "warning",
		RuleName: "Aucun service déclaré",
		Message:  "Structure opérationnelle sans aucun service déclaré fonctionnel",
	}}
}
