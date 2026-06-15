package quality

import "iss-dashboard-backend/internal/models"

// ComputeScore calculates the quality score for an event based on its issues.
// score = max(0, 100 - 15*errors - 5*warnings - 1*infos)
func ComputeScore(issues []models.Issue) models.EventQuality {
	var eq models.EventQuality
	for _, iss := range issues {
		switch iss.Severity {
		case "error":
			eq.NError++
		case "warning":
			eq.NWarning++
		case "info":
			eq.NInfo++
		}
	}

	eq.Score = 100 - 15*eq.NError - 5*eq.NWarning - 1*eq.NInfo
	if eq.Score < 0 {
		eq.Score = 0
	}

	if eq.NError > 0 {
		eq.WorstSeverity = "error"
	} else if eq.NWarning > 0 {
		eq.WorstSeverity = "warning"
	} else if eq.NInfo > 0 {
		eq.WorstSeverity = "info"
	}

	return eq
}
