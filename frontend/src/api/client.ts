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
  Filters,
  SyncStatus,
  ReportingRate,
} from '../types';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

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

  if (res.status === 401) {
    // Token expired or invalid
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

  getFilters: () => request<Filters>('/api/meta/filters'),

  // Admin
  triggerSync: () =>
    request<{ status: string; message: string }>('/api/admin/sync', { method: 'POST' }),

  getSyncStatus: () =>
    request<SyncStatus>('/api/admin/sync/status'),

  getUsers: () =>
    request<{ id: number; username: string; name: string; role: string }[]>('/api/admin/users'),

  createUser: (data: { username: string; password: string; name: string; role: string }) =>
    request<{ id: number; username: string; name: string; role: string }>('/api/admin/users', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  deleteUser: (id: number) =>
    request<{ message: string }>(`/api/admin/users/${id}`, { method: 'DELETE' }),
};
