export interface SyncRun {
  id: number;
  started_at: string;
  finished_at?: string;
  status: string;
  events_pulled: number;
  issues_found: number;
  duration_ms: number;
  error_text?: string;
}

export interface Summary {
  n_structures: number;
  n_operationnel: number;
  avg_score: number;
  n_error: number;
  n_warning: number;
  n_info: number;
  last_sync: SyncRun | null;
}

export interface QualitySummaryRow {
  dimension: string;
  key: string;
  label: string;
  avg_score: number;
  n_error: number;
  n_warning: number;
  n_info: number;
  n_structures: number;
}

export interface Issue {
  rule_code: string;
  severity: string;
  rule_name: string;
  message: string;
}

export interface IssueListItem {
  event_uid: string;
  org_unit_name: string;
  district: string;
  region: string;
  worst_severity: string;
  score: number;
  n_error: number;
  n_warning: number;
  n_info: number;
  issues: Issue[];
}

export interface IssueListResult {
  data: IssueListItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface EventValueDisplay {
  de_code: string;
  de_name: string;
  value: string;
  section_prefix: string;
}

export interface StructureListItem {
  event_uid: string;
  org_unit_uid: string;
  org_unit_name: string;
  district: string;
  region: string;
  event_date: string;
  status: string;
  type_code: string;
  type_label: string;
  has_gps: boolean;
  score: number;
  n_error: number;
  n_warning: number;
  n_info: number;
}

export interface StructureListResult {
  data: StructureListItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface EventQuality {
  event_uid: string;
  n_error: number;
  n_warning: number;
  n_info: number;
  worst_severity: string;
  score: number;
}

export interface EventDetail {
  // Clés JSON de models.Event côté backend (camelCase DHIS2)
  event: {
    event: string;
    orgUnit: string;
    orgUnitName: string;
    district: string;
    region: string;
    eventDate: string;
    status: string;
    districtUid?: string;
    sousPrefecture?: string;
    sousPrefectureUid?: string;
    typeCode?: string;
    typeSource?: string;
    lat?: number;
    lng?: number;
  };
  values: EventValueDisplay[];
  issues: Issue[];
  quality: EventQuality;
}

export interface UsageRecensement {
  dimension: string;
  key: string;
  label: string;
  n_structures: number;
  n_operationnel: number;
  n_non_operationnel: number;
  n_ferme_temp: number;
}

export interface UsageService {
  service_code: string;
  service_label: string;
  district: string;
  n_oui: number;
  n_oui_pas_fonc: number;
  n_non: number;
  n_total: number;
  pct_fonctionnel: number;
}

export interface UsageEquipement {
  equip_root: string;
  label: string;
  district: string;
  sum_total: number;
  sum_fonct: number;
  pct_fonct: number;
  category: string;
}

export interface UsageRH {
  profil_code: string;
  label: string;
  district: string;
  effectif_fonc: number;
  effectif_contr: number;
  effectif_benev: number;
  effectif_asc: number;
  effectif_reco: number;
  effectif_total: number;
}

export interface UsageCommodite {
  indicator: string;
  district: string;
  n_oui: number;
  n_total: number;
  pct: number;
}

export interface ReportingRate {
  dimension: string;
  key: string;
  label: string;
  n_expected: number;
  n_reported: number;
  pct: number;
}

export interface PlateauItem {
  service_code: string;
  service_label: string;
  n_oui: number;
  n_total: number;
  pct: number;
}

export interface ServiceMatrixRow {
  service_code: string;
  service_label: string;
  districts: Record<string, number>;
  overall: number;
}

export interface RHSummaryResult {
  total_effectif: number;
  total_fonc: number;
  total_contr: number;
  total_benev: number;
  total_asc: number;
  total_reco: number;
  n_structures: number;
  ratio_med_per_structure: number;
  n_structures_sans_medecin: number;
  pct_structures_sans_medecin: number;
}

export interface ClosedOUItem {
  uid: string;
  name: string;
  closed_date: string;
  district: string;
  region: string;
  has_data: boolean;
}

export interface RuleInfo {
  code: string;
  name: string;
}

export interface Filters {
  districts: string[];
  regions: string[];
  district_regions: Record<string, string>;
  district_uids: Record<string, string>;
  rules: RuleInfo[];
  services: string[];
  statuts: string[];
  types: PublicFilterType[];
  sous_prefectures: { name: string; district: string }[];
}

export interface SyncStatus {
  current: SyncRun | null;
  last: SyncRun | null;
  history: SyncRun[];
}

// --- Compare types ---

export interface CompareDistrictData {
  name: string;
  avg_score: number;
  n_structures: number;
  reporting_pct: number;
  reporting_expected: number;
  reporting_reported: number;
  services: UsageService[];
  equipements: UsageEquipement[];
  rh: UsageRH[];
  rh_summary: RHSummaryResult;
  commodites: UsageCommodite[];
}

export interface CompareResult {
  districts: CompareDistrictData[];
  national: CompareDistrictData;
}

// --- Map / Carte types ---

export interface ServiceMapData {
  service_label: string;
  pct_fonctionnel: number;
  n_oui: number;
  n_total: number;
}

export interface EquipMapData {
  label: string;
  category: string;
  sum_total: number;
  sum_fonct: number;
}

export interface RhMapData {
  label: string;
  effectif_total: number;
}

export interface MapDistrictProperties {
  district_uid: string;
  district_name: string;
  rapportage_pct: number | null;
  rapportage_expected: number;
  rapportage_reported: number;
  qualite_avg_score: number | null;
  qualite_n_structures: number;
  services: Record<string, ServiceMapData>;
  equipements: Record<string, EquipMapData>;
  wash_forage_ou_reseau_pct: number | null;
  wash_forage_ou_reseau_n: number;
  wash_total: number;
  wash_eau_pts_critiques_pct: number | null;
  wash_eau_pts_critiques_n: number;
  rh_medecins_total: number;
  rh_n_structures: number;
  rh_medecins_par_structure: number | null;
  rh: Record<string, RhMapData>;
}

export interface MapDistrictFeature {
  type: 'Feature';
  geometry: GeoJSON.Geometry;
  properties: MapDistrictProperties;
}

export interface MapDistrictCollection {
  type: 'FeatureCollection';
  features: MapDistrictFeature[];
}

// --- Carte sanitaire : espace public (projections réduites servies par /api/public/*) ---

export interface PublicService {
  code: string;
  label: string;
}

export interface PublicPointProperties {
  uid: string;
  name: string;
  type: string;
  type_label: string;
  statut: string;
  op: string;
  region: string;
  district: string;
  sp: string;
  svc: string[] | null;
}

export interface PublicPointFeature {
  type: 'Feature';
  geometry: { type: 'Point'; coordinates: [number, number] };
  properties: PublicPointProperties;
}

export interface PublicPointCollection {
  type: 'FeatureCollection';
  features: PublicPointFeature[];
}

export interface PublicFilterType {
  code: string;
  label: string;
  n: number;
}

export interface PublicFilters {
  types: PublicFilterType[];
  services: PublicService[];
  regions: string[];
  districts: { name: string; region: string }[];
}

export interface PublicStructure {
  uid: string;
  name: string;
  type: string;
  type_label: string;
  statut: string;
  statut_detail: string;
  op: string;
  region: string;
  district: string;
  sous_prefecture: string;
  lat: number | null;
  lng: number | null;
  services: PublicService[];
  plateau: Record<string, boolean>;
  recense_le: string;
}

export interface PublicStructureItem {
  uid: string;
  name: string;
  type: string;
  type_label: string;
  op: string;
  region: string;
  district: string;
  lat: number | null;
  lng: number | null;
  distance_km?: number;
}

export interface PublicSummary {
  n_structures: number;
  n_par_type: Record<string, number>;
  pct_gps: number;
  derniere_synchro: string;
}

// --- Carte sanitaire : espace pro (géo, couverture) ---

export interface UsageGeo {
  level: number;
  ou_uid: string;
  name: string;
  parent_name: string;
  n_structures: number;
  n_gps: number;
  pct_gps: number | null;
  avg_score: number | null;
  population: number | null;
  n_par_type: Record<string, number>;
}

export interface MapGeoFeature {
  type: 'Feature';
  geometry: GeoJSON.Geometry;
  properties: UsageGeo & {
    ratio_structures_10k: number | null;
    ratios: Record<string, number | null>; // /10 000 hab. par indicateur de couverture
    numerators: Record<string, number>;
  };
}

export interface MapGeoCollection {
  type: 'FeatureCollection';
  features: MapGeoFeature[];
}

export interface ProPointProperties {
  event_uid: string;
  uid: string;
  name: string;
  type: string;
  type_label: string;
  region: string;
  district: string;
  score: number;
  worst_severity: string;
  n_issues: number;
}

export interface ProPointCollection {
  type: 'FeatureCollection';
  features: { type: 'Feature'; geometry: { type: 'Point'; coordinates: [number, number] }; properties: ProPointProperties }[];
}

export interface MissingGPSItem {
  event_uid: string;
  org_unit_uid: string;
  name: string;
  type: string;
  type_label: string;
  region: string;
  district: string;
  sous_prefecture: string;
  event_date: string;
}

export interface MissingGPSResult {
  data: MissingGPSItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface UsageCouverture {
  dimension: string;
  key: string;
  ou_uid: string;
  label: string;
  indicator: string;
  numerator: number;
  population: number | null;
  ratio_10k: number | null;
}

export interface PublicAnnuaireRow {
  uid: string;
  name: string;
  type: string;
  type_label: string;
  statut: string;
  op: string;
  region: string;
  district: string;
  sous_prefecture: string;
  lat: number | null;
  lng: number | null;
  n_services: number;
}

export interface PublicAnnuaireResult {
  data: PublicAnnuaireRow[];
  total: number;
  page: number;
  page_size: number;
}
