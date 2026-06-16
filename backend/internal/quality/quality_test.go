package quality

import (
	"testing"

	"iss-dashboard-backend/internal/models"
)

func makeEvent(uid string, dataValues map[string]string) *models.Event {
	evt := &models.Event{
		EventUID:    uid,
		OrgUnitUID:  "ou1",
		OrgUnitName: "Test Facility",
		EventDate:   "2025-01-15",
		Status:      "COMPLETED",
	}
	for deUID, val := range dataValues {
		evt.DataValues = append(evt.DataValues, models.DataValue{
			DataElement: deUID,
			Value:       val,
		})
	}
	return evt
}

func buildTestContext(events []*models.Event) *QualityContext {
	metadata := []models.DataElementMeta{
		{UID: "HpjvSNCEWM0", Code: "ISS_STATUT_OP_DE", Name: "Statut opérationnel", ValueType: "TEXT", SectionPrefix: "ISS_GEN"},
		{UID: "GLngjZxh1Vm", Code: "ISS_GEN_NOM_RESP_DE", Name: "Nom du responsable", ValueType: "TEXT", SectionPrefix: "ISS_GEN"},
		{UID: "totalA", Code: "ISS_EQUI_FRIGO_TOTAL_DE", Name: "Réfrigérateurs total", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "foncA", Code: "ISS_EQUI_FRIGO_FONC_DE", Name: "Réfrigérateurs fonctionnel", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "totalB", Code: "ISS_EQUI_LIT_TOTAL_DE", Name: "Lits total", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "foncB", Code: "ISS_EQUI_LIT_FONC_DE", Name: "Lits fonctionnel", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "Zq34u53MgeI", Code: "ISS_SVC_LABO_DE", Name: "Service laboratoire", ValueType: "TEXT", SectionPrefix: "ISS_LAB"},
		{UID: "bWGkmx4RfoE", Code: "ISS_EQUI_MICROSCOPE_FONC_DE", Name: "Microscopes fonctionnel", ValueType: "NUMBER", SectionPrefix: "ISS_EQ"},
		{UID: "HfU1YquEtbm", Code: "ISS_INFRA_LABO_DE", Name: "Salles de laboratoire", ValueType: "NUMBER", SectionPrefix: "ISS_INFRA"},
		{UID: "G6aAGwMfuOH", Code: "ISS_ENERGIE_OUI_NON_DE", Name: "Source d'énergie", ValueType: "BOOLEAN", SectionPrefix: "ISS_COMMO"},
		{UID: "ZKd23M3NVu0", Code: "ISS_ENERGIE_RESEAU_DE", Name: "Réseau électrique", ValueType: "BOOLEAN", SectionPrefix: "ISS_COMMO"},
		{UID: "Z5G3epiH9hh", Code: "ISS_ENERGIE_SOLAIRE_DE", Name: "Solaire", ValueType: "BOOLEAN", SectionPrefix: "ISS_COMMO"},
		{UID: "Ff1uAvJbxXm", Code: "ISS_ENERGIE_GEN_DE", Name: "Générateur", ValueType: "BOOLEAN", SectionPrefix: "ISS_COMMO"},
		{UID: "mr2SQNgReyd", Code: "ISS_EAU_DISPO_PTS_CRITIQUES", Name: "Eau pts critiques", ValueType: "BOOLEAN", SectionPrefix: "ISS_COMMO"},
		{UID: "IzfXJ0Zrfxh", Code: "ISS_SOURCE_EAU_DE", Name: "Source d'eau", ValueType: "TEXT", SectionPrefix: "ISS_COMMO"},
		// RH field for completeness check
		{UID: "rhField1", Code: "ISS_RH_INF_FN_DE", Name: "Infirmier fonctionnaire", ValueType: "NUMBER", SectionPrefix: "ISS_RH"},
	}

	options := []models.OptionEntry{
		{OptionSetID: "zGp7IfEpwim", Code: "operationnel", Name: "Opérationnel"},
		{OptionSetID: "RGsTov6dBHH", Code: "oui", Name: "Oui, fonctionnel"},
	}

	return BuildContext(metadata, options, events, nil)
}

// --- R1 Tests ---

func TestR1_MissingDate(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"HpjvSNCEWM0": "operationnel",
		"GLngjZxh1Vm": "Dr Test",
	})
	evt.EventDate = ""
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckRequiredFields(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Severity != "error" {
		t.Errorf("expected error severity, got %s", issues[0].Severity)
	}
}

