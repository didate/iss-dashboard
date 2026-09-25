import { httpError } from './errors';
import type {
  PublicAnnuaireResult,
  PublicFilters,
  PublicPointCollection,
  PublicStructure,
  PublicStructureItem,
  PublicSummary,
} from '../types';

// Client de l'espace public : aucun jeton, aucun en-tête d'auth. Les réponses
// sont mises en cache par le navigateur (Cache-Control / ETag côté serveur).
const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8081/iss';

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`);
  if (!res.ok) throw await httpError(res);
  return res.json();
}

function qs(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
  }
  return parts.length ? `?${parts.join('&')}` : '';
}

export interface PublicSearchParams {
  search?: string;
  type?: string;
  service?: string;
  district?: string;
  region?: string;
  sous_prefecture?: string;
  near?: string; // "lat,lng"
  radius_km?: number;
  limit?: number;
}

export const publicApi = {
  getPoints: () => get<PublicPointCollection>('/api/public/points.geojson'),
  getFilters: () => get<PublicFilters>('/api/public/filters'),
  getSummary: () => get<PublicSummary>('/api/public/summary'),
  search: (params: PublicSearchParams) => get<PublicStructureItem[]>(`/api/public/structures${qs({ ...params })}`),
  getStructure: (uid: string) => get<PublicStructure>(`/api/public/structure/${encodeURIComponent(uid)}`),
  getAnnuaire: (params: PublicSearchParams & { page?: number; pageSize?: number }) =>
    get<PublicAnnuaireResult>(`/api/public/annuaire${qs({ ...params })}`),
  // Lien direct (pas de fetch) : le navigateur télécharge le CSV avec les mêmes filtres.
  annuaireCSVUrl: (params: PublicSearchParams) => `${BASE_URL}/api/public/structures.csv${qs({ ...params })}`,
};

// Couleur par type de structure (affichage uniquement).
export function typeColor(type: string): string {
  switch (type) {
    case 'HN':
    case 'HR':
    case 'HP':
      return '#dc2626';
    case 'CMC':
    case 'CSA':
      return '#ea580c';
    case 'CS':
      return '#2563eb';
    case 'PS':
      return '#16a34a';
    case 'CABINET':
    case 'CLINIQUE':
    case 'AUTRE_PRIVE':
      return '#7c3aed';
    default:
      return '#6b7280';
  }
}

export function opLabel(op: string): string {
  switch (op) {
    case 'operationnel':
      return 'Opérationnelle';
    case 'non_operationnel':
      return 'Non opérationnelle';
    case 'ferme_temporairement':
      return 'Fermée temporairement';
    default:
      return 'Statut inconnu';
  }
}

export function opColor(op: string): string {
  switch (op) {
    case 'operationnel':
      return 'bg-green-100 text-green-800';
    case 'non_operationnel':
      return 'bg-red-100 text-red-800';
    case 'ferme_temporairement':
      return 'bg-amber-100 text-amber-800';
    default:
      return 'bg-gray-100 text-gray-600';
  }
}
