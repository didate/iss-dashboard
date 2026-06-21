package quality

import (
	"fmt"

	"iss-dashboard-backend/internal/models"
)

// CheckInvalidOptions flags data values that have an invalid option code (R9).
// This catches values like "forage" that are not in the optionSet for the data element.
func CheckInvalidOptions(event *models.Event, ctx *QualityContext) []models.Issue {
	var issues []models.Issue
	vals := event.Values()

	for deUID, value := range vals {
		if value == "" {
			continue
		}

		de, ok := ctx.MetadataByUID[deUID]
		if !ok || de.OptionSetID == "" {
			continue
		}

		options, ok := ctx.OptionsBySet[de.OptionSetID]
		if !ok || len(options) == 0 {
			continue
		}

		// Check if the value is a valid option code
		valid := false
		for _, opt := range options {
			if opt.Code == value {
				valid = true
				break
			}
		}

		if !valid {
			validCodes := make([]string, 0, len(options))
			for _, opt := range options {
				validCodes = append(validCodes, opt.Code)
			}
			issues = append(issues, models.Issue{
				RuleCode: "R9",
				Severity: "error",
				RuleName: "Valeur invalide",
				Message:  fmt.Sprintf("%s : valeur « %s » n'est pas un code valide (attendu : %v)", de.DisplayName(), value, validCodes),
			})
		}
	}

	return issues
}
