package quality

import (
	"iss-dashboard-backend/internal/models"
)

// Service-staff coherence rules:
// R10 — Maternité sans sage-femme
// R11 — Laboratoire sans technicien labo ni biologiste
// R12 — Pharmacie sans pharmacien

type serviceStaffRule struct {
	ServiceUID string
	ServiceName string
	StaffUIDs  []string
	StaffName  string
}

var serviceStaffRules = []serviceStaffRule{
	{
		ServiceUID:  "Xt8lMQH4wUY", // ISS_SVC_MATERNITE_DE
		ServiceName: "Maternité",
		StaffUIDs: []string{
			"cb6wT7726fr", // ISS_RH_SAGEF_FN_DE
			"KU8zq1MNa75", // ISS_RH_SAGEF_CT_DE
			"wuhQDLD9Pxg", // ISS_RH_SAGEF_BN_DE
		},
		StaffName: "sage-femme",
	},
	{
		ServiceUID:  "Zq34u53MgeI", // ISS_SVC_LABO_DE
		ServiceName: "Laboratoire",
		StaffUIDs: []string{
			"Lj57VFWXXV1", // ISS_RH_TECH_LAB_FN_DE
			"KbZz1cd4agz", // ISS_RH_TECH_LAB_CT_DE
			"A0g2VXT9kcb", // ISS_RH_TECH_LAB_BN_DE
			"NPTMo8Bid2m", // ISS_RH_BIO_FN_DE
			"EjDTf9f3C0y", // ISS_RH_BIO_CT_DE
			"OslD3g816RI", // ISS_RH_BIO_BN_DE
		},
		StaffName: "technicien de laboratoire ou biologiste",
	},
	{
		ServiceUID:  "GBFdKT6M2Or", // ISS_SVC_PHARMACIE_DE
		ServiceName: "Pharmacie",
		StaffUIDs: []string{
			"tcAdCwRUpWt", // ISS_RH_PHARM_FN_DE
			"UfPuzGxRQ1b", // ISS_RH_PHARM_CT_DE
			"JM1OXCgiGeY", // ISS_RH_PHARM_BN_DE
		},
		StaffName: "pharmacien",
	},
}

func checkServiceStaff(event *models.Event, _ *QualityContext, rule serviceStaffRule, ruleCode string) []models.Issue {
	vals := event.Values()

	svcVal := vals[rule.ServiceUID]
	if svcVal != "oui" {
		return nil
	}

	// Check if at least one staff member exists
	for _, uid := range rule.StaffUIDs {
		if v := vals[uid]; v != "" && ParseNum(v) > 0 {
			return nil
		}
	}

	return []models.Issue{{
		RuleCode: ruleCode,
		Severity: "warning",
		RuleName: rule.ServiceName + " sans " + rule.StaffName,
		Message:  rule.ServiceName + " déclarée fonctionnelle mais aucun(e) " + rule.StaffName + " renseigné(e) dans les RH",
	}}
}

// CheckMaternityStaff checks R10: maternité without sage-femme.
func CheckMaternityStaff(event *models.Event, ctx *QualityContext) []models.Issue {
	return checkServiceStaff(event, ctx, serviceStaffRules[0], "R10")
}

// CheckLabStaff checks R11: lab without technician or biologist.
func CheckLabStaff(event *models.Event, ctx *QualityContext) []models.Issue {
	return checkServiceStaff(event, ctx, serviceStaffRules[1], "R11")
}

// CheckPharmacyStaff checks R12: pharmacy without pharmacist.
func CheckPharmacyStaff(event *models.Event, ctx *QualityContext) []models.Issue {
	return checkServiceStaff(event, ctx, serviceStaffRules[2], "R12")
}
