import { useEffect, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend, ReferenceLine } from 'recharts';
import { api } from '../api/client';
import type { CompareResult, Filters } from '../types';
import MethodNote from '../components/MethodNote';

export default function Comparison() {
  const [filters, setFilters] = useState<Filters | null>(null);
  const [d1, setD1] = useState('');
  const [d2, setD2] = useState('');
  const [result, setResult] = useState<CompareResult | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
  }, []);

  const compare = () => {
    if (!d1 || !d2) return;
    setLoading(true);
    api.getCompare(d1, d2)
      .then(setResult)
      .catch(console.error)
      .finally(() => setLoading(false));
  };

  const colorScore = (v: number) => v >= 80 ? 'text-green-600' : v >= 50 ? 'text-yellow-600' : 'text-red-600';
  const colorPct = (v: number) => v >= 80 ? 'text-green-600' : v >= 50 ? 'text-yellow-600' : 'text-red-600';

  // Build services comparison chart data
  const servicesChartData = result ? (() => {
    const allCodes = new Set<string>();
    result.district1.services.forEach(s => allCodes.add(s.service_code));
    result.district2.services.forEach(s => allCodes.add(s.service_code));
    const d1Map = new Map(result.district1.services.map(s => [s.service_code, s]));
    const d2Map = new Map(result.district2.services.map(s => [s.service_code, s]));
    const natMap = new Map(result.national.services.map(s => [s.service_code, s]));
    return Array.from(allCodes).map(code => ({
      name: d1Map.get(code)?.service_label || d2Map.get(code)?.service_label || code,
      [result.district1.name]: d1Map.get(code)?.n_oui ?? 0,
      [result.district2.name]: d2Map.get(code)?.n_oui ?? 0,
      national: natMap.get(code)?.n_oui ?? 0,
    })).sort((a, b) => a.name.localeCompare(b.name));
  })() : [];

  // Build commodites comparison
  const commoditesData = result ? (() => {
    const labels: Record<string, string> = {
      energie: 'Energie', eau_pts_critiques: 'Eau pts critiques',
      energie_solaire: 'Solaire', energie_reseau: 'Reseau elec.', energie_generateur: 'Generateur',
    };
    const allInds = new Set<string>();
    result.district1.commodites.filter(c => !c.indicator.startsWith('source_eau_')).forEach(c => allInds.add(c.indicator));
    result.district2.commodites.filter(c => !c.indicator.startsWith('source_eau_')).forEach(c => allInds.add(c.indicator));
    const d1Map = new Map(result.district1.commodites.map(c => [c.indicator, c]));
    const d2Map = new Map(result.district2.commodites.map(c => [c.indicator, c]));
    const natMap = new Map(result.national.commodites.map(c => [c.indicator, c]));
    return Array.from(allInds).map(ind => ({
      name: labels[ind] || ind,
      [result.district1.name]: d1Map.get(ind)?.pct ?? 0,
      [result.district2.name]: d2Map.get(ind)?.pct ?? 0,
      national: natMap.get(ind)?.pct ?? 0,
    }));
  })() : [];

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Comparaison de districts</h2>

      {/* Selectors */}
      <div className="flex flex-wrap gap-3 bg-white p-4 rounded-lg border border-gray-200 items-end">
        <div>
          <label className="block text-xs text-gray-500 mb-1">District 1</label>
          <select className="border border-gray-300 rounded px-3 py-1.5 text-sm" value={d1} onChange={e => setD1(e.target.value)}>
            <option value="">Selectionner...</option>
            {filters?.districts.filter(d => d !== d2).map(d => <option key={d} value={d}>{d}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-1">District 2</label>
          <select className="border border-gray-300 rounded px-3 py-1.5 text-sm" value={d2} onChange={e => setD2(e.target.value)}>
            <option value="">Selectionner...</option>
            {filters?.districts.filter(d => d !== d1).map(d => <option key={d} value={d}>{d}</option>)}
          </select>
        </div>
        <button
          onClick={compare}
          disabled={!d1 || !d2 || loading}
          className="px-4 py-1.5 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-40"
        >
          {loading ? 'Chargement...' : 'Comparer'}
        </button>
      </div>

      {result && (
        <>
          {/* KPI comparison */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            {[
              {
                label: 'Score qualite',
                v1: result.district1.avg_score.toFixed(1),
                v2: result.district2.avg_score.toFixed(1),
                nat: result.national.avg_score.toFixed(1),
                c1: colorScore(result.district1.avg_score),
                c2: colorScore(result.district2.avg_score),
              },
              {
                label: 'Taux rapportage',
                v1: `${result.district1.reporting_pct.toFixed(1)}%`,
                v2: `${result.district2.reporting_pct.toFixed(1)}%`,
                nat: `${result.national.reporting_pct.toFixed(1)}%`,
                c1: colorPct(result.district1.reporting_pct),
                c2: colorPct(result.district2.reporting_pct),
              },
              {
                label: 'Structures',
                v1: String(result.district1.n_structures),
                v2: String(result.district2.n_structures),
                nat: String(result.national.n_structures),
                c1: 'text-gray-900', c2: 'text-gray-900',
              },
              {
                label: 'Med./structure',
                v1: result.district1.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-',
                v2: result.district2.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-',
                nat: result.national.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-',
                c1: 'text-gray-900', c2: 'text-gray-900',
              },
            ].map((kpi, i) => (
              <div key={i} className="bg-white rounded-lg border border-gray-200 p-4">
                <p className="text-xs text-gray-500 mb-3 font-medium">{kpi.label}</p>
                <div className="flex justify-between items-end">
                  <div className="text-center">
                    <p className={`text-lg font-bold ${kpi.c1}`}>{kpi.v1}</p>
                    <p className="text-[10px] text-gray-400 mt-1">{result.district1.name}</p>
                  </div>
                  <div className="text-center">
                    <p className="text-sm text-gray-400">{kpi.nat}</p>
                    <p className="text-[10px] text-gray-300">National</p>
                  </div>
                  <div className="text-center">
                    <p className={`text-lg font-bold ${kpi.c2}`}>{kpi.v2}</p>
                    <p className="text-[10px] text-gray-400 mt-1">{result.district2.name}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Services table */}
          {servicesChartData.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Services — nombre de structures avec le service</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-gray-50">
                      <th className="text-left px-3 py-2 font-medium text-gray-500">Service</th>
                      <th className="text-right px-3 py-2 font-medium text-blue-600">{result.district1.name}</th>
                      <th className="text-right px-3 py-2 font-medium text-orange-600">{result.district2.name}</th>
                      <th className="px-3 py-2 font-medium text-gray-500 w-48">Comparaison</th>
                    </tr>
                  </thead>
                  <tbody>
                    {servicesChartData.map((row) => {
                      const v1 = (row as Record<string, unknown>)[result.district1.name] as number;
                      const v2 = (row as Record<string, unknown>)[result.district2.name] as number;
                      const max = Math.max(v1, v2, 1);
                      return (
                        <tr key={row.name} className="border-b border-gray-100">
                          <td className="px-3 py-1.5 text-gray-700">{row.name}</td>
                          <td className="px-3 py-1.5 text-right font-medium">{v1}</td>
                          <td className="px-3 py-1.5 text-right font-medium">{v2}</td>
                          <td className="px-3 py-1.5">
                            <div className="flex gap-0.5 items-center h-4">
                              <div className="h-3 rounded-sm bg-blue-500" style={{ width: `${(v1 / max) * 45}%` }} />
                              <div className="h-3 rounded-sm bg-orange-500" style={{ width: `${(v2 / max) * 45}%` }} />
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Commodites chart */}
          {commoditesData.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Commodites — % de structures</h3>
              <ResponsiveContainer width="100%" height={Math.max(200, commoditesData.length * 35)}>
                <BarChart data={commoditesData} layout="vertical" margin={{ left: 120 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis type="number" domain={[0, 100]} unit="%" />
                  <YAxis type="category" dataKey="name" tick={{ fontSize: 10 }} width={110} />
                  <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
                  <Legend />
                  <ReferenceLine x={0} stroke="transparent" />
                  <Bar dataKey={result.district1.name} fill="#3b82f6" />
                  <Bar dataKey={result.district2.name} fill="#f97316" />
                  <Bar dataKey="national" fill="#d1d5db" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* RH comparison table */}
          {result.district1.rh.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">Ressources humaines — effectifs par profil</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-gray-50">
                      <th className="text-left px-3 py-2 font-medium text-gray-500">Profil</th>
                      <th className="text-right px-3 py-2 font-medium text-blue-600">{result.district1.name}</th>
                      <th className="text-right px-3 py-2 font-medium text-orange-600">{result.district2.name}</th>
                      <th className="text-right px-3 py-2 font-medium text-gray-400">National</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(() => {
                      const allProfiles = new Set<string>();
                      result.district1.rh.forEach(r => allProfiles.add(r.profil_code));
                      result.district2.rh.forEach(r => allProfiles.add(r.profil_code));
                      const d1Map = new Map(result.district1.rh.map(r => [r.profil_code, r]));
                      const d2Map = new Map(result.district2.rh.map(r => [r.profil_code, r]));
                      const natMap = new Map(result.national.rh.map(r => [r.profil_code, r]));
                      return Array.from(allProfiles).map(code => {
                        const r1 = d1Map.get(code);
                        const r2 = d2Map.get(code);
                        const rn = natMap.get(code);
                        return (
                          <tr key={code} className="border-b border-gray-100">
                            <td className="px-3 py-2 text-gray-700">{r1?.label || r2?.label || code}</td>
                            <td className="px-3 py-2 text-right font-medium">{r1?.effectif_total ?? 0}</td>
                            <td className="px-3 py-2 text-right font-medium">{r2?.effectif_total ?? 0}</td>
                            <td className="px-3 py-2 text-right text-gray-400">{rn?.effectif_total ?? 0}</td>
                          </tr>
                        );
                      });
                    })()}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}

      <MethodNote title="Methodologie - Comparaison de districts">
        <p>Cet outil permet de comparer deux districts cote a cote sur l'ensemble des indicateurs ISS.</p>
        <p><strong>Score qualite</strong> : moyenne des scores de toutes les structures du district.</p>
        <p><strong>Taux de rapportage</strong> : structures ayant soumis / structures attendues.</p>
        <p><strong>Services</strong> : nombre de structures disposant de chaque service (fonctionnel).</p>
        <p><strong>Commodites</strong> : pourcentage de structures disposant de chaque commodite (energie, eau).</p>
        <p>La <strong>moyenne nationale</strong> est affichee comme reference (barres grises ou valeur centrale).</p>
      </MethodNote>
    </div>
  );
}
