package quality

import (
	"fmt"
	"strings"

	"iss-dashboard-backend/internal/models"
)

// CheckOutliers flags extreme values based on median + 5*MAD (R5).
func CheckOutliers(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue
	vals := event.Values()

	for _, pair := range ctx.EquipPairs {
		// Exclude beds (lits) — hospitals legitimately have high counts
		if strings.Contains(pair.Root, "LIT") {
			continue
		}
		stat, ok := ctx.Medians[pair.TotalUID]
		if !ok {
			continue
		}

		vStr := vals[pair.TotalUID]
		if vStr == "" {
			continue
		}
		v := ParseNum(vStr)

		threshold := stat.Median + 5*stat.MAD
		if v > threshold && v > 50 {
			issues = append(issues, models.Issue{
				RuleCode: "R5",
				Severity: "info",
				RuleName: "Valeurs aberrantes",
				Message:  fmt.Sprintf("%s : valeur %.0f dépasse le seuil (médiane=%.1f, MAD=%.1f, seuil=%.1f)", pair.Label, v, stat.Median, stat.MAD, threshold),
			})
		}
	}

	return issues
}
