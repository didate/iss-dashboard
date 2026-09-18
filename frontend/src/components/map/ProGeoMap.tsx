import { useCallback, useEffect, useMemo, useState } from 'react';
import { MapContainer, GeoJSON, TileLayer } from 'react-leaflet';
import type { Layer, PathOptions } from 'leaflet';
import type { Feature, Geometry } from 'geojson';
import { api } from '../../api/client';
import { useUrlState } from '../../hooks/useUrlState';
import type { MapGeoCollection, MapGeoFeature, ProPointCollection } from '../../types';
import { typologieLabel } from '../../utils/typologie';
import ClusterLayer, { escapeHtml, type MarkerSpec } from './ClusterLayer';
import InvalidateOnResize from './InvalidateOnResize';

type Props = { mode: 'gps' | 'points' };

const METRICS: { key: string; label: string; unit: string }[] = [
  { key: 'pct_gps', label: 'Couverture GPS', unit: '%' },
  { key: 'avg_score', label: 'Score qualité moyen', unit: '' },
  { key: 'n_structures', label: 'Nombre de structures', unit: '' },
  { key: 'ratio_structures_10k', label: 'Structures pour 10 000 hab.', unit: '' },
  // ratios de couverture (usage_couverture) : clé = ratio:<indicateur>
  { key: 'ratio:personnel_soignant', label: 'Personnel soignant pour 10 000 hab.', unit: '' },
  { key: 'ratio:medecins', label: 'Médecins pour 10 000 hab.', unit: '' },
  { key: 'ratio:sages_femmes', label: 'Sages-femmes pour 10 000 hab.', unit: '' },
  { key: 'ratio:infirmiers', label: 'Infirmiers pour 10 000 hab.', unit: '' },
  { key: 'ratio:ats', label: 'ATS pour 10 000 hab.', unit: '' },
  { key: 'ratio:lits', label: "Lits d'hospitalisation pour 10 000 hab.", unit: '' },
];

const GREY = '#d1d5db';

const RATIO_LABELS: Record<string, string> = {
  personnel_soignant: 'Personnel soignant',
  medecins: 'Médecins',
  sages_femmes: 'Sages-femmes',
  infirmiers: 'Infirmiers',
  ats: 'ATS',
  lits: 'Lits',
};

function scoreColor(score: number): string {
  if (score < 50) return '#ef4444';
  if (score < 65) return '#f97316';
  if (score < 80) return '#eab308';
  return '#22c55e';
}

function quantileBreaks(values: number[], n: number): number[] {
  const sorted = values.filter((v) => v > 0).sort((a, b) => a - b);
  if (sorted.length === 0) return [];
  const out: number[] = [];
  for (let i = 1; i < n; i++) out.push(sorted[Math.min(Math.floor((i / n) * sorted.length), sorted.length - 1)]);
  return out;
}

const RAMP = ['#ef4444', '#f97316', '#eab308', '#84cc16', '#22c55e'];

function rampColor(v: number | null, breaks: number[]): string {
  if (v === null || v === undefined) return GREY;
  for (let i = 0; i < breaks.length; i++) if (v <= breaks[i]) return RAMP[i];
  return RAMP[RAMP.length - 1];
}

function metricValue(p: MapGeoFeature['properties'], key: string): number | null {
  if (key.startsWith('ratio:')) {
    const v = p.ratios?.[key.slice(6)];
    return typeof v === 'number' ? v : null;
  }
  const v = p[key as keyof typeof p];
  return typeof v === 'number' ? v : null;
}

const STRUCT_BASE = `${import.meta.env.BASE_URL.replace(/\/$/, '')}/structure/`;

