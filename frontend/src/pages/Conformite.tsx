import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import { AlertTriangle, ClipboardCheck } from 'lucide-react';
import { api } from '../api/client';
import { useUrlState, useUrlStateInt } from '../hooks/useUrlState';
import type { ConformiteGap, ConformiteStructureResult, ConformiteSummaryRow, Filters, NormesMeta } from '../types';
import { typologieLabel } from '../utils/typologie';
import { typeColor } from '../api/public';
import KpiCard from '../components/KpiCard';
import DataTable from '../components/DataTable';
import ExportCSV from '../components/ExportCSV';
import MethodNote from '../components/MethodNote';

const KIND_LABELS: Record<string, string> = { service: 'Service', rh: 'RH', equipement: 'Équipement', infra: 'Infrastructure' };
const scoreColor = (v: number | null) => (v === null ? '#d1d5db' : v < 50 ? '#ef4444' : v < 70 ? '#f97316' : v < 85 ? '#eab308' : '#22c55e');
const fmt = (v: number | null | undefined, d = 0) => (v === null || v === undefined ? '—' : v.toFixed(d));

export default function Conformite() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<Filters | null>(null);
  const [meta, setMeta] = useState<NormesMeta | null>(null);
  const [by, setBy] = useUrlState('by', 'district');
  const [type, setType] = useUrlState('type');
  const [region, setRegion] = useUrlState('region');
  const [district, setDistrict] = useUrlState('district');
  const [kind, setKind] = useUrlState('kind');
  const [level, setLevel] = useUrlState('level', 'essentiel');
  const [status, setStatus] = useUrlState('status');
  const [page, setPage] = useUrlStateInt('page', 1);

  const [byType, setByType] = useState<ConformiteSummaryRow[]>([]);
  const [rows, setRows] = useState<ConformiteSummaryRow[]>([]);
  const [gaps, setGaps] = useState<ConformiteGap[]>([]);
  const [structures, setStructures] = useState<ConformiteStructureResult | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
    api.getNormesMeta().then(setMeta).catch((e) => setError(e.message));
    api.getConformiteSummary('type').then(setByType).catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    api.getConformiteSummary(by, type).then(setRows).catch((e) => setError(e.message));
  }, [by, type]);

  // Écarts : au district si un district est choisi, sinon à la région, sinon national.
  const gapScope = district ? { by: 'district', key: district } : region ? { by: 'region', key: region } : { by: 'global' };
  useEffect(() => {
    api.getConformiteGaps({ ...gapScope, type, kind, level, limit: 200 }).then(setGaps).catch((e) => setError(e.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [district, region, type, kind, level]);

  useEffect(() => {
    api.getConformiteStructures({ region, district, type, status, page, pageSize: 25 }).then(setStructures).catch((e) => setError(e.message));
  }, [region, district, type, status, page]);

  const filteredRows = useMemo(
    () => (by === 'district' && region && filters ? rows.filter((r) => filters.district_regions[r.key] === region) : rows),
    [rows, by, region, filters],
  );
  const chartData = useMemo(
    () => filteredRows.filter((r) => r.avg_score !== null).map((r) => ({ name: r.label, score: r.avg_score as number, pct: r.pct_conformes as number })).sort((a, b) => a.score - b.score),
    [filteredRows],
  );

  const run = meta?.last_run;
  const pctConf = run && run.n_evaluees > 0 ? (100 * run.n_conformes) / run.n_evaluees : null;
  const national = byType.reduce((acc, r) => acc + (r.avg_score ?? 0) * r.n_evaluees, 0) / Math.max(1, byType.reduce((a, r) => a + r.n_evaluees, 0));
  const essentialGapsNational = gaps.filter((g) => g.level === 'essentiel').reduce((a, g) => a + g.n_manque, 0);

  if (meta && !meta.active) {
    return (
      <div className="space-y-4">
        <h2 className="text-xl font-bold text-gray-900">Conformité aux normes</h2>
        <div className="bg-white rounded-lg border border-gray-200 p-8 text-center text-gray-500">
          <ClipboardCheck size={32} className="mx-auto mb-2 text-gray-300" />
          Aucun référentiel de normes n'est actif. Un administrateur peut en créer un dans{' '}
          <Link to="/admin?tab=normes" className="text-blue-600 hover:underline">Admin → Normes</Link> (un CSV d'exemple est fourni dans le dépôt).
        </div>
      </div>
    );
  }

  const summaryColumns = [
    { key: 'label', header: by === 'type' ? 'Type' : by === 'sous_prefecture' ? 'Sous-préfecture' : by.charAt(0).toUpperCase() + by.slice(1) },
    { key: 'n_structures', header: 'Structures' },
    { key: 'n_evaluees', header: 'Évaluées' },
    { key: 'avg_score', header: 'Score moyen', render: (r: Record<string, unknown>) => (
      <span className="inline-flex items-center gap-2"><span className="w-2.5 h-2.5 rounded-full" style={{ background: scoreColor(r.avg_score as number | null) }} />{fmt(r.avg_score as number | null)}</span>
    ) },
    { key: 'n_conformes', header: 'Conformes' },
    { key: 'pct_conformes', header: '% conformes', render: (r: Record<string, unknown>) => (r.pct_conformes === null ? '—' : `${fmt(r.pct_conformes as number)}%`) },
  ];
  const gapColumns = [
    { key: 'type_code', header: 'Type', render: (g: Record<string, unknown>) => typologieLabel(String(g.type_code)) },
    { key: 'kind', header: 'Famille', render: (g: Record<string, unknown>) => KIND_LABELS[String(g.kind)] ?? String(g.kind) },
    { key: 'label', header: 'Exigence' },
    { key: 'level', header: 'Niveau', render: (g: Record<string, unknown>) => (g.level === 'essentiel' ? <span className="text-red-700 font-medium">essentiel</span> : 'recommandé') },
    { key: 'n_manque', header: 'En manque', render: (g: Record<string, unknown>) => `${g.n_manque} / ${g.n_concernees}` },
    { key: 'pct', header: '%', render: (g: Record<string, unknown>) => `${((100 * (g.n_manque as number)) / Math.max(1, g.n_concernees as number)).toFixed(0)}%` },
    { key: 'deficit', header: 'Déficit', render: (g: Record<string, unknown>) => String(Math.round(g.deficit as number)) },
    { key: 'n_inconnu', header: 'Non renseigné' },
  ];
  const districtsOfRegion = filters?.districts.filter((d) => !region || filters.district_regions[d] === region) ?? [];
  const totalPages = structures ? Math.ceil(structures.total / structures.page_size) : 0;
  const scopeLabel = district || region || 'National';

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <h2 className="text-xl font-bold text-gray-900">Conformité aux normes</h2>
        {meta?.active && (
          <span className="text-xs text-gray-500">
            Référentiel <strong>{meta.active.name}</strong> v{meta.active.version} · {meta.active.n_rules} règles
            {run && ` · calculé le ${new Date(run.computed_at).toLocaleString('fr-FR')}`}
          </span>
        )}
      </div>
      {error && <div className="text-sm text-red-600">{error}</div>}

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <KpiCard title="Score de conformité moyen" value={fmt(national, 1)} subtitle="sur 100, pondéré essentiel ×2" color={national >= 85 ? 'text-green-600' : national >= 70 ? 'text-yellow-600' : 'text-red-600'} />
        <KpiCard title="Structures conformes" value={pctConf === null ? '—' : `${pctConf.toFixed(1)}%`} subtitle={run ? `${run.n_conformes} / ${run.n_evaluees} évaluées — aucun manque essentiel` : ''} />
        <KpiCard title={`Manques essentiels — ${scopeLabel}`} value={essentialGapsNational.toLocaleString('fr-FR')} subtitle="structure × exigence essentielle" color="text-red-600" icon={<AlertTriangle size={18} />} />
        <KpiCard title="Structures évaluées" value={run?.n_evaluees ?? '—'} subtitle={run ? `sur ${run.n_structures}` : ''} />
      </div>

      {/* Filtres globaux */}
      <div className="flex flex-wrap gap-2 bg-white p-3 rounded-lg border border-gray-200 items-center">
        <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={type} onChange={(e) => { setType(e.target.value); setPage(1); }}>
          <option value="">Tous les types</option>
          {filters?.types?.map((t) => <option key={t.code} value={t.code}>{t.label} ({t.n})</option>)}
        </select>
        <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={region} onChange={(e) => { setRegion(e.target.value); setDistrict(''); setPage(1); }}>
          <option value="">Toutes régions</option>
          {filters?.regions.map((r) => <option key={r} value={r}>{r}</option>)}
        </select>
        <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={district} onChange={(e) => { setDistrict(e.target.value); setPage(1); }}>
          <option value="">Tous districts</option>
          {districtsOfRegion.map((d) => <option key={d} value={d}>{d}</option>)}
        </select>
      </div>

      {/* Par type */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <h3 className="font-semibold text-gray-800">Par type de structure</h3>
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-2">
          {byType.filter((r) => r.n_evaluees > 0).map((r) => (
            <button key={r.key} onClick={() => setType(type === r.key ? '' : r.key)} className={`text-left rounded-lg border p-2 hover:bg-gray-50 ${type === r.key ? 'border-blue-500 bg-blue-50' : 'border-gray-200'}`}>
              <div className="flex items-center gap-1.5 text-xs text-gray-600"><span className="w-2 h-2 rounded-full" style={{ background: typeColor(r.key) }} />{r.label}</div>
              <div className="text-lg font-semibold" style={{ color: scoreColor(r.avg_score) }}>{fmt(r.avg_score)}</div>
              <div className="text-[11px] text-gray-500">{fmt(r.pct_conformes)}% conformes · {r.n_structures}</div>
            </button>
          ))}
        </div>
      </div>

      {/* Par unité géographique */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-800 mr-2">Par zone</h3>
          {['region', 'district', 'sous_prefecture'].map((v) => (
            <button key={v} onClick={() => setBy(v)} className={`px-2 py-1 text-xs rounded ${by === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}>
              {v === 'sous_prefecture' ? 'Sous-préfecture' : v.charAt(0).toUpperCase() + v.slice(1)}
            </button>
          ))}
          <span className="text-xs text-gray-400">{type ? `type ${typologieLabel(type)}` : 'tous types'}</span>
          <div className="ml-auto"><ExportCSV data={filteredRows as unknown as Record<string, unknown>[]} columns={summaryColumns} filename={`conformite_${by}`} /></div>
        </div>
        {chartData.length > 0 && chartData.length <= 60 && (
          <ResponsiveContainer width="100%" height={Math.max(200, chartData.length * 18)}>
            <BarChart data={chartData} layout="vertical" margin={{ left: 20, right: 30 }}>
              <CartesianGrid strokeDasharray="3 3" horizontal={false} />
              <XAxis type="number" domain={[0, 100]} fontSize={11} />
              <YAxis type="category" dataKey="name" width={140} fontSize={11} interval={0} />
              <Tooltip formatter={(v: number, _n, p) => [`${v.toFixed(1)} (${p.payload.pct?.toFixed(0)}% conformes)`, 'Score']} />
              <Bar dataKey="score" radius={[0, 3, 3, 0]}>{chartData.map((d) => <Cell key={d.name} fill={scoreColor(d.score)} />)}</Bar>
            </BarChart>
          </ResponsiveContainer>
        )}
        <DataTable columns={summaryColumns} data={filteredRows as unknown as Record<string, unknown>[]} />
      </div>

      {/* Écarts */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-800 mr-2">Écarts aux normes — {scopeLabel}</h3>
          <select className="border border-gray-300 rounded px-2 py-1 text-xs" value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="">Toutes familles</option>
            {Object.entries(KIND_LABELS).map(([k, l]) => <option key={k} value={k}>{l}</option>)}
          </select>
          <select className="border border-gray-300 rounded px-2 py-1 text-xs" value={level} onChange={(e) => setLevel(e.target.value)}>
            <option value="">Tous niveaux</option>
            <option value="essentiel">Essentiel</option>
            <option value="recommande">Recommandé</option>
          </select>
          <div className="ml-auto"><ExportCSV data={gaps as unknown as Record<string, unknown>[]} columns={gapColumns.filter((c) => c.key !== 'pct')} filename={`ecarts_${scopeLabel}`} /></div>
        </div>
        <p className="text-xs text-gray-500">« En manque » = structures du type concernées qui n'atteignent pas le minimum ; « déficit » = total à combler (effectifs, unités) pour que toutes l'atteignent. Triés par déficit.</p>
        <DataTable columns={gapColumns} data={gaps as unknown as Record<string, unknown>[]} />
      </div>

      {/* Structures */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-800 mr-2">Structures {structures ? `(${structures.total})` : ''}</h3>
          <select className="border border-gray-300 rounded px-2 py-1 text-xs" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }}>
            <option value="">Toutes</option>
            <option value="non_conforme">Non conformes</option>
            <option value="conforme">Conformes</option>
            <option value="non_evalue">Non évaluées</option>
          </select>
        </div>
        <DataTable
          columns={[
            { key: 'name', header: 'Structure' },
            { key: 'type_code', header: 'Type', render: (r: Record<string, unknown>) => typologieLabel(String(r.type_code)) },
            { key: 'district', header: 'District' },
            { key: 'score', header: 'Score', render: (r: Record<string, unknown>) => (
              <span className="inline-flex items-center gap-2"><span className="w-2.5 h-2.5 rounded-full" style={{ background: scoreColor(r.score as number | null) }} />{fmt(r.score as number | null)}</span>
            ) },
            { key: 'conforme', header: 'Conforme', render: (r: Record<string, unknown>) => (r.score === null ? '—' : r.conforme ? <span className="text-green-700">oui</span> : <span className="text-red-700">non</span>) },
            { key: 'n_manque_essentiel', header: 'Manques essentiels' },
            { key: 'n_manque', header: 'Manques' },
            { key: 'n_inconnu', header: 'Non renseignés' },
          ]}
          data={(structures?.data ?? []) as unknown as Record<string, unknown>[]}
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

      <MethodNote title="Méthodologie - Conformité aux normes">
        <p>Chaque structure est comparée aux exigences du référentiel actif pour son type (services attendus, effectifs minimaux, équipements fonctionnels et infrastructures minimales). Les règles « * » s'appliquent à tous les types ; une règle spécifique au type prime.</p>
        <p><strong>Score</strong> = 100 × Σ poids(exigences satisfaites) / Σ poids(satisfaites + manquantes), poids 2 pour une exigence essentielle, 1 pour une recommandée. Une exigence dont la donnée n'est pas renseignée dans ISS est « non renseignée » et n'entre pas dans le score (la qualité des données est traitée à part).</p>
        <p><strong>Conforme</strong> = aucune exigence essentielle manquante. Les effectifs RH additionnent tous les statuts d'emploi ; les équipements sont comptés en unités fonctionnelles.</p>
        <p>Le référentiel est une donnée éditée par l'administration (Admin → Normes), versionnée ; le calcul est refait à chaque synchronisation et à chaque changement du référentiel actif.</p>
      </MethodNote>
    </div>
  );
}
