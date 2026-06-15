package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckServiceSupport checks services declared functional without required support (R3).
func CheckServiceSupport(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue

	for _, spec := range ctx.ServiceSpecs {
		serviceVal := GetEventValue(event, spec.ServiceUID)
		if serviceVal != "oui" {
			continue
		}

		// Check if ALL support fields are 0 or empty
		allZero := true
		for _, supportUID := range spec.SupportUIDs {
			if supportUID == "" {
				continue
			}
			v := GetEventValue(event, supportUID)
			if v != "" && ParseNum(v) > 0 {
				allZero = false
				break
			}
		}

		if allZero {
			issues = append(issues, models.Issue{
				RuleCode: "R3",
				Severity: "warning",
				RuleName: "Service sans support",
				Message:  spec.Message,
			})
		}
	}

	return issues
}
