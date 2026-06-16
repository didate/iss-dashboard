package quality

import (
	"fmt"

	"iss-dashboard-backend/internal/models"
)

// CheckDuplicates flags events when an org unit has more than one submission per year (R6).
func CheckDuplicates(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue

	// Check per year: an OU should submit only one form per year
	year := ""
	if len(event.EventDate) >= 4 {
		year = event.EventDate[:4]
	}
	yearKey := event.OrgUnitUID + "|" + year
	countYear := ctx.OrgUnitYearCounts[yearKey]
	if countYear > 1 {
		label := year
		if label == "" {
			label = "sans date"
		}
		issues = append(issues, models.Issue{
			RuleCode: "R6",
			Severity: "warning",
			RuleName: "Soumissions multiples",
			Message:  fmt.Sprintf("%s : %d soumissions pour l'annee %s (1 attendue)", event.OrgUnitName, countYear, label),
		})
	}

	return issues
}
