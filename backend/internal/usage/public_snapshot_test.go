package usage

import (
	"encoding/json"
	"strings"
	"testing"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

func publicCtx() *quality.QualityContext {
	meta := []models.DataElementMeta{
		{UID: "statStruct", Code: "ISS_STATUT_STRUCT_DE", Name: "Statut", SectionPrefix: "ISS_GEN"},
		{UID: "statOp", Code: "ISS_STATUT_OP_DE", Name: "Op", SectionPrefix: "ISS_GEN"},
		{UID: "svcMat", Code: "ISS_SVC_MATERNITE_DE", Name: "ISS_SVC Service de maternité", SectionPrefix: "ISS_SVC", OptionSetID: serviceOptionSet},
		{UID: "svcLab", Code: "ISS_SVC_LABO_DE", Name: "ISS_LAB Service laboratoire", SectionPrefix: "ISS_LAB", OptionSetID: serviceOptionSet},
		{UID: "respName", Code: "ISS_GEN_NOM_RESP_DE", Name: "Nom du responsable", SectionPrefix: "ISS_GEN"},
		{UID: "rhInf", Code: "ISS_RH_INF_FN_DE", Name: "Infirmiers", SectionPrefix: "ISS_RH"},
	}
	return quality.BuildContext(meta, nil, nil, nil)
}

func TestBuildPublicSnapshot_ReducedProjection(t *testing.T) {
	ctx := publicCtx()
	old := &models.Event{EventUID: "e0", OrgUnitUID: "ou1", OrgUnitName: "CS Alpha", EventDate: "2024-01-01", Region: "R", District: "D", TypeCode: "CS", Lat: pf(9.5), Lng: pf(-13.7),
		DataValues: []models.DataValue{{DataElement: "svcMat", Value: "non"}}}
	recent := &models.Event{EventUID: "e1", OrgUnitUID: "ou1", OrgUnitName: "CS Alpha", EventDate: "2025-06-01", Region: "R", District: "D", SousPrefecture: "SP", TypeCode: "CS", Lat: pf(9.5), Lng: pf(-13.7),
		DataValues: []models.DataValue{
			{DataElement: "svcMat", Value: "oui"}, {DataElement: "svcLab", Value: "oui_pas_fonctionnel"},
			{DataElement: "statStruct", Value: "publique"}, {DataElement: "statOp", Value: "operationnel"},
			{DataElement: "respName", Value: "Dr Secret"}, {DataElement: "rhInf", Value: "12"},
		}}
	noGPS := &models.Event{EventUID: "e2", OrgUnitUID: "ou2", OrgUnitName: "PS Beta", EventDate: "2025-06-01", Region: "R", District: "D2", TypeCode: "PS"}

	blobs, extras, err := BuildPublicSnapshot([]*models.Event{old, recent, noGPS}, ctx, []models.OrgUnit{{UID: "ou1", Level: 5}})
	if err != nil {
		t.Fatal(err)
	}

	var fc models.PublicPointCollection
	if err := json.Unmarshal(blobs[BlobPublicPoints], &fc); err != nil {
		t.Fatal(err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("only geolocated structures, one per org unit: got %d features", len(fc.Features))
	}
	p := fc.Features[0].Properties
	if p.UID != "ou1" || p.TypeLabel != "Centre de santé" || p.StatutJuri != "publique" || p.StatutOp != "operationnel" || p.SousPrefecture != "SP" {
		t.Fatalf("properties: %+v", p)
	}
	if len(p.Services) != 1 || p.Services[0] != "MATERNITE" {
		t.Fatalf("services must list only 'oui' with short keys, got %v (the older event said non)", p.Services)
	}
	raw := string(blobs[BlobPublicPoints])
	for _, secret := range []string{"Dr Secret", "rhInf"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("public payload leaks %q", secret)
		}
	}
	// Agrégats publics : effectif RH total (12 infirmiers), niveau, score de services (maternité n'en fait pas partie)
	if p.Niveau != 5 || p.RhTotal == nil || *p.RhTotal != 12 || *p.RhSoignants != 12 || *p.RhMedecins != 0 {
		t.Fatalf("extras: %+v", p.PublicExtras)
	}
	// Labo (service principal) répondu « prévu mais non fonctionnel » → score 0 / 7
	if p.ScoreServices == nil || *p.ScoreServices != 0 || p.ScoreServicesN != 7 {
		t.Fatalf("score services: %v / %d", p.ScoreServices, p.ScoreServicesN)
	}
	if len(extras) != 2 {
		t.Fatalf("one extras row per org unit, got %d", len(extras))
	}
	var geom struct {
		Coordinates []float64 `json:"coordinates"`
	}
	json.Unmarshal(fc.Features[0].Geometry, &geom)
	if geom.Coordinates[0] != -13.7 || geom.Coordinates[1] != 9.5 {
		t.Fatalf("GeoJSON order is [lng, lat]: %v", geom.Coordinates)
	}

	var f models.PublicFilters
	json.Unmarshal(blobs[BlobPublicFilters], &f)
	if len(f.Types) != 2 || f.Types[0].Code != "CS" && f.Types[1].Code != "CS" {
		t.Fatalf("types: %+v", f.Types)
	}
	if len(f.Services) != 2 || f.Services[0].Label != "Service de maternité" && f.Services[1].Label != "Service de maternité" {
		t.Fatalf("services: %+v", f.Services)
	}
	if len(f.Districts) != 2 || f.Districts[0].Name != "D" || f.Districts[0].Region != "R" {
		t.Fatalf("districts: %+v", f.Districts)
	}
}
