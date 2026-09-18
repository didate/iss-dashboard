import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Download, MapPin, MapPinOff, Search } from 'lucide-react';
import { publicApi, typeColor, opLabel, opColor } from '../../api/public';
import { useUrlState, useUrlStateInt } from '../../hooks/useUrlState';
import type { PublicAnnuaireResult, PublicFilters } from '../../types';

const PAGE_SIZE = 50;

function useDebounced(value: string, ms: number): string {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

// Annuaire public : le registre des structures en tableau, avec les mêmes
// filtres que la carte, paginé côté serveur, exportable en CSV (open data).
export default function Annuaire() {
  const [search, setSearch] = useUrlState('q');
  const [type, setType] = useUrlState('type');
  const [service, setService] = useUrlState('service');
  const [region, setRegion] = useUrlState('region');
  const [district, setDistrict] = useUrlState('district');
  const [page, setPage] = useUrlStateInt('page', 1);
  const debouncedSearch = useDebounced(search, 300);

  const [filters, setFilters] = useState<PublicFilters | null>(null);
  const [result, setResult] = useState<PublicAnnuaireResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    publicApi.getFilters().then(setFilters).catch((e) => setError(e.message));
  }, []);

  const params = useMemo(
    () => ({ search: debouncedSearch, type, service, region, district }),
    [debouncedSearch, type, service, region, district],
  );

  useEffect(() => {
    setLoading(true);
    publicApi
      .getAnnuaire({ ...params, page, pageSize: PAGE_SIZE })
      .then(setResult)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [params, page]);

  const districtsOfRegion = filters ? filters.districts.filter((d) => !region || d.region === region) : [];
  const totalPages = result ? Math.ceil(result.total / result.page_size) : 0;
  const selectCls = 'text-sm border border-gray-300 rounded-md px-2 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-emerald-500';
  const reset = () => setPage(1);

  return (
    <div className="max-w-6xl mx-auto p-4 sm:p-6 space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Annuaire des structures de santé</h1>
          <p className="text-sm text-gray-500">
            {result ? `${result.total.toLocaleString('fr-FR')} structure${result.total > 1 ? 's' : ''}` : '…'} — registre issu du recensement ISS (DHIS2)
          </p>
        </div>
        <a
          href={publicApi.annuaireCSVUrl(params)}
          className="inline-flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50"
          title="Télécharger la liste filtrée (CSV, ouvrable dans Excel)"
        >
          <Download size={14} /> Télécharger (CSV)
        </a>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-5 gap-2">
        <div className="relative col-span-2 md:col-span-1">
          <Search size={15} className="absolute left-2.5 top-2.5 text-gray-400" />
          <input
            type="search"
            value={search}
            onChange={(e) => { setSearch(e.target.value); reset(); }}
            placeholder="Nom…"
            className={`${selectCls} w-full pl-8`}
          />
        </div>
        <select value={type} onChange={(e) => { setType(e.target.value); reset(); }} className={selectCls}>
          <option value="">Tous les types</option>
          {filters?.types.map((t) => <option key={t.code} value={t.code}>{t.label} ({t.n})</option>)}
        </select>
        <select value={service} onChange={(e) => { setService(e.target.value); reset(); }} className={selectCls}>
          <option value="">Tous les services</option>
          {filters?.services.map((s) => <option key={s.code} value={s.code}>{s.label}</option>)}
        </select>
        <select value={region} onChange={(e) => { setRegion(e.target.value); setDistrict(''); reset(); }} className={selectCls}>
          <option value="">Toutes les régions</option>
          {filters?.regions.map((r) => <option key={r} value={r}>{r}</option>)}
        </select>
        <select value={district} onChange={(e) => { setDistrict(e.target.value); reset(); }} className={selectCls}>
          <option value="">Tous les districts</option>
          {districtsOfRegion.map((d) => <option key={d.name} value={d.name}>{d.name}</option>)}
        </select>
      </div>

      {error && <div className="text-sm text-red-700 bg-red-50 border border-red-100 rounded px-3 py-2">{error}</div>}

      <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 bg-gray-50 text-left text-xs text-gray-500">
                <th className="px-3 py-2 font-medium">Structure</th>
                <th className="px-3 py-2 font-medium hidden md:table-cell">Type</th>
                <th className="px-3 py-2 font-medium">District</th>
                <th className="px-3 py-2 font-medium hidden lg:table-cell">Sous-préfecture</th>
                <th className="px-3 py-2 font-medium hidden sm:table-cell">Statut</th>
                <th className="px-3 py-2 font-medium text-right hidden sm:table-cell">Services</th>
                <th className="px-3 py-2 font-medium text-center">GPS</th>
              </tr>
            </thead>
            <tbody>
              {(result?.data ?? []).map((r) => (
                <tr key={r.uid} className="border-b border-gray-100 hover:bg-emerald-50/40">
                  <td className="px-3 py-2">
                    <Link to={`/fs/${r.uid}`} className="font-medium text-gray-900 hover:text-emerald-700">
                      <span className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle" style={{ background: typeColor(r.type) }} />
                      {r.name}
                    </Link>
                    <div className="text-xs text-gray-500 md:hidden">{r.type_label}</div>
                  </td>
                  <td className="px-3 py-2 text-gray-600 hidden md:table-cell">{r.type_label}</td>
                  <td className="px-3 py-2 text-gray-600">
                    {r.district}
                    <div className="text-xs text-gray-400">{r.region}</div>
                  </td>
                  <td className="px-3 py-2 text-gray-600 hidden lg:table-cell">{r.sous_prefecture}</td>
                  <td className="px-3 py-2 hidden sm:table-cell">
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${opColor(r.op)}`}>{opLabel(r.op)}</span>
                    {r.statut && <span className="ml-1 text-xs text-gray-500 capitalize">{r.statut}</span>}
                  </td>
                  <td className="px-3 py-2 text-right text-gray-700 hidden sm:table-cell">{r.n_services}</td>
                  <td className="px-3 py-2 text-center">
                    {r.lat !== null ? (
                      <MapPin size={14} className="inline text-emerald-600" aria-label="Géolocalisée" />
                    ) : (
                      <MapPinOff size={14} className="inline text-gray-300" aria-label="Non géolocalisée" />
                    )}
                  </td>
                </tr>
              ))}
              {!loading && (result?.data.length ?? 0) === 0 && (
                <tr>
                  <td colSpan={7} className="px-3 py-8 text-center text-gray-400">Aucune structure ne correspond.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-3 py-2 border-t border-gray-100 text-xs text-gray-500">
            <span>Page {page} / {totalPages}</span>
            <div className="flex gap-1">
              <button className="px-2 py-1 border rounded disabled:opacity-40" disabled={page <= 1} onClick={() => setPage(page - 1)}>Précédent</button>
              <button className="px-2 py-1 border rounded disabled:opacity-40" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>Suivant</button>
            </div>
          </div>
        )}
      </div>

      <p className="text-xs text-gray-400">
        Données ouvertes : le CSV contient l'identifiant DHIS2, le nom, le type, les statuts, le rattachement, les coordonnées et le nombre de services fonctionnels de chaque structure. Une structure est listée une fois (dernier recensement).
      </p>
    </div>
  );
}
