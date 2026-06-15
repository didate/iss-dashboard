package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckRequiredFields checks for mandatory fields (R1).
func CheckRequiredFields(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue

	// Date d'event absente → error
	if event.EventDate == "" {
		issues = append(issues, models.Issue{
			RuleCode: "R1",
			Severity: "error",
			RuleName: "Champs obligatoires",
			Message:  "Date de l'événement absente",
		})
	}

	// Statut opérationnel manquant → warning
	statutUID := "HpjvSNCEWM0"
	if v := GetEventValue(event, statutUID); v == "" {
		issues = append(issues, models.Issue{
			RuleCode: "R1",
			Severity: "warning",
			RuleName: "Champs obligatoires",
			Message:  "Statut opérationnel manquant",
		})
	}

	// Nom du responsable manquant → warning
	respUID := "GLngjZxh1Vm"
	if v := GetEventValue(event, respUID); v == "" {
		issues = append(issues, models.Issue{
			RuleCode: "R1",
			Severity: "warning",
			RuleName: "Champs obligatoires",
			Message:  "Nom du responsable manquant",
		})
	}

	return issues
}
