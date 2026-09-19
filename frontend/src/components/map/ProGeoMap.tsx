import { useCallback, useEffect, useMemo, useState } from 'react';
import { MapContainer, GeoJSON, TileLayer } from 'react-leaflet';
import GeoLabels from './GeoLabels';
import IndicatorHelp from './IndicatorHelp';
import ConakryInset, { isConakry } from './ConakryInset';
import type { Layer, PathOptions } from 'leaflet';
import type { Feature, Geometry } from 'geojson';
import { api } from '../../api/client';
import { useUrlState } from '../../hooks/useUrlState';
import type { MapGeoCollection, MapGeoFeature, ProPointCollection } from '../../types';
import { typologieLabel } from '../../utils/typologie';
import PointsCanvasLayer, { escapeHtml, type MarkerSpec } from './PointsCanvasLayer';
import InvalidateOnResize from './InvalidateOnResize';

type Props = { mode: 'gps' | 'points' };

interface Metric {
  key: string;
  label: string;
  unit: string;
  /** Explication affichée dans le panneau pliable de la carte (définition, calcul, lecture). */
  help: string[];
}

const RATIO_HELP = (quoi: string, source: string) => [
  `Nombre de ${quoi} pour 10 000 habitants de l'unité administrative.`,
  `Calcul : ${source} ÷ population de l'unité × 10 000. Une structure est comptée une fois (dernier recensement).`,
  'Population : data set DHIS2 SIS_POPULATION, dernière période mensuelle renseignée × 12. Gris = population inconnue.',
  'Lecture : échelle à quantiles (5 classes calculées sur les unités affichées), du rouge (les 20 % les moins dotés) au vert (les 20 % les mieux dotés). Les seuils changent donc selon le niveau et le filtre.',
];

const METRICS: Metric[] = [
  {
    key: 'pct_gps', label: 'Couverture GPS', unit: '%',
    help: [
      "Part des structures recensées dont l'unité d'organisation DHIS2 porte un point GPS.",
      'Calcul : structures avec coordonnées ÷ structures recensées de l\'unité × 100.',
      'Lecture : vert ≥ 80 %, jaune 50–80 %, rouge < 50 %. Les structures sans point n\'apparaissent pas sur la carte des points ; la liste à saisir est dans la page GPS.',
    ],
  },
  {
    key: 'avg_score', label: 'Score qualité moyen', unit: '',
    help: [
      'Moyenne du score qualité des données des structures de l\'unité (0–100).',
      'Score par structure : 100 − 15 par erreur − 5 par avertissement − 1 par info, plancher 0 (règles R1–R18).',
      'Lecture : vert ≥ 80, jaune 65–80, orange 50–65, rouge < 50. Mesure la fiabilité des données saisies, pas l\'état de la structure.',
    ],
  },
  {
    key: 'n_structures', label: 'Nombre de structures', unit: '',
    help: ['Nombre de structures recensées rattachées à l\'unité (une par unité d\'organisation, dernier recensement).', 'Lecture : échelle à quantiles sur les unités affichées.'],
  },
  {
    key: 'ratio_structures_10k', label: 'Structures pour 10 000 hab.', unit: '',
    help: RATIO_HELP('structures sanitaires', 'structures recensées'),
  },
  // ratios de couverture (usage_couverture) : clé = ratio:<indicateur>
  { key: 'ratio:personnel_soignant', label: 'Personnel soignant pour 10 000 hab.', unit: '', help: RATIO_HELP('soignants (médecins, sages-femmes, infirmiers, ATS)', 'effectifs déclarés, tous statuts d\'emploi') },
  { key: 'ratio:medecins', label: 'Médecins pour 10 000 hab.', unit: '', help: RATIO_HELP('médecins (généralistes et spécialistes)', 'effectifs déclarés ISS_RH_MED_*, tous statuts') },
  { key: 'ratio:sages_femmes', label: 'Sages-femmes pour 10 000 hab.', unit: '', help: RATIO_HELP('sages-femmes', 'effectifs déclarés, tous statuts') },
  { key: 'ratio:infirmiers', label: 'Infirmiers pour 10 000 hab.', unit: '', help: RATIO_HELP('infirmiers', 'effectifs déclarés, tous statuts') },
  { key: 'ratio:ats', label: 'ATS pour 10 000 hab.', unit: '', help: RATIO_HELP('agents techniques de santé', 'effectifs déclarés, tous statuts') },
  { key: 'ratio:lits', label: "Lits d'hospitalisation pour 10 000 hab.", unit: '', help: RATIO_HELP('lits', 'nombre total de lits déclarés') },
  // conformité aux normes (référentiel actif)
  {
    key: 'conformite_score', label: 'Score de conformité aux normes', unit: '',
    help: [
      'Moyenne, sur les structures de l\'unité, du score de conformité au référentiel de normes actif (0–100).',
      'Score par structure : 100 × Σ poids des exigences satisfaites ÷ Σ poids des exigences satisfaites + manquantes ; poids 2 pour une exigence essentielle, 1 pour une recommandée. Une exigence non renseignée dans ISS n\'entre pas dans le calcul.',
      'Lecture : vert ≥ 80, jaune 65–80, orange 50–65, rouge < 50. Un score élevé n\'implique pas la conformité : il suffit d\'une exigence essentielle manquante pour ne pas être conforme.',
      'Dépend du référentiel actif (Admin → Normes) — avec l\'exemple non officiel, les seuils sont indicatifs.',
    ],
  },
  {
    key: 'pct_conformes', label: '% de structures conformes', unit: '%',
    help: [
      'Part des structures de l\'unité sans aucune exigence essentielle manquante (référentiel actif).',
      'Calcul : structures conformes ÷ structures évaluées × 100.',
      'Lecture : vert ≥ 80 %, jaune 50–80 %, orange 20–50 %, rouge < 20 %. Critère binaire et sévère : une seule exigence essentielle absente suffit.',
      'Un « manque » peut aussi être une erreur de saisie (valeur 0 dans ISS) : vérifier dans le détail de la structure avant de conclure.',
    ],
  },
];

