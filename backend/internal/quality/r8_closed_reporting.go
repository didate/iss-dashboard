package quality

import (
	"fmt"

	"iss-dashboard-backend/internal/models"
)

// CheckClosedReporting flags events submitted by org units after their closure date (R8).
func CheckClosedReporting(event *models.Event, ctx *QualityContext) []models.Issue {
	closedDate, ok := ctx.OrgUnitClosedDate[event.OrgUnitUID]
	if !ok || closedDate == "" {
		return nil
	}

	// Compare dates (both ISO format: YYYY-MM-DD)
	eventDate := event.EventDate
	if len(eventDate) > 10 {
		eventDate = eventDate[:10]
	}
	if len(closedDate) > 10 {
		closedDate = closedDate[:10]
	}

	if eventDate >= closedDate {
		return []models.Issue{{
			RuleCode: "R8",
			Severity: "warning",
			RuleName: "Rapport apres fermeture",
			Message:  fmt.Sprintf("%s : rapport soumis le %s alors que la structure est fermee depuis le %s", event.OrgUnitName, eventDate, closedDate),
		}}
	}

	return nil
}
