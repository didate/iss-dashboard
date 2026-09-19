import { getToken } from './auth';
import type {
  Summary,
  QualitySummaryRow,
  IssueListResult,
  EventDetail,
  UsageRecensement,
  UsageService,
  UsageEquipement,
  UsageRH,
  UsageCommodite,
  PlateauItem,
  ServiceMatrixRow,
  RHSummaryResult,
  ClosedOUItem,
  Filters,
  SyncStatus,
  ReportingRate,
  MapDistrictCollection,
  StructureListResult,
  CompareResult,
  UsageGeo,
  MapGeoCollection,
  ProPointCollection,
  MissingGPSResult,
  UsageCouverture,
  NormeSet,
  NormeRule,
  NormeTargets,
  NormeLineError,
  NormesMeta,
  ConformiteSummaryRow,
  ConformiteGap,
  ConformiteStructureResult,
} from '../types';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8081/iss';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options?.headers as Record<string, string>,
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401 && path === '/api/auth/me') {
    // Only clear auth on explicit auth check failure, not on transient DB locks
    const { clearAuth } = await import('./auth');
    clearAuth();
  }

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json();
}

function qs(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') {
      parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
    }
  }
  return parts.length ? `?${parts.join('&')}` : '';
}

async function downloadAreaPDF(params: { district?: string; region?: string }) {
  const token = (await import('./auth')).getToken();
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${BASE_URL}/api/export/pdf${qs(params)}`, { headers });
  if (!res.ok) throw new Error(`Export failed: ${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `rapport_iss_${params.district ?? params.region}.pdf`;
  a.click();
  URL.revokeObjectURL(url);
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ token: string; user: { id: number; username: string; name: string; role: string } }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  // Read endpoints
  getSummary: () => request<Summary>('/api/summary'),

  getQualitySummary: (by: string) =>
    request<QualitySummaryRow[]>(`/api/quality/summary${qs({ by })}`),

  getQualityIssues: (params: {
    severity?: string;
    rule?: string;
    district?: string;
    region?: string;
    sous_prefecture?: string;
    search?: string;
    page?: number;
    pageSize?: number;
  }) => request<IssueListResult>(`/api/quality/issues${qs(params)}`),

  getEventDetail: (uid: string) =>
    request<EventDetail>(`/api/quality/event/${uid}`),

  getReportingRate: (by: string) =>
    request<ReportingRate[]>(`/api/usage/reporting${qs({ by })}`),

  getUsageRecensement: (by: string) =>
    request<UsageRecensement[]>(`/api/usage/recensement${qs({ by })}`),

  getUsageServices: (district?: string) =>
    request<UsageService[]>(`/api/usage/services${qs({ district })}`),

  getUsageEquipements: (focus?: string, district?: string) =>
    request<UsageEquipement[]>(`/api/usage/equipements${qs({ focus, district })}`),

  getUsageRH: (district?: string) =>
    request<UsageRH[]>(`/api/usage/rh${qs({ district })}`),

  getUsageCommodites: (district?: string) =>
    request<UsageCommodite[]>(`/api/usage/commodites${qs({ district })}`),

  getPlateauTechnique: (district?: string) =>
    request<PlateauItem[]>(`/api/usage/plateau${qs({ district })}`),

  getServiceMatrix: () =>
    request<ServiceMatrixRow[]>('/api/usage/services/matrix'),

  getRHSummary: (district?: string) =>
    request<RHSummaryResult>(`/api/usage/rh/summary${qs({ district })}`),

  getClosedOUs: (district?: string) =>
    request<ClosedOUItem[]>(`/api/usage/closed-ous${qs({ district })}`),

  getFilters: () => request<Filters>('/api/meta/filters'),

  getStructuresList: (params: {
    district?: string;
    sous_prefecture?: string;
    search?: string;
    type?: string;
    gps?: string;
    page?: number;
    pageSize?: number;
  }) => request<StructureListResult>(`/api/structures${qs(params)}`),

  getCompare: (districts: string[]) =>
    request<CompareResult>(`/api/compare${qs({ districts: districts.join(',') })}`),

  exportStructurePDF: async (uid: string) => {
    const token = (await import('./auth')).getToken();
    const headers: Record<string, string> = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const res = await fetch(`${BASE_URL}/api/export/pdf/structure/${uid}`, { headers });
    if (!res.ok) throw new Error(`Export failed: ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `fiche_structure_${uid}.pdf`;
    a.click();
    URL.revokeObjectURL(url);
  },

  exportDistrictPDF: async (district: string) => downloadAreaPDF({ district }),
  exportRegionPDF: async (region: string) => downloadAreaPDF({ region }),

  getMapData: () => request<MapDistrictCollection>('/api/map/districts'),

  // Carte sanitaire (espace pro)
  getMapGeo: (level: number) => request<MapGeoCollection>(`/api/map/geo${qs({ level })}`),
  getMapPoints: () => request<ProPointCollection>('/api/map/points'),
  getGeoCoverage: (level: number, region?: string) => request<UsageGeo[]>(`/api/geo/coverage${qs({ level, region })}`),
  getMissingGPS: (params: { district?: string; region?: string; type?: string; page?: number; pageSize?: number }) =>
    request<MissingGPSResult>(`/api/geo/missing${qs(params)}`),
  getCouverture: (by: string, indicator?: string) => request<UsageCouverture[]>(`/api/usage/couverture${qs({ by, indicator })}`),
  exportMissingGPSCSV: async (params: { district?: string; region?: string; type?: string }) => {
    const token = (await import('./auth')).getToken();
    const headers: Record<string, string> = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const res = await fetch(`${BASE_URL}/api/geo/missing.csv${qs(params)}`, { headers });
    if (!res.ok) throw new Error(`Export failed: ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'structures_sans_gps.csv';
    a.click();
    URL.revokeObjectURL(url);
  },

  // Admin
  triggerSync: () =>
    request<{ status: string; message: string }>('/api/admin/sync', { method: 'POST' }),

  getSyncStatus: () =>
    request<SyncStatus>('/api/admin/sync/status'),

  getUsers: () =>
    request<{ id: number; username: string; name: string; role: string }[]>('/api/admin/users'),

  createUser: (data: { username: string; password: string; name: string; role: string }) =>
    request<{ id: number; username: string; name: string; role: string }>('/iss/api/admin/users', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  deleteUser: (id: number) =>
    request<{ message: string }>(`/api/admin/users/${id}`, { method: 'DELETE' }),

  // Normes (admin)
  getNormeSets: () => request<NormeSet[]>('/api/admin/normes'),
  createNormeSet: (data: { name: string; notes: string }) =>
    request<NormeSet>('/api/admin/normes', { method: 'POST', body: JSON.stringify(data) }),
  updateNormeSet: (id: number, data: { name: string; notes: string }) =>
    request<NormeSet>(`/api/admin/normes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  duplicateNormeSet: (id: number) => request<NormeSet>(`/api/admin/normes/${id}/duplicate`, { method: 'POST' }),
  activateNormeSet: (id: number) => request<NormeSet>(`/api/admin/normes/${id}/activate`, { method: 'POST' }),
  deleteNormeSet: (id: number) => request<{ deleted: number }>(`/api/admin/normes/${id}`, { method: 'DELETE' }),
  getNormeRules: (id: number) => request<NormeRule[]>(`/api/admin/normes/${id}/rules`),
  putNormeRules: (id: number, rules: NormeRule[]) =>
    request<NormeRule[]>(`/api/admin/normes/${id}/rules`, { method: 'PUT', body: JSON.stringify(rules) }),
  getNormeTargets: () => request<NormeTargets>('/api/admin/normes/targets'),
  recomputeConformite: () => request<{ status: string }>('/api/admin/normes/recompute', { method: 'POST' }),
  importNormeRules: async (id: number, file: File, mode: 'replace' | 'append' = 'replace') => {
    const token = (await import('./auth')).getToken();
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`${BASE_URL}/api/admin/normes/${id}/rules/import?mode=${mode}`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: form,
    });
    const body = (await res.json()) as { imported?: number; total?: number; error?: string; errors?: NormeLineError[]; valid?: number };
    if (!res.ok) {
      const err = new Error(body.error || `Import failed: ${res.status}`) as Error & { errors?: NormeLineError[] };
      err.errors = body.errors;
      throw err;
    }
    return body as { imported: number; total: number; errors: NormeLineError[] | null };
  },
  exportNormeRulesCSV: async (id: number, version: number) => {
    const token = (await import('./auth')).getToken();
    const res = await fetch(`${BASE_URL}/api/admin/normes/${id}/rules/export.csv`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) throw new Error(`Export failed: ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `normes_v${version}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  },

  // Conformité (lecture)
  getNormesMeta: () => request<NormesMeta>('/api/meta/normes'),
  getConformiteSummary: (by: string, type?: string) =>
    request<ConformiteSummaryRow[]>(`/api/conformite/summary${qs({ by, type })}`),
  getConformiteGaps: (params: { by?: string; key?: string; type?: string; kind?: string; level?: string; limit?: number }) =>
    request<ConformiteGap[]>(`/api/conformite/gaps${qs(params)}`),
  getConformiteStructures: (params: {
    region?: string; district?: string; sous_prefecture?: string; type?: string; status?: string; search?: string; page?: number; pageSize?: number;
  }) => request<ConformiteStructureResult>(`/api/conformite/structures${qs(params)}`),
};
