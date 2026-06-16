package models

import (
	"encoding/json"
	"time"
)

// SyncRun tracks a synchronization execution.
type SyncRun struct {
	ID           int64      `json:"id"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Status       string     `json:"status"` // running, success, error
	EventsPulled int        `json:"events_pulled"`
	IssuesFound  int        `json:"issues_found"`
	DurationMs   int64      `json:"duration_ms"`
	ErrorText    string     `json:"error_text,omitempty"`
}

// Event represents a single DHIS2 event (one health facility).
type Event struct {
	EventUID    string      `json:"event"`
	OrgUnitUID  string      `json:"orgUnit"`
	OrgUnitName string      `json:"orgUnitName"`
	District    string      `json:"district"`
	Region      string      `json:"region"`
	EventDate   string      `json:"eventDate"`
	Status      string      `json:"status"`
	DataValues  []DataValue `json:"dataValues"`
	RawJSON     string      `json:"-"`
	SyncRunID   int64       `json:"-"`
}

// Values returns a map de_uid → value for quick lookups.
func (e *Event) Values() map[string]string {
	m := make(map[string]string, len(e.DataValues))
	for _, dv := range e.DataValues {
		m[dv.DataElement] = dv.Value
	}
	return m
}

// ValueByCode returns the value for a given DE code, using context lookup.
func (e *Event) ValueByCode(code string, codeToUID map[string]string) string {
	uid, ok := codeToUID[code]
	if !ok {
		return ""
	}
	for _, dv := range e.DataValues {
		if dv.DataElement == uid {
			return dv.Value
		}
	}
	return ""
}

// DataValue is a single data element value within an event.
type DataValue struct {
	DataElement string `json:"dataElement"`
	Value       string `json:"value"`
}

// DataElementMeta holds metadata about a data element.
type DataElementMeta struct {
	UID           string `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	ValueType     string `json:"valueType"`
	OptionSetID   string `json:"optionSetId,omitempty"`
	SectionPrefix string `json:"sectionPrefix,omitempty"`
}

// OptionEntry is one option within an option set.
type OptionEntry struct {
	OptionSetID string `json:"optionSetId"`
	Code        string `json:"code"`
	Name        string `json:"name"`
}

// OrgUnit from DHIS2 hierarchy.
type OrgUnit struct {
	UID        string `json:"id"`
	Name       string `json:"name"`
	Level      int    `json:"level"`
	ParentUID  string `json:"parentUid,omitempty"`
	ParentName string `json:"parentName,omitempty"`
	ClosedDate string `json:"closedDate,omitempty"`
	Geometry   string `json:"geometry,omitempty"`
}

// EquipPair represents a TOTAL/FONC equipment pair.
type EquipPair struct {
	Root     string // common root code (e.g. ISS_EQUI_FRIGO)
	TotalUID string
	FoncUID  string
	Label    string // human-readable name
}

// Issue is a quality problem detected on an event.
type Issue struct {
	RuleCode string `json:"rule_code"`
	Severity string `json:"severity"` // error, warning, info
	RuleName string `json:"rule_name"`
	Message  string `json:"message"`
}

// EventQuality is the quality summary for one event.
type EventQuality struct {
	EventUID      string `json:"event_uid"`
	NError        int    `json:"n_error"`
	NWarning      int    `json:"n_warning"`
	NInfo         int    `json:"n_info"`
	WorstSeverity string `json:"worst_severity"`
	Score         int    `json:"score"`
}

// QualitySummaryRow is a pre-aggregated quality row.
type QualitySummaryRow struct {
	Dimension   string  `json:"dimension"`
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	AvgScore    float64 `json:"avg_score"`
	NError      int     `json:"n_error"`
	NWarning    int     `json:"n_warning"`
	NInfo       int     `json:"n_info"`
	NStructures int     `json:"n_structures"`
}

// UsageRecensement is a census aggregation row.
type UsageRecensement struct {
	Dimension        string `json:"dimension"`
	Key              string `json:"key"`
	Label            string `json:"label"`
	NStructures      int    `json:"n_structures"`
	NOperationnel    int    `json:"n_operationnel"`
	NNonOperationnel int    `json:"n_non_operationnel"`
	NFermeTemp       int    `json:"n_ferme_temp"`
}

// UsageService is a service availability row.
type UsageService struct {
	ServiceCode    string  `json:"service_code"`
	ServiceLabel   string  `json:"service_label"`
	District       string  `json:"district"`
	NOui           int     `json:"n_oui"`
	NOuiPasFonc    int     `json:"n_oui_pas_fonc"`
	NNon           int     `json:"n_non"`
	NTotal         int     `json:"n_total"`
	PctFonctionnel float64 `json:"pct_fonctionnel"`
}

