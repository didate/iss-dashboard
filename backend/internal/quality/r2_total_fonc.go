package quality

import (
	"fmt"

	"iss-dashboard-backend/internal/models"
)

// CheckTotalFonctionnel checks total/fonctionnel coherence for equipment pairs (R2).
func CheckTotalFonctionnel(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue
	vals := event.Values()

	for _, pair := range ctx.EquipPairs {
		totalStr := vals[pair.TotalUID]
		foncStr := vals[pair.FoncUID]

		totalVal := ParseNum(totalStr)
		foncVal := ParseNum(foncStr)

		// If fonctionnel > total (both provided)
		if totalStr != "" && foncStr != "" && foncVal > totalVal {
			issues = append(issues, models.Issue{
				RuleCode: "R2",
				Severity: "error",
				RuleName: "Cohérence total/fonctionnel",
				Message:  fmt.Sprintf("%s : fonctionnel (%.0f) > total (%.0f)", pair.Label, foncVal, totalVal),
			})
		}

		// If fonctionnel > 0 but total empty
		if foncVal > 0 && totalStr == "" {
			issues = append(issues, models.Issue{
				RuleCode: "R2",
				Severity: "warning",
				RuleName: "Cohérence total/fonctionnel",
				Message:  fmt.Sprintf("%s : fonctionnel (%.0f) renseigné mais total manquant", pair.Label, foncVal),
			})
		}
	}

	return issues
}
