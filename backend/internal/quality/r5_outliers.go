package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckOutliers was R5 — disabled: thresholds based on median+MAD are not
// meaningful for equipment counts that vary widely by facility type (hospitals
// vs health posts).  Kept as a no-op so the rule registry doesn't break.
func CheckOutliers(_ *models.Event, _ *QualityContext) []models.Issue {
	return nil
}
