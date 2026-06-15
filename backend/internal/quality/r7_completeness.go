package quality

import (
	"iss-dashboard-backend/internal/models"
)

// CheckCompleteness flags "empty shell" events with no equipment/RH data (R7).
func CheckCompleteness(event *models.Event, ctx *QualityContext) []models.Issue {
	vals := event.Values()

	// Check if any equipment total > 0
	hasEquip := false
	for _, pair := range ctx.EquipPairs {
		if v := vals[pair.TotalUID]; v != "" && ParseNum(v) > 0 {
			hasEquip = true
			break
		}
	}
	if hasEquip {
		return nil
	}

	// Check if any RH field > 0
	hasRH := false
	for uid, de := range ctx.MetadataByUID {
		if de.SectionPrefix != "ISS_RH" && de.SectionPrefix != "ISS_RH_SPE" {
			continue
		}
		if de.ValueType == "BOOLEAN" {
			continue
		}
		if v := vals[uid]; v != "" && ParseNum(v) > 0 {
			hasRH = true
			break
		}
	}

	if !hasEquip && !hasRH {
		return []models.Issue{{
			RuleCode: "R7",
			Severity: "info",
			RuleName: "Complétude",
			Message:  "Structure « coquille vide » : aucun équipement et aucune ressource humaine renseignés",
		}}
	}
	return nil
}