// Cartes « carte sanitaire » de l'espace pro : choroplèthe par district ou
// sous-préfecture (usage_geo) et points des structures colorés par score qualité.
export default function ProGeoMap({ mode }: Props) {
  const [level, setLevel] = useUrlState('level', '3');
  const [metric, setMetric] = useUrlState('metric', 'pct_gps');
  const [clusterParam, setClusterParam] = useUrlState('cluster');
  const cluster = clusterParam !== 'off';

  const [geo, setGeo] = useState<MapGeoCollection | null>(null);
  const [points, setPoints] = useState<ProPointCollection | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    if (mode !== 'gps') return;
    setGeo(null);
    api.getMapGeo(Number(level)).then(setGeo).catch((e) => setError(e.message));
  }, [mode, level]);

  useEffect(() => {
    if (mode !== 'points' || points) return;
    api.getMapPoints().then(setPoints).catch((e) => setError(e.message));
  }, [mode, points]);

  const isPct = metric === 'pct_gps' || metric === 'avg_score';
  const breaks = useMemo(() => {
    if (!geo || isPct) return [];
    return quantileBreaks(geo.features.map((f) => metricValue(f.properties, metric) ?? 0), 5);
  }, [geo, metric, isPct]);

  const colorOf = useCallback(
    (v: number | null): string => {
      if (v === null) return GREY;
      if (metric === 'pct_gps') return v < 50 ? '#ef4444' : v < 80 ? '#eab308' : '#22c55e';
      if (metric === 'avg_score') return scoreColor(v);
      return rampColor(v, breaks);
    },
    [metric, breaks],
  );

  const style = useCallback(
    (f?: Feature<Geometry, MapGeoFeature['properties']>): PathOptions => ({
      fillColor: f ? colorOf(metricValue(f.properties, metric)) : GREY,
      weight: level === '4' ? 0.8 : 1.5,
      color: '#374151',
      fillOpacity: 0.7,
    }),
    [colorOf, metric, level],
  );

  const onEachFeature = useCallback((f: Feature<Geometry, MapGeoFeature['properties']>, layer: Layer) => {
    const p = f.properties;
    const types = Object.entries(p.n_par_type || {})
      .sort((a, b) => b[1] - a[1])
      .map(([k, n]) => `${escapeHtml(typologieLabel(k))} : ${n}`)
      .join('<br/>');
    layer.bindPopup(`
      <div style="min-width:200px;font-size:12px">
        <div style="font-weight:700;font-size:13px">${escapeHtml(p.name)}</div>
        <div style="color:#6b7280;margin-bottom:6px">${escapeHtml(p.parent_name)}</div>
        <div><b>${p.n_structures}</b> structures, <b>${p.n_gps}</b> géolocalisées (${p.pct_gps === null ? '—' : p.pct_gps.toFixed(0) + '%'})</div>
        <div>Score qualité moyen : <b>${p.avg_score === null ? '—' : p.avg_score.toFixed(1)}</b></div>
        <div>Population : <b>${p.population === null ? '—' : Math.round(p.population).toLocaleString('fr-FR')}</b></div>
        <div>Structures /10 000 hab. : <b>${p.ratio_structures_10k === null ? '—' : p.ratio_structures_10k.toFixed(2)}</b></div>
        ${['personnel_soignant', 'medecins', 'sages_femmes', 'infirmiers', 'lits']
          .filter((k) => p.numerators?.[k] !== undefined)
          .map((k) => `<div>${escapeHtml(RATIO_LABELS[k] ?? k)} : <b>${p.numerators[k]}</b>${p.ratios?.[k] != null ? ` (${p.ratios[k]!.toFixed(2)} /10 000)` : ''}</div>`)
          .join('')}
        ${types ? `<div style="margin-top:6px;color:#374151">${types}</div>` : ''}
      </div>`);
  }, []);

  const markers = useMemo<MarkerSpec[]>(
    () =>
      (points?.features ?? []).map((f) => {
        const p = f.properties;
        return {
          uid: p.event_uid,
          lat: f.geometry.coordinates[1],
          lng: f.geometry.coordinates[0],
          color: scoreColor(p.score),
          popup: `
            <div style="min-width:180px;font-size:12px">
              <div style="font-weight:600;font-size:13px">${escapeHtml(p.name)}</div>
              <div style="color:#374151">${escapeHtml(p.type_label)} · ${escapeHtml(p.district)}</div>
              <div style="margin:4px 0">Score qualité : <b>${p.score}</b> · ${p.n_issues} problème${p.n_issues > 1 ? 's' : ''}</div>
              <a href="${STRUCT_BASE}${encodeURIComponent(p.event_uid)}" style="color:#1d4ed8;font-weight:500">Détail de la structure →</a>
            </div>`,
        };
      }),
    [points],
  );

  const legend =
    mode === 'points'
      ? [['#22c55e', '≥ 80'], ['#eab308', '65 – 80'], ['#f97316', '50 – 65'], ['#ef4444', '< 50']]
      : metric === 'pct_gps'
        ? [['#22c55e', '≥ 80 %'], ['#eab308', '50 – 80 %'], ['#ef4444', '< 50 %'], [GREY, 'Aucune structure']]
        : metric === 'avg_score'
          ? [['#22c55e', '≥ 80'], ['#eab308', '65 – 80'], ['#f97316', '50 – 65'], ['#ef4444', '< 50'], [GREY, 'Pas de données']]
          : [
              ...breaks.map((b, i) => [RAMP[i], `≤ ${b.toFixed(metric === 'n_structures' ? 0 : 2)}`]),
              [RAMP[RAMP.length - 1], `> ${(breaks[breaks.length - 1] ?? 0).toFixed(metric === 'n_structures' ? 0 : 2)}`],
              [GREY, 'Pas de données'],
            ];

  const metricLabel = METRICS.find((m) => m.key === metric)?.label ?? '';

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        {mode === 'gps' && (
          <>
            <div className="flex gap-1 bg-gray-100 rounded p-0.5">
              {[['3', 'Districts'], ['4', 'Sous-préfectures']].map(([v, l]) => (
                <button key={v} onClick={() => setLevel(v)} className={`px-3 py-1 text-xs rounded ${level === v ? 'bg-gray-800 text-white' : 'text-gray-600'}`}>
                  {l}
                </button>
              ))}
            </div>
            <select value={metric} onChange={(e) => setMetric(e.target.value)} className="border border-gray-300 rounded px-3 py-1.5 text-sm bg-white">
              {METRICS.map((m) => <option key={m.key} value={m.key}>{m.label}</option>)}
            </select>
          </>
        )}
        {mode === 'points' && (
          <label className="flex items-center gap-1.5 text-sm text-gray-700">
            <input type="checkbox" checked={cluster} onChange={(e) => setClusterParam(e.target.checked ? '' : 'off')} className="accent-blue-600" />
            Regrouper les points
            {points && <span className="text-xs text-gray-400 ml-2">{points.features.length.toLocaleString('fr-FR')} structures géolocalisées</span>}
          </label>
        )}
        {error && <span className="text-sm text-red-600">{error}</span>}
      </div>

      <div className="relative rounded-lg overflow-hidden border border-gray-200 bg-white" style={{ height: 'calc(100vh - 220px)', minHeight: '400px' }}>
        <MapContainer center={[10.5, -11.8]} zoom={7} style={{ height: '100%', width: '100%' }} preferCanvas>
          <InvalidateOnResize />
          {mode === 'points' && (
            <TileLayer attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>' url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" />
          )}
          {mode === 'gps' && geo && (
            <GeoJSON
              key={`${level}-${metric}-${breaks.join(',')}`}
              data={geo as unknown as GeoJSON.FeatureCollection}
              style={style as (f?: Feature) => PathOptions}
              onEachFeature={onEachFeature as (f: Feature, l: Layer) => void}
            />
          )}
          {mode === 'points' && points && <ClusterLayer points={markers} cluster={cluster} />}
        </MapContainer>

        <div className="absolute bottom-4 right-4 bg-white rounded-lg shadow-lg p-3 z-[1000] text-xs">
          <h4 className="font-semibold mb-2 text-gray-700">{mode === 'points' ? 'Score qualité' : metricLabel}</h4>
          {legend.map(([c, l]) => (
            <div key={l} className="flex items-center gap-2 text-gray-600 mb-1">
              <span className="inline-block w-4 h-3 rounded-sm border border-gray-300" style={{ background: c }} />
              {l}
            </div>
          ))}
        </div>
        {((mode === 'gps' && !geo) || (mode === 'points' && !points)) && !error && (
          <div className="absolute inset-0 flex items-center justify-center bg-white/60 z-[1000] text-sm text-gray-500">Chargement…</div>
        )}
      </div>
    </div>
  );
}
