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
	// Les codes ci-dessous sont ceux réellement émis par chaque fonction (les
	// règles les fixent elles-mêmes sur l'issue) ; ils sont persistés dans
	// quality_issue.rule_code et servent de clé de filtre dans le front.
	// Ajouter une règle = ajouter une fonction ET une entrée ici avec un code libre.
	Registry = []Rule{
		// Complétude
		{Code: "R1", Name: "Champs obligatoires", Fn: CheckRequiredFields},
		{Code: "R7", Name: "Complétude", Fn: CheckCompleteness},
		// Cohérence
		{Code: "R2", Name: "Cohérence total/fonctionnel", Fn: CheckTotalFonctionnel},
		{Code: "R3", Name: "Service sans support", Fn: CheckServiceSupport},
		{Code: "R4", Name: "Cohérence commodités", Fn: CheckCommodites},
		{Code: "R5", Name: "Valeurs aberrantes", Fn: CheckOutliers}, // désactivée, voir r5_outliers.go
		{Code: "R9", Name: "Valeur invalide", Fn: CheckInvalidOptions},
		// Services et RH
		{Code: "R10", Name: "Maternité sans sage-femme", Fn: CheckMaternityStaff},
		{Code: "R11", Name: "Laboratoire sans technicien", Fn: CheckLabStaff},
		// R12 "Pharmacie sans pharmacien" (CheckPharmacyStaff) volontairement non
		// enregistrée : elle signalerait ~2 300 postes de santé, qui n'ont pas de pharmacien par norme.
		{Code: "R13", Name: "Aucun service déclaré", Fn: CheckNoServices},
		// WASH
		{Code: "R15", Name: "Source d'eau non renseignée", Fn: CheckMissingWaterSource},
		{Code: "R16", Name: "Source d'énergie non renseignée", Fn: CheckMissingEnergy},
		// Doublons et fermeture
		{Code: "R6", Name: "Soumissions multiples", Fn: CheckDuplicates},
		{Code: "R8", Name: "Rapport après fermeture", Fn: CheckClosedReporting},
		// Carte sanitaire : référentiel géographique et typologie
		{Code: "R14", Name: "Coordonnées GPS manquantes", Fn: CheckMissingGPS},
		{Code: "R17", Name: "Type de structure indéterminé", Fn: CheckTypologie},
		{Code: "R18", Name: "Statut juridique incohérent", Fn: CheckOwnership},
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
