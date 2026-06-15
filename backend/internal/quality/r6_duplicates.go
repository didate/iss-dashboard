package quality

import (
	"fmt"

	"iss-dashboard-backend/internal/models"
)

// CheckDuplicates flags events sharing the same org unit (R6).
func CheckDuplicates(event *models.Event, ctx *QualityContext) []models.Issue {
	count := ctx.OrgUnitCounts[event.OrgUnitUID]
	if count > 1 {
		return []models.Issue{{
			RuleCode: "R6",
			Severity: "warning",
			RuleName: "Doublons",
			Message:  fmt.Sprintf("Doublon potentiel : %d événements actifs sur l'org unit %s", count, event.OrgUnitName),
		}}
	}
	return nil
}
