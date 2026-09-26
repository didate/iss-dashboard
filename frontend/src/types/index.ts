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
  conformite?: EventConformite | null;
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

export interface PublicExtras {
  niveau: number;
  rh_total: number | null;
  rh_medecins: number | null;
  rh_soignants: number | null;
  eau: boolean | null;
  energie: boolean | null;
  score_services: number | null;
  score_services_max: number;
}

export interface PublicPointProperties extends PublicExtras {
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

export interface PublicStructure extends PublicExtras {
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
  dashboard_public: boolean;
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
    conformite_score: number | null;
    pct_conformes: number | null;
    drh_ratio_10k: number | null;
    drh_depart_5ans_pct: number | null;
    drh_part_etat_pct: number | null;
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

// --- Normes et conformité (palier 2) ---

export interface NormeSet {
  id: number;
  name: string;
  version: number;
  status: 'draft' | 'active' | 'archived';
  notes: string;
  created_at: string;
  created_by: string;
  activated_at?: string;
  n_rules: number;
}

export interface NormeRule {
  id?: number;
  set_id?: number;
  type_code: string;
  kind: string; // service | rh | equipement | infra
  target: string;
  label: string;
  min_value: number;
  level: string; // essentiel | recommande
}

export interface NormeTarget {
  kind: string;
  code: string;
  label: string;
  prefix?: boolean;
}

export interface NormeTargets {
  targets: NormeTarget[];
  kinds: string[];
  levels: string[];
}

export interface NormeLineError {
  line: number;
  message: string;
}

export interface ConformiteRun {
  set_id: number;
  set_version: number;
  n_rules: number;
  n_structures: number;
  n_evaluees: number;
  n_conformes: number;
  computed_at: string;
}

export interface NormesMeta {
  active: NormeSet | null;
  last_run: ConformiteRun | null;
}

export interface ConformiteSummaryRow {
  dimension: string;
  key: string;
  label: string;
  type_code: string;
  n_structures: number;
  n_evaluees: number;
  avg_score: number | null;
  n_conformes: number;
  pct_conformes: number | null;
}

export interface ConformiteGap {
  dimension: string;
  key: string;
  type_code: string;
  kind: string;
  target: string;
  label: string;
  level: string;
  n_concernees: number;
  n_manque: number;
  n_inconnu: number;
  deficit: number;
}

export interface ConformiteStructureItem {
  event_uid: string;
  org_unit_uid: string;
  name: string;
  type_code: string;
  region: string;
  district: string;
  sous_prefecture: string;
  score: number | null;
  conforme: boolean;
  n_manque: number;
  n_manque_essentiel: number;
  n_inconnu: number;
  n_rules: number;
}

export interface ConformiteStructureResult {
  data: ConformiteStructureItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface ConformiteItem {
  rule_id: number;
  type_code: string;
  kind: string;
  target: string;
  label: string;
  level: string;
  expected: number;
  observed: number | null;
  status: 'ok' | 'manque' | 'inconnu';
}

export interface EventConformite {
  summary: {
    n_rules: number;
    n_ok: number;
    n_manque: number;
    n_inconnu: number;
    n_manque_essentiel: number;
    score: number | null;
    conforme: boolean;
  };
  items: ConformiteItem[];
}

// --- Personnel de l'État (DRH/CNPS) ---

export interface DrhImport {
  id: number;
  label: string;
  annee: number;
  status: 'active' | 'archived';
  age_retraite: number;
  n_agents: number;
  n_structure: number;
  n_bureau: number;
  n_bureau_regional: number;
  n_centrale: number;
  n_non_rattache: number;
  n_structures: number;
  imported_at: string;
  imported_by: string;
  source_file: string;
}

export interface DrhInconnu {
  libelle: string;
  prefecture: string;
  n_agents: number;
}

export interface DrhReport {
  n_agents: number;
  n_structure: number;
  n_bureau: number;
  n_bureau_regional: number;
  n_centrale: number;
  n_non_rattache: number;
  n_structures_couvertes: number;
  par_source: Record<string, number>;
  inconnus: DrhInconnu[] | null;
}

export interface DrhImportResult {
  import: DrhImport;
  report: DrhReport;
  duree_ms: number;
}

export interface DrhCorrespondance {
  libelle_norm: string;
  libelle_drh: string;
  org_unit_uid: string;
  statut: 'ok' | 'bureau_district' | 'non_rattache' | 'a_trancher';
  district: string;
}

export interface DrhCategorie {
  code: string;
  label: string;
  famille: 'soignant' | 'technique' | 'support';
  iss?: string;
}

export interface DrhEffectif {
  dimension: string;
  key: string;
  label: string;
  district: string;
  region: string;
  categorie: string;
  n_agents: number;
  n_femmes: number;
  n_depart_5ans: number;
  n_depart_10ans: number;
  n_age_connu: number;
  n_structure: number;
  n_bureau: number;
  n_bureau_regional: number;
  n_centrale: number;
  n_non_rattache: number;
  population?: number | null;
  ratio_10k?: number | null;
}

export interface DrhPyramide {
  dimension: string;
  key: string;
  categorie: string;
  tranche: string;
  n_agents: number;
  n_femmes: number;
}

export interface DrhComparaison {
  dimension: string;
  key: string;
  label: string;
  categorie: string;
  n_drh: number;
  n_iss?: number | null;
  ecart?: number | null;
  ratio?: number | null;
  /** Les deux nomenclatures se recouvrent : le rapport DRH/ISS est alors une part. */
  aligne: boolean;
  /** 100 × DRH ÷ ISS, renseignée pour les seules catégories alignées. */
  part_etat?: number | null;
}

export interface DrhStructureRow {
  /** Vide quand la structure n'a jamais été recensée : il n'y a pas de fiche. */
  event_uid: string;
  hors_recensement: boolean;
  org_unit_uid: string;
  name: string;
  type_code: string;
  district: string;
  region: string;
  n_agents: number;
  n_femmes: number;
  n_depart_5ans: number;
}

export interface DrhSummary {
  import: DrhImport;
  national: DrhEffectif | null;
  categories: DrhEffectif[];
  catalogue: DrhCategorie[];
  tranches: string[];
}
