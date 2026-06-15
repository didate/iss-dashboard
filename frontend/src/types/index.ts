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
  event: {
    event_uid: string;
    org_unit_uid: string;
    org_unit_name: string;
    district: string;
    region: string;
    event_date: string;
    status: string;
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
  effectif_total: number;
}

export interface UsageCommodite {
  indicator: string;
  district: string;
  n_oui: number;
  n_total: number;
  pct: number;
}

export interface Filters {
  districts: string[];
  regions: string[];
  rules: string[];
  services: string[];
  statuts: string[];
}

export interface SyncStatus {
  current: SyncRun | null;
  last: SyncRun | null;
  history: SyncRun[];
}
