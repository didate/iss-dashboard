import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import { FileDown, MapPinOff } from 'lucide-react';
import { api } from '../api/client';
import { useUrlState, useUrlStateInt } from '../hooks/useUrlState';
import type { Filters, MissingGPSResult, UsageGeo } from '../types';
import KpiCard from '../components/KpiCard';
import DataTable from '../components/DataTable';
import ExportCSV from '../components/ExportCSV';
import MethodNote from '../components/MethodNote';

const pctColor = (v: number | null) => (v === null ? '#d1d5db' : v < 50 ? '#ef4444' : v < 80 ? '#eab308' : '#22c55e');

export default function Geolocalisation() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<Filters | null>(null);
  const [level, setLevel] = useUrlState('level', '3');
  const [region, setRegion] = useUrlState('region');
  const [district, setDistrict] = useUrlState('district');
  const [type, setType] = useUrlState('type');
  const [page, setPage] = useUrlStateInt('page', 1);

  const [national, setNational] = useState<UsageGeo[]>([]);
  const [coverage, setCoverage] = useState<UsageGeo[]>([]);
  const [missing, setMissing] = useState<MissingGPSResult | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
    api.getGeoCoverage(3).then(setNational).catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    api.getGeoCoverage(Number(level), region).then(setCoverage).catch((e) => setError(e.message));
  }, [level, region]);

  useEffect(() => {
    api.getMissingGPS({ district, region, type, page, pageSize: 25 }).then(setMissing).catch((e) => setError(e.message));
  }, [district, region, type, page]);

  // KPI nationaux : somme des districts (usage_geo niveau 3 couvre tout le pays).
  const kpi = useMemo(() => {
    const n = national.reduce((s, r) => s + r.n_structures, 0);
    const g = national.reduce((s, r) => s + r.n_gps, 0);
    const low = national.filter((r) => r.n_structures > 0 && (r.pct_gps ?? 0) < 50).length;
    return { n, g, pct: n ? (100 * g) / n : 0, missing: n - g, low };
  }, [national]);

  const chartData = useMemo(
    () =>
      coverage
        .filter((r) => r.n_structures > 0)
        .map((r) => ({ name: r.name, pct: r.pct_gps ?? 0, n: r.n_structures }))
        .sort((a, b) => a.pct - b.pct),
    [coverage],
  );

  const columns = [
    { key: 'name', header: level === '4' ? 'Sous-préfecture' : 'District' },
    { key: 'parent_name', header: level === '4' ? 'District' : 'Région' },
    { key: 'n_structures', header: 'Structures' },
    { key: 'n_gps', header: 'Avec GPS' },
    {
      key: 'missing',
      header: 'Sans GPS',
      render: (r: Record<string, unknown>) => String((r.n_structures as number) - (r.n_gps as number)),
    },
    {
      key: 'pct_gps',
      header: '% GPS',
      render: (r: Record<string, unknown>) => {
        const v = r.pct_gps as number | null;
        return v === null ? '—' : (
          <span className="inline-flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded-full" style={{ background: pctColor(v) }} />
            {v.toFixed(0)}%
          </span>
        );
      },
    },
  ];

  const districtsOfRegion = filters?.districts.filter((d) => !region || filters.district_regions[d] === region) ?? [];
  const totalPages = missing ? Math.ceil(missing.total / missing.page_size) : 0;

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Géolocalisation des structures</h2>
      {error && <div className="text-sm text-red-600">{error}</div>}

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <KpiCard title="Structures géolocalisées" value={`${kpi.pct.toFixed(1)}%`} subtitle={`${kpi.g.toLocaleString('fr-FR')} / ${kpi.n.toLocaleString('fr-FR')}`} color={kpi.pct >= 80 ? 'text-green-600' : 'text-yellow-600'} />
        <KpiCard title="Sans coordonnées" value={kpi.missing} subtitle="à positionner sur le terrain" color="text-red-600" icon={<MapPinOff size={18} />} />
        <KpiCard title="Districts < 50 % GPS" value={kpi.low} subtitle={`sur ${national.length}`} />
        <KpiCard title="Source" value="DHIS2" subtitle="géométrie des unités d'organisation" />
      </div>

      {/* Couverture par unité administrative */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex gap-1 bg-gray-100 rounded p-0.5">
            {[['3', 'Districts'], ['4', 'Sous-préfectures']].map(([v, l]) => (
              <button key={v} onClick={() => setLevel(v)} className={`px-3 py-1 text-xs rounded ${level === v ? 'bg-gray-800 text-white' : 'text-gray-600'}`}>
                {l}
              </button>
            ))}
          </div>
          <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={region} onChange={(e) => { setRegion(e.target.value); setDistrict(''); setPage(1); }}>
            <option value="">Toutes régions</option>
            {filters?.regions.map((r) => <option key={r} value={r}>{r}</option>)}
          </select>
          <div className="ml-auto">
            <ExportCSV data={coverage as unknown as Record<string, unknown>[]} columns={columns.filter((c) => c.key !== 'missing')} filename={`couverture_gps_niveau${level}`} />
          </div>
        </div>

        {chartData.length > 0 && chartData.length <= 60 && (
          <ResponsiveContainer width="100%" height={Math.max(220, chartData.length * 18)}>
            <BarChart data={chartData} layout="vertical" margin={{ left: 20, right: 30 }}>
              <CartesianGrid strokeDasharray="3 3" horizontal={false} />
              <XAxis type="number" domain={[0, 100]} tickFormatter={(v) => `${v}%`} fontSize={11} />
              <YAxis type="category" dataKey="name" width={140} fontSize={11} interval={0} />
              <Tooltip formatter={(v: number, _n, p) => [`${v.toFixed(1)}% (${p.payload.n} structures)`, '% GPS']} />
              <Bar dataKey="pct" radius={[0, 3, 3, 0]}>
                {chartData.map((d) => <Cell key={d.name} fill={pctColor(d.pct)} />)}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        )}

        <DataTable columns={columns} data={coverage as unknown as Record<string, unknown>[]} />
      </div>

      {/* Liste des structures sans GPS */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <h3 className="font-semibold text-gray-800">Structures sans coordonnées {missing ? `(${missing.total})` : ''}</h3>
          <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={district} onChange={(e) => { setDistrict(e.target.value); setPage(1); }}>
            <option value="">Tous districts</option>
            {districtsOfRegion.map((d) => <option key={d} value={d}>{d}</option>)}
          </select>
          <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={type} onChange={(e) => { setType(e.target.value); setPage(1); }}>
            <option value="">Tous types</option>
            {filters?.types?.map((t) => <option key={t.code} value={t.code}>{t.label}</option>)}
          </select>
          <button
            onClick={() => api.exportMissingGPSCSV({ district, region, type }).catch((e) => setError(e.message))}
            className="ml-auto flex items-center gap-1 px-3 py-1.5 text-xs border border-gray-300 rounded hover:bg-gray-50 text-gray-600"
          >
            <FileDown size={14} /> Liste complète (CSV)
          </button>
        </div>

        <DataTable
          columns={[
            { key: 'name', header: 'Structure' },
            { key: 'type_label', header: 'Type' },
            { key: 'sous_prefecture', header: 'Sous-préfecture' },
            { key: 'district', header: 'District' },
            { key: 'region', header: 'Région' },
            { key: 'event_date', header: 'Recensée le', render: (r: Record<string, unknown>) => String(r.event_date ?? '').slice(0, 10) },
          ]}
          data={(missing?.data ?? []) as unknown as Record<string, unknown>[]}
          onRowClick={(r) => navigate(`/structure/${r.event_uid}`)}
        />
        {totalPages > 1 && (
          <div className="flex items-center justify-between text-xs text-gray-500">
            <span>page {page}/{totalPages}</span>
            <div className="flex gap-1">
              <button className="px-2 py-1 border rounded disabled:opacity-40" disabled={page <= 1} onClick={() => setPage(page - 1)}>Préc.</button>
              <button className="px-2 py-1 border rounded disabled:opacity-40" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>Suiv.</button>
            </div>
          </div>
        )}
      </div>

      <MethodNote title="Méthodologie - Géolocalisation">
        <p>Une structure est <strong>géolocalisée</strong> quand son unité d'organisation DHIS2 porte une géométrie de type <em>Point</em>. Les coordonnées sont copiées à chaque synchronisation ; l'app ne les modifie jamais.</p>
        <p>Chaque structure sans point génère l'alerte qualité <strong>R14</strong> (avertissement). La liste ci-dessus et son export CSV servent de feuille de route aux équipes terrain ; la saisie se fait dans DHIS2 (Maintenance → Unités d'organisation).</p>
        <p>Le pourcentage par district/sous-préfecture compte une structure une seule fois (dernier recensement), rattachée par la hiérarchie DHIS2.</p>
      </MethodNote>
    </div>
  );
}