// UsageEquipement is an equipment functionality row.
type UsageEquipement struct {
	EquipRoot string  `json:"equip_root"`
	Label     string  `json:"label"`
	District  string  `json:"district"`
	SumTotal  int     `json:"sum_total"`
	SumFonct  int     `json:"sum_fonct"`
	PctFonct  float64 `json:"pct_fonct"`
	Category  string  `json:"category"`
}

// UsageRH is a human resources row.
type UsageRH struct {
	ProfilCode    string `json:"profil_code"`
	Label         string `json:"label"`
	District      string `json:"district"`
	EffectifFonc  int    `json:"effectif_fonc"`
	EffectifContr int    `json:"effectif_contr"`
	EffectifBenev int    `json:"effectif_benev"`
	EffectifTotal int    `json:"effectif_total"`
}

// UsageCommodite is a WASH/energy indicator row.
type UsageCommodite struct {
	Indicator string  `json:"indicator"`
	District  string  `json:"district"`
	NOui      int     `json:"n_oui"`
	NTotal    int     `json:"n_total"`
	Pct       float64 `json:"pct"`
}

// ReportingRate is a reporting completeness row.
type ReportingRate struct {
	Dimension  string  `json:"dimension"`
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	NExpected  int     `json:"n_expected"`
	NReported  int     `json:"n_reported"`
	Pct        float64 `json:"pct"`
}

// User represents an application user.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Name         string `json:"name"`
	Role         string `json:"role"` // admin, viewer
	CreatedAt    string `json:"created_at"`
}

// DHIS2 API response structures

type DHIS2EventsResponse struct {
	Pager  DHIS2Pager `json:"pager"`
	Events []Event    `json:"events"`
}

type DHIS2Pager struct {
	Page      int `json:"page"`
	PageCount int `json:"pageCount"`
	Total     int `json:"total"`
	PageSize  int `json:"pageSize"`
}

type DHIS2DataElementsResponse struct {
	DataElements []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Code      string `json:"code"`
		ValueType string `json:"valueType"`
		OptionSet *struct {
			ID string `json:"id"`
		} `json:"optionSet"`
	} `json:"dataElements"`
}

type DHIS2OptionSetsResponse struct {
	OptionSets []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Options []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"options"`
	} `json:"optionSets"`
}

type DHIS2OrgUnitsResponse struct {
	OrganisationUnits []struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Level      int             `json:"level"`
		ClosedDate string          `json:"closedDate"`
		Geometry   json.RawMessage `json:"geometry"`
		Parent     *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"parent"`
	} `json:"organisationUnits"`
}

// --- Map / Carte types ---

type ServiceMapData struct {
	ServiceLabel   string  `json:"service_label"`
	PctFonctionnel float64 `json:"pct_fonctionnel"`
	NOui           int     `json:"n_oui"`
	NTotal         int     `json:"n_total"`
}

type EquipMapData struct {
	Label    string `json:"label"`
	Category string `json:"category"`
	SumTotal int    `json:"sum_total"`
	SumFonct int    `json:"sum_fonct"`
}

type MapDistrictProperties struct {
	DistrictUID  string `json:"district_uid"`
	DistrictName string `json:"district_name"`
	// Couche 1 : Rapportage
	RapportagePct      *float64 `json:"rapportage_pct"`
	RapportageExpected int      `json:"rapportage_expected"`
	RapportageReported int      `json:"rapportage_reported"`
	// Couche 2 : Qualité
	QualiteAvgScore *float64 `json:"qualite_avg_score"`
	QualiteNStruct  int      `json:"qualite_n_structures"`
	// Couche 3 : Services
	Services map[string]ServiceMapData `json:"services"`
	// Couche 4 : Équipements (nombres bruts)
	Equipements map[string]EquipMapData `json:"equipements"`
	// Couche 5 : WASH forage/réseau
	WashForageOuReseauPct *float64 `json:"wash_forage_ou_reseau_pct"`
	WashForageOuReseauN   int      `json:"wash_forage_ou_reseau_n"`
	WashTotal             int      `json:"wash_total"`
	// Couche 6 : Densité RH
	RhMedecinsTotal     int      `json:"rh_medecins_total"`
	RhNStructures       int      `json:"rh_n_structures"`
	RhMedecinsParStruct *float64 `json:"rh_medecins_par_structure"`
}

type MapDistrictFeature struct {
	Type       string                 `json:"type"`
	Geometry   json.RawMessage        `json:"geometry"`
	Properties MapDistrictProperties  `json:"properties"`
}

type MapDistrictCollection struct {
	Type     string               `json:"type"`
	Features []MapDistrictFeature `json:"features"`
}
