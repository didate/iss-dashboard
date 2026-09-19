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

	// Carte sanitaire : attributs de la structure dérivés au sync (hiérarchie,
	// typologie, position). Jamais fournis par DHIS2 tels quels.
	DistrictUID       string   `json:"districtUid,omitempty"`
	SousPrefecture    string   `json:"sousPrefecture,omitempty"`
	SousPrefectureUID string   `json:"sousPrefectureUid,omitempty"`
	TypeCode          string   `json:"typeCode,omitempty"`   // PS|CS|CSA|CMC|HP|HR|HN|CABINET|CLINIQUE|AUTRE_PRIVE|INDETERMINE
	TypeSource        string   `json:"typeSource,omitempty"` // group|group_multiple|name|none
	Lat               *float64 `json:"lat,omitempty"`
	Lng               *float64 `json:"lng,omitempty"`
}

// HasGPS reports whether the structure has a usable point position.
func (e *Event) HasGPS() bool { return e.Lat != nil && e.Lng != nil }

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
	FormName      string `json:"formName,omitempty"`
	ValueType     string `json:"valueType"`
	OptionSetID   string `json:"optionSetId,omitempty"`
	SectionPrefix string `json:"sectionPrefix,omitempty"`
}

// DisplayName returns formName if set, otherwise name.
func (d DataElementMeta) DisplayName() string {
	if d.FormName != "" {
		return d.FormName
	}
	return d.Name
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

// OrgUnitGroup is one org unit's membership in a DHIS2 organisation unit group.
type OrgUnitGroup struct {
	GroupUID  string `json:"groupUid"`
	GroupName string `json:"groupName"`
	SetName   string `json:"setName"` // parent group set name, "" if the group belongs to none
	OrgUnit   string `json:"orgUnit"`
}

// PopulationRow is the latest known population figure for one org unit and indicator.
type PopulationRow struct {
	OrgUnitUID string  `json:"ou_uid"`
	Indicator  string  `json:"indicator"` // total | moins5 | fap | grossesses | accouchements ...
	Period     string  `json:"period"`    // DHIS2 period id, e.g. 202508
	Value      float64 `json:"value"`
}

// UsageGeo is the geographic/demographic coverage of one administrative unit.
type UsageGeo struct {
	Level       int            `json:"level"` // 3 = district/préfecture, 4 = sous-préfecture
	OrgUnitUID  string         `json:"ou_uid"`
	Name        string         `json:"name"`
	ParentName  string         `json:"parent_name"`
	NStructures int            `json:"n_structures"`
	NGPS        int            `json:"n_gps"`
	PctGPS      *float64       `json:"pct_gps"`
	AvgScore    *float64       `json:"avg_score"`
	Population  *float64       `json:"population"`
	NParType    map[string]int `json:"n_par_type"`
}

// UsageCouverture is one demographic ratio (numerator per 10 000 inhabitants).
type UsageCouverture struct {
	Dimension  string   `json:"dimension"` // global | region | district | sous_prefecture
	Key        string   `json:"key"`
	OrgUnitUID string   `json:"ou_uid"`
	Label      string   `json:"label"`
	Indicator  string   `json:"indicator"` // structures | lits | medecins | sages_femmes | infirmiers | ats
	Numerator  float64  `json:"numerator"`
	Population *float64 `json:"population"`
	Ratio10k   *float64 `json:"ratio_10k"`
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
	EffectifASC   int    `json:"effectif_asc"`  // Agents de santé communautaire
	EffectifRECO  int    `json:"effectif_reco"` // Relais communautaires
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
	Dimension string  `json:"dimension"`
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	NExpected int     `json:"n_expected"`
	NReported int     `json:"n_reported"`
	Pct       float64 `json:"pct"`
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
		FormName  string `json:"formName"`
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

type RhMapData struct {
	Label         string `json:"label"`
	EffectifTotal int    `json:"effectif_total"`
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
	WashForageOuReseauPct  *float64 `json:"wash_forage_ou_reseau_pct"`
	WashForageOuReseauN    int      `json:"wash_forage_ou_reseau_n"`
	WashTotal              int      `json:"wash_total"`
	WashEauPtsCritiquesPct *float64 `json:"wash_eau_pts_critiques_pct"`
	WashEauPtsCritiquesN   int      `json:"wash_eau_pts_critiques_n"`
	// Couche 6 : Densité RH
	RhMedecinsTotal     int      `json:"rh_medecins_total"`
	RhNStructures       int      `json:"rh_n_structures"`
	RhMedecinsParStruct *float64 `json:"rh_medecins_par_structure"`
	// Effectifs par profil RH (pour le sélecteur de densité par type de RH)
	Rh map[string]RhMapData `json:"rh"`
}

type MapDistrictFeature struct {
	Type       string                `json:"type"`
	Geometry   json.RawMessage       `json:"geometry"`
	Properties MapDistrictProperties `json:"properties"`
}

type MapDistrictCollection struct {
	Type     string               `json:"type"`
	Features []MapDistrictFeature `json:"features"`
}

// --- Carte sanitaire : projections publiques (fiche réduite, option B) ---

// PublicService is one service offered by a structure.
type PublicService struct {
	Code  string `json:"code"`  // short key, e.g. MATERNITE
	Label string `json:"label"` // human label
}

// PublicPointProperties are the reduced properties of one structure on the public map.
// Never carries HR counts, equipment counts, responsible person or quality data.
type PublicPointProperties struct {
	UID            string   `json:"uid"` // org unit uid = stable public id
	Name           string   `json:"name"`
	TypeCode       string   `json:"type"`
	TypeLabel      string   `json:"type_label"`
	StatutJuri     string   `json:"statut"` // publique | privée | ""
	StatutOp       string   `json:"op"`     // operationnel | non_operationnel | ferme_temporairement | ""
	Region         string   `json:"region"`
	District       string   `json:"district"`
	SousPrefecture string   `json:"sp"`
	Services       []string `json:"svc"` // short service keys declared "oui"
}

type PublicPointFeature struct {
	Type       string                `json:"type"`
	Geometry   json.RawMessage       `json:"geometry"`
	Properties PublicPointProperties `json:"properties"`
}

type PublicPointCollection struct {
	Type     string               `json:"type"`
	Features []PublicPointFeature `json:"features"`
}

// PublicFilterType is a structure type with its count.
type PublicFilterType struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	N     int    `json:"n"`
}

type PublicFilterDistrict struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

// PublicFilters feeds the public map's filter widgets.
type PublicFilters struct {
	Types     []PublicFilterType     `json:"types"`
	Services  []PublicService        `json:"services"`
	Regions   []string               `json:"regions"`
	Districts []PublicFilterDistrict `json:"districts"`
}

// PublicStructure is the reduced public record (fiche) of one structure.
type PublicStructure struct {
	UID            string          `json:"uid"`
	Name           string          `json:"name"`
	TypeCode       string          `json:"type"`
	TypeLabel      string          `json:"type_label"`
	StatutJuri     string          `json:"statut"`
	StatutDetail   string          `json:"statut_detail"` // parapublique, confessionnel, ...
	StatutOp       string          `json:"op"`
	Region         string          `json:"region"`
	District       string          `json:"district"`
	SousPrefecture string          `json:"sous_prefecture"`
	Lat            *float64        `json:"lat"`
	Lng            *float64        `json:"lng"`
	Services       []PublicService `json:"services"`
	Plateau        map[string]bool `json:"plateau"` // labo, maternite, imagerie, urgences, pharmacie, chirurgie
	RecenseLe      string          `json:"recense_le"`
}

// PublicStructureItem is one public search result.
type PublicStructureItem struct {
	UID        string   `json:"uid"`
	Name       string   `json:"name"`
	TypeCode   string   `json:"type"`
	TypeLabel  string   `json:"type_label"`
	StatutOp   string   `json:"op"`
	Region     string   `json:"region"`
	District   string   `json:"district"`
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	DistanceKm *float64 `json:"distance_km,omitempty"`
}

// PublicSummary is the public landing page's headline figures.
type PublicSummary struct {
	NStructures     int            `json:"n_structures"`
	NParType        map[string]int `json:"n_par_type"`
	PctGPS          float64        `json:"pct_gps"`
	DerniereSynchro string         `json:"derniere_synchro"`
	// true = l'espace planification est lisible sans connexion (DASHBOARD_PUBLIC)
	DashboardPublic bool `json:"dashboard_public"`
}

// MissingGPSItem is one structure without coordinates (pro geolocation screen).
type MissingGPSItem struct {
	EventUID       string `json:"event_uid"`
	OrgUnitUID     string `json:"org_unit_uid"`
	Name           string `json:"name"`
	TypeCode       string `json:"type"`
	TypeLabel      string `json:"type_label"`
	Region         string `json:"region"`
	District       string `json:"district"`
	SousPrefecture string `json:"sous_prefecture"`
	EventDate      string `json:"event_date"`
}

// --- Carte sanitaire : normes (palier 2) ---

// NormeSet is one version of the norms referential.
type NormeSet struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Version     int    `json:"version"`
	Status      string `json:"status"` // draft | active | archived
	Notes       string `json:"notes"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
	ActivatedAt string `json:"activated_at,omitempty"`
	NRules      int    `json:"n_rules"`
}

// NormeRule is one requirement: structures of TypeCode must have at least MinValue of Target.
type NormeRule struct {
	ID       int64   `json:"id,omitempty"`
	SetID    int64   `json:"set_id,omitempty"`
	TypeCode string  `json:"type_code"` // PS | CS | … | * (all types)
	Kind     string  `json:"kind"`      // service | rh | equipement | infra
	Target   string  `json:"target"`    // DE code (service/infra), RH profile root or prefix, equipment root
	Label    string  `json:"label"`
	MinValue float64 `json:"min_value"` // service: 1 = must be 'oui'; others: minimum count
	Level    string  `json:"level"`     // essentiel | recommande
}
