package quality

import (
	"iss-dashboard-backend/internal/models"
)

// Rule is a function that checks an event and returns issues.
type Rule struct {
	Code string
	Name string
	Fn   func(event *models.Event, ctx *QualityContext) []models.Issue
}

// Registry holds all registered quality rules.
var Registry []Rule

func init() {
	Registry = []Rule{
		// Complétude
		{Code: "R1", Name: "Champs obligatoires", Fn: CheckRequiredFields},
		{Code: "R2", Name: "Complétude", Fn: CheckCompleteness},
		// Cohérence
		{Code: "R3", Name: "Cohérence total/fonctionnel", Fn: CheckTotalFonctionnel},
		{Code: "R4", Name: "Service sans support", Fn: CheckServiceSupport},
		{Code: "R5", Name: "Cohérence commodités", Fn: CheckCommodites},
		{Code: "R6", Name: "Valeur invalide", Fn: CheckInvalidOptions},
		// Services et RH
		{Code: "R7", Name: "Maternité sans sage-femme", Fn: CheckMaternityStaff},
		{Code: "R8", Name: "Laboratoire sans technicien", Fn: CheckLabStaff},
		{Code: "R9", Name: "Aucun service déclaré", Fn: CheckNoServices},
		// WASH
		{Code: "R10", Name: "Source d'eau non renseignée", Fn: CheckMissingWaterSource},
		{Code: "R11", Name: "Source d'énergie non renseignée", Fn: CheckMissingEnergy},
		// Doublons et fermeture
		{Code: "R12", Name: "Soumissions multiples", Fn: CheckDuplicates},
		{Code: "R13", Name: "Rapport après fermeture", Fn: CheckClosedReporting},
	}
}

// RunAll runs all rules on one event and returns the combined issues.
func RunAll(event *models.Event, ctx *QualityContext) []models.Issue {
	var all []models.Issue
	for _, r := range Registry {
		issues := r.Fn(event, ctx)
		for i := range issues {
			if issues[i].RuleCode == "" {
				issues[i].RuleCode = r.Code
			}
			if issues[i].RuleName == "" {
				issues[i].RuleName = r.Name
			}
		}
		all = append(all, issues...)
	}
	return all
}