func TestR1_MissingStatut(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"GLngjZxh1Vm": "Dr Test",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckRequiredFields(evt, ctx)
	found := false
	for _, iss := range issues {
		if iss.Message == "Statut opérationnel manquant" {
			found = true
			if iss.Severity != "warning" {
				t.Errorf("expected warning, got %s", iss.Severity)
			}
		}
	}
	if !found {
		t.Error("expected statut opérationnel issue")
	}
}

func TestR1_AllPresent(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"HpjvSNCEWM0": "operationnel",
		"GLngjZxh1Vm": "Dr Test",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckRequiredFields(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %+v", len(issues), issues)
	}
}

// --- R2 Tests ---

func TestR2_FoncGreaterThanTotal(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"totalA": "3",
		"foncA":  "5",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckTotalFonctionnel(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Severity != "error" {
		t.Errorf("expected error, got %s", issues[0].Severity)
	}
}

func TestR2_FoncWithoutTotal(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"foncA": "2",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckTotalFonctionnel(evt, ctx)
	found := false
	for _, iss := range issues {
		if iss.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for fonc without total")
	}
}

func TestR2_Coherent(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"totalA": "5",
		"foncA":  "3",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckTotalFonctionnel(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

// --- R3 Tests ---

func TestR3_LaboWithoutSupport(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"Zq34u53MgeI": "oui",
		"bWGkmx4RfoE": "0",
		"HfU1YquEtbm": "0",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckServiceSupport(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
}

func TestR3_LaboWithMicroscope(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"Zq34u53MgeI": "oui",
		"bWGkmx4RfoE": "2",
		"HfU1YquEtbm": "0",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckServiceSupport(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

// --- R4 Tests ---

func TestR4_EnergyWithoutSource(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"G6aAGwMfuOH": "true",
		// no source checked
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckCommodites(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Severity != "warning" {
		t.Errorf("expected warning, got %s", issues[0].Severity)
	}
}

func TestR4_EnergyWithSolaire(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"G6aAGwMfuOH": "true",
		"Z5G3epiH9hh": "true",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckCommodites(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestR4_WaterWithoutSource(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"mr2SQNgReyd": "true",
		"IzfXJ0Zrfxh": "aucune",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckCommodites(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Severity != "info" {
		t.Errorf("expected info, got %s", issues[0].Severity)
	}
}

// --- R5 Tests ---

func TestR5_Outlier(t *testing.T) {
	// Create many events with normal values, plus one outlier
	var events []*models.Event
	for i := 0; i < 20; i++ {
		events = append(events, makeEvent("normal", map[string]string{
			"totalA": "5",
		}))
	}
	outlier := makeEvent("outlier", map[string]string{
		"totalA": "500",
	})
	events = append(events, outlier)
	// Fix UIDs to be unique for org unit counting
	for i, e := range events {
		e.OrgUnitUID = "ou_" + string(rune('A'+i))
	}

	ctx := buildTestContext(events)
	issues := CheckOutliers(outlier, ctx)
	if len(issues) == 0 {
		t.Fatal("expected outlier to be flagged")
	}
}

func TestR5_NormalValue(t *testing.T) {
	var events []*models.Event
	for i := 0; i < 20; i++ {
		e := makeEvent("e", map[string]string{"totalA": "5"})
		e.OrgUnitUID = "ou_" + string(rune('A'+i))
		events = append(events, e)
	}
	ctx := buildTestContext(events)
	issues := CheckOutliers(events[0], ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for normal value, got %d", len(issues))
	}
}

// --- R6 Tests ---

func TestR6_Duplicate(t *testing.T) {
	e1 := makeEvent("e1", nil)
	e1.OrgUnitUID = "ou1"
	e2 := makeEvent("e2", nil)
	e2.OrgUnitUID = "ou1"
	ctx := buildTestContext([]*models.Event{e1, e2})

	issues := CheckDuplicates(e1, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 duplicate issue, got %d", len(issues))
	}
}

func TestR6_NoDuplicate_DifferentOU(t *testing.T) {
	e1 := makeEvent("e1", nil)
	e1.OrgUnitUID = "ou1"
	e2 := makeEvent("e2", nil)
	e2.OrgUnitUID = "ou2"
	ctx := buildTestContext([]*models.Event{e1, e2})

	issues := CheckDuplicates(e1, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestR6_NoDuplicate_DifferentYear(t *testing.T) {
	e1 := makeEvent("e1", nil)
	e1.OrgUnitUID = "ou1"
	e1.EventDate = "2025-01-15"
	e2 := makeEvent("e2", nil)
	e2.OrgUnitUID = "ou1"
	e2.EventDate = "2026-03-20"
	ctx := buildTestContext([]*models.Event{e1, e2})

	issues := CheckDuplicates(e1, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for different years, got %d: %+v", len(issues), issues)
	}
}

// --- R8 Tests ---

func TestR8_ClosedReporting(t *testing.T) {
	evt := makeEvent("e1", nil)
	evt.OrgUnitUID = "ou_closed"
	evt.EventDate = "2026-03-15"
	orgUnits := []models.OrgUnit{
		{UID: "ou_closed", Name: "Closed Facility", ClosedDate: "2025-12-31"},
	}
	ctx := BuildContext(nil, nil, []*models.Event{evt}, orgUnits)
	issues := CheckClosedReporting(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Severity != "warning" {
		t.Errorf("expected warning, got %s", issues[0].Severity)
	}
}

func TestR8_OpenFacility(t *testing.T) {
	evt := makeEvent("e1", nil)
	evt.OrgUnitUID = "ou_open"
	evt.EventDate = "2026-03-15"
	ctx := BuildContext(nil, nil, []*models.Event{evt}, nil)
	issues := CheckClosedReporting(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestR8_ReportBeforeClosure(t *testing.T) {
	evt := makeEvent("e1", nil)
	evt.OrgUnitUID = "ou_closed"
	evt.EventDate = "2025-06-15"
	orgUnits := []models.OrgUnit{
		{UID: "ou_closed", Name: "Closed Facility", ClosedDate: "2025-12-31"},
	}
	ctx := BuildContext(nil, nil, []*models.Event{evt}, orgUnits)
	issues := CheckClosedReporting(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for report before closure, got %d", len(issues))
	}
}

// --- R7 Tests ---

func TestR7_EmptyShell(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"HpjvSNCEWM0": "operationnel",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckCompleteness(evt, ctx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
}

func TestR7_WithEquipment(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"totalA": "3",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckCompleteness(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestR7_WithRH(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		"rhField1": "2",
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := CheckCompleteness(evt, ctx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

// --- Score Tests ---

func TestScore_Perfect(t *testing.T) {
	eq := ComputeScore(nil)
	if eq.Score != 100 {
		t.Errorf("expected 100, got %d", eq.Score)
	}
}

func TestScore_WithErrors(t *testing.T) {
	issues := []models.Issue{
		{Severity: "error"},
		{Severity: "warning"},
		{Severity: "info"},
	}
	eq := ComputeScore(issues)
	// 100 - 15 - 5 - 1 = 79
	if eq.Score != 79 {
		t.Errorf("expected 79, got %d", eq.Score)
	}
	if eq.WorstSeverity != "error" {
		t.Errorf("expected error, got %s", eq.WorstSeverity)
	}
}

func TestScore_Floor(t *testing.T) {
	issues := []models.Issue{
		{Severity: "error"},
		{Severity: "error"},
		{Severity: "error"},
		{Severity: "error"},
		{Severity: "error"},
		{Severity: "error"},
		{Severity: "error"},
	}
	eq := ComputeScore(issues)
	if eq.Score != 0 {
		t.Errorf("expected 0 (floor), got %d", eq.Score)
	}
}

// --- RunAll integration ---

func TestRunAll_Integration(t *testing.T) {
	evt := makeEvent("e1", map[string]string{
		// Missing statut and responsable → 2 R1 warnings
		"totalA": "3",
		"foncA":  "5", // R2 error: fonc > total
	})
	ctx := buildTestContext([]*models.Event{evt})

	issues := RunAll(evt, ctx)
	if len(issues) < 3 {
		t.Fatalf("expected at least 3 issues, got %d: %+v", len(issues), issues)
	}

	eq := ComputeScore(issues)
	if eq.NError < 1 {
		t.Error("expected at least 1 error")
	}
	if eq.NWarning < 2 {
		t.Error("expected at least 2 warnings")
	}
}
