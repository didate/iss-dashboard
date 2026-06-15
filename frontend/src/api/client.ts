const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json();
}

function adminHeaders(token: string): Record<string, string> {
  return { 'X-Admin-Token': token };
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
  Filters,
  SyncStatus,
} from '../types';

export const api = {
  getSummary: () => request<Summary>('/api/summary'),

  getQualitySummary: (by: string) =>
    request<QualitySummaryRow[]>(`/api/quality/summary${qs({ by })}`),

  getQualityIssues: (params: {
    severity?: string;
    rule?: string;
    district?: string;
    search?: string;
    page?: number;
    pageSize?: number;
  }) => request<IssueListResult>(`/api/quality/issues${qs(params)}`),

  getEventDetail: (uid: string) =>
    request<EventDetail>(`/api/quality/event/${uid}`),

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

  getFilters: () => request<Filters>('/api/meta/filters'),

  triggerSync: (token: string) =>
    request<{ status: string; message: string }>('/api/admin/sync', {
      method: 'POST',
      headers: adminHeaders(token),
    }),

  getSyncStatus: (token: string) =>
    request<SyncStatus>('/api/admin/sync/status', {
      headers: adminHeaders(token),
    }),
};
