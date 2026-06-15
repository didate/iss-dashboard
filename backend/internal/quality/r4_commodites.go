package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckCommodites checks energy/water coherence (R4).
func CheckCommodites(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue

	// Energy declared but no source checked
	energyUID := "G6aAGwMfuOH"
	reseauUID := "ZKd23M3NVu0"
	solaireUID := "Z5G3epiH9hh"
	genUID := "Ff1uAvJbxXm"

	if IsTruthy(GetEventValue(event, energyUID)) {
		reseau := IsTruthy(GetEventValue(event, reseauUID))
		solaire := IsTruthy(GetEventValue(event, solaireUID))
		generateur := IsTruthy(GetEventValue(event, genUID))

		if !reseau && !solaire && !generateur {
			issues = append(issues, models.Issue{
				RuleCode: "R4",
				Severity: "warning",
				RuleName: "Cohérence commodités",
				Message:  "Structure déclare disposer d'une source d'énergie mais aucune source cochée (réseau, solaire, générateur)",
			})
		}
	}

	// Water at critical points but source = aucune or empty
	eauPtsCritUID := "mr2SQNgReyd"
	sourceEauUID := "IzfXJ0Zrfxh"

	if IsTruthy(GetEventValue(event, eauPtsCritUID)) {
		sourceEau := GetEventValue(event, sourceEauUID)
		if sourceEau == "" || sourceEau == "aucune" {
			issues = append(issues, models.Issue{
				RuleCode: "R4",
				Severity: "info",
				RuleName: "Cohérence commodités",
				Message:  "Eau déclarée aux points critiques mais source d'eau absente ou « aucune »",
			})
		}
	}

	return issues
}