const POINTS_HELP = [
  'Chaque cercle est une structure géolocalisée (dernier recensement), colorée par son score qualité des données : vert ≥ 80, jaune 65–80, orange 50–65, rouge < 50.',
  'Le score mesure la fiabilité de la saisie ISS (règles R1–R18), pas l\'état de la structure. Clic sur un point → score et nombre de problèmes → détail.',
  'Les structures sans coordonnées GPS ne sont pas représentées (voir la page GPS). Le regroupement en clusters est optionnel (case « Regrouper les points »).',
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

function formatMetric(v: number | null, key: string): string {
  if (v === null) return '—';
  if (key === 'pct_gps' || key === 'pct_conformes') return `${v.toFixed(0)}%`;
  if (key === 'n_structures') return String(Math.round(v));
  if (key === 'avg_score' || key === 'conformite_score') return v.toFixed(0);
  return v.toFixed(2);
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
  const cluster = clusterParam === 'on'; // désactivé par défaut ; ?cluster=on pour regrouper

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

  const isPct = metric === 'pct_gps' || metric === 'avg_score' || metric === 'conformite_score' || metric === 'pct_conformes';
  const breaks = useMemo(() => {
    if (!geo || isPct) return [];
    return quantileBreaks(geo.features.map((f) => metricValue(f.properties, metric) ?? 0), 5);
  }, [geo, metric, isPct]);

  const colorOf = useCallback(
    (v: number | null): string => {
      if (v === null) return GREY;
      if (metric === 'pct_gps') return v < 50 ? '#ef4444' : v < 80 ? '#eab308' : '#22c55e';
      if (metric === 'avg_score' || metric === 'conformite_score') return scoreColor(v);
      if (metric === 'pct_conformes') return v < 20 ? '#ef4444' : v < 50 ? '#f97316' : v < 80 ? '#eab308' : '#22c55e';
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
        ${p.conformite_score != null ? `<div>Conformité aux normes : <b>${p.conformite_score.toFixed(0)}</b> · ${p.pct_conformes?.toFixed(0) ?? '—'}% conformes</div>` : ''}
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
          popup: () => `
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
        : metric === 'avg_score' || metric === 'conformite_score'
          ? [['#22c55e', '≥ 80'], ['#eab308', '65 – 80'], ['#f97316', '50 – 65'], ['#ef4444', '< 50'], [GREY, 'Pas de données']]
          : metric === 'pct_conformes'
            ? [['#22c55e', '≥ 80 %'], ['#eab308', '50 – 80 %'], ['#f97316', '20 – 50 %'], ['#ef4444', '< 20 %'], [GREY, 'Pas de données']]
          : [
              ...breaks.map((b, i) => [RAMP[i], `≤ ${b.toFixed(metric === 'n_structures' ? 0 : 2)}`]),
              [RAMP[RAMP.length - 1], `> ${(breaks[breaks.length - 1] ?? 0).toFixed(metric === 'n_structures' ? 0 : 2)}`],
              [GREY, 'Pas de données'],
            ];

  // Communes de Conakry (niveau 3) ou leurs sous-préfectures (niveau 4, via le district parent)
  const conakry = useMemo(
    () => (geo ? { type: 'FeatureCollection' as const, features: geo.features.filter((f) => isConakry(f.properties.name, f.properties.parent_name)) } : null),
    [geo],
  );

  const metricDef = METRICS.find((m) => m.key === metric);
  const metricLabel = metricDef?.label ?? '';
  const helpTitle = mode === 'points' ? 'Structures (points) — score qualité' : metricLabel;
  const helpLines = mode === 'points' ? POINTS_HELP : metricDef?.help ?? [];

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
            <input type="checkbox" checked={cluster} onChange={(e) => setClusterParam(e.target.checked ? 'on' : '')} className="accent-blue-600" />
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
            <>
              <GeoJSON
                key={`${level}-${metric}-${breaks.join(',')}`}
                data={geo as unknown as GeoJSON.FeatureCollection}
                style={style as (f?: Feature) => PathOptions}
                onEachFeature={onEachFeature as (f: Feature, l: Layer) => void}
              />
              <GeoLabels
                features={geo.features}
                minZoom={level === '4' ? 9 : 0}
                text={(p) => formatMetric(metricValue(p, metric), metric)}
              />
            </>
          )}
          {mode === 'points' && points && <PointsCanvasLayer points={markers} cluster={cluster} />}
        </MapContainer>

        <IndicatorHelp title={helpTitle} lines={helpLines} />

        {mode === 'gps' && conakry && conakry.features.length > 0 && (
          <ConakryInset mapKey={`${level}-${metric}-${breaks.join(',')}`}>
            <GeoJSON
              data={conakry as unknown as GeoJSON.FeatureCollection}
              style={style as (f?: Feature) => PathOptions}
              onEachFeature={onEachFeature as (f: Feature, l: Layer) => void}
            />
            <GeoLabels features={conakry.features} text={(p) => formatMetric(metricValue(p, metric), metric)} small />
          </ConakryInset>
        )}

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
