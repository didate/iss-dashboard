import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { StructureListResult, Filters } from '../types';
import ScoreBar from '../components/ScoreBar';
import SeverityBadge from '../components/SeverityBadge';

export default function Structures() {
  const navigate = useNavigate();
  const [result, setResult] = useState<StructureListResult | null>(null);
  const [filters, setFilters] = useState<Filters | null>(null);
  const [loading, setLoading] = useState(true);

  const [district, setDistrict] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
  }, []);

  const fetchData = useCallback(() => {
    setLoading(true);
    api.getStructuresList({ district, search, page, pageSize: 25 })
      .then(setResult)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [district, search, page]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const items = result?.data ?? [];
  const totalPages = result ? Math.ceil(result.total / result.page_size) : 0;

  const worstSeverity = (item: typeof items[0]) => {
    if (item.n_error > 0) return 'error';
    if (item.n_warning > 0) return 'warning';
    if (item.n_info > 0) return 'info';
    return '';
  };

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Structures sanitaires</h2>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 bg-white p-3 rounded-lg border border-gray-200 items-center">
        <select
          className="border border-gray-300 rounded px-2 py-1.5 text-sm"
          value={district}
          onChange={(e) => { setDistrict(e.target.value); setPage(1); }}
        >
          <option value="">Tous districts</option>
          {filters?.districts.map((d) => (
            <option key={d} value={d}>{d}</option>
          ))}
        </select>

        <input
          type="text"
          placeholder="Rechercher par nom..."
          className="border border-gray-300 rounded px-2 py-1.5 text-sm flex-1 min-w-0 w-full sm:w-auto"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1); }}
        />

        {result && (
          <span className="text-xs text-gray-500">{result.total} structures</span>
        )}
      </div>

      {/* Table */}
      <div className="bg-white rounded-lg border border-gray-200">
        {loading ? (
          <div className="p-8 text-gray-400 text-center">Chargement...</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b bg-gray-50">
                    <th className="text-left px-3 py-2 font-medium text-gray-500">Structure</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-500">District</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-500 hidden sm:table-cell">Region</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-500 hidden md:table-cell">Date</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-500">Score</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-500">Severite</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-500 hidden sm:table-cell">E/W/I</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => (
                    <tr
                      key={item.event_uid}
                      className="border-b border-gray-100 cursor-pointer hover:bg-blue-50"
                      onClick={() => navigate(`/structure/${item.event_uid}`)}
                    >
                      <td className="px-3 py-2 font-medium text-gray-800">{item.org_unit_name}</td>
                      <td className="px-3 py-2 text-gray-600">{item.district}</td>
                      <td className="px-3 py-2 text-gray-600 hidden sm:table-cell">{item.region}</td>
                      <td className="px-3 py-2 text-gray-500 hidden md:table-cell">{item.event_date?.slice(0, 10)}</td>
                      <td className="px-3 py-2"><ScoreBar score={item.score} /></td>
                      <td className="px-3 py-2">
                        {worstSeverity(item) && <SeverityBadge severity={worstSeverity(item)} />}
                      </td>
                      <td className="px-3 py-2 text-xs hidden sm:table-cell">
                        <span className="text-red-600">{item.n_error}</span>
                        {' / '}
                        <span className="text-yellow-600">{item.n_warning}</span>
                        {' / '}
                        <span className="text-blue-600">{item.n_info}</span>
                      </td>
                    </tr>
                  ))}
                  {items.length === 0 && (
                    <tr>
                      <td colSpan={7} className="px-3 py-8 text-center text-gray-400">
                        Aucune structure trouvee
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {totalPages > 1 && (
              <div className="flex items-center justify-between px-3 py-2 border-t">
                <span className="text-xs text-gray-500">
                  {result?.total} resultats — page {page}/{totalPages}
                </span>
                <div className="flex gap-1">
                  <button
                    className="px-2 py-1 text-xs border rounded disabled:opacity-40"
                    disabled={page <= 1}
                    onClick={() => setPage(page - 1)}
                  >Prec.</button>
                  <button
                    className="px-2 py-1 text-xs border rounded disabled:opacity-40"
                    disabled={page >= totalPages}
                    onClick={() => setPage(page + 1)}
                  >Suiv.</button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
