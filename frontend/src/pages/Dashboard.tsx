import { useEffect, useState } from 'react';
import { MapContainer, GeoJSON } from 'react-leaflet';
import type { PathOptions } from 'leaflet';
import type { Feature, Geometry } from 'geojson';
import { Activity, AlertTriangle, AlertCircle, Users, Zap, Droplets, Clock, TrendingUp, TrendingDown } from 'lucide-react';
import MethodNote from '../components/MethodNote';
import { api } from '../api/client';
import type { Summary, QualitySummaryRow, ReportingRate, RHSummaryResult, UsageCommodite, UsageRecensement, MapDistrictCollection } from '../types';
import KpiCard from '../components/KpiCard';

export default function Dashboard() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [districtScores, setDistrictScores] = useState<QualitySummaryRow[]>([]);
  const [reportingGlobal, setReportingGlobal] = useState<ReportingRate | null>(null);
  const [reportingDistrict, setReportingDistrict] = useState<ReportingRate[]>([]);
  const [rhSummary, setRHSummary] = useState<RHSummaryResult | null>(null);
  const [commodites, setCommodites] = useState<UsageCommodite[]>([]);
  const [recensementStatut, setRecensementStatut] = useState<UsageRecensement[]>([]);
  const [mapData, setMapData] = useState<MapDistrictCollection | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getSummary(),
      api.getQualitySummary('district'),
      api.getReportingRate('global'),
      api.getReportingRate('district'),
      api.getRHSummary(''),
      api.getUsageCommodites(''),
      api.getUsageRecensement('statut_juridique'),
      api.getMapData(),
    ])
      .then(([s, d, rr, rd, rh, co, rec, md]) => {
        setSummary(s);
        setDistrictScores(d ?? []);
        const global = (rr ?? []).find((x) => x.key === 'all');
        if (global) setReportingGlobal(global);
        setReportingDistrict(rd ?? []);
        setRHSummary(rh);
        setCommodites(co ?? []);
        setRecensementStatut(rec ?? []);
        setMapData(md);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="text-gray-400 p-8">Chargement...</div>;
  if (!summary) return <div className="text-gray-400 p-8">Aucune donnée. Lancez une synchronisation.</div>;

  const lastSyncDate = summary.last_sync?.finished_at
    ? new Date(summary.last_sync.finished_at).toLocaleString('fr-FR')
    : 'Jamais';

  const pctOper = summary.n_structures > 0 ? (summary.n_operationnel / summary.n_structures * 100) : 0;
  const energie = commodites.find(c => c.indicator === 'energie');
  const eau = commodites.find(c => c.indicator === 'eau_pts_critiques');

  // Top 5 / Bottom 5 districts by reporting
  const sortedDistricts = [...reportingDistrict].sort((a, b) => a.pct - b.pct);
  const bottom5 = sortedDistricts.slice(0, 5);
  const top5 = sortedDistricts.slice(-5).reverse();

  // Statut juridique — group into publique/privee then sub-categories
  const publicStatuts = ['publique', 'parapublique'];
  const privateStatuts = ['privélucratif', 'confessionnel', 'associatif', 'non lucratif', 'privée'];
  const nPublic = recensementStatut.filter(r => publicStatuts.includes(r.label.toLowerCase())).reduce((s, r) => s + r.n_structures, 0);
  const nPrivate = recensementStatut.filter(r => privateStatuts.includes(r.label.toLowerCase())).reduce((s, r) => s + r.n_structures, 0);

  const statutColorMap: Record<string, string> = {
    publique: '#3b82f6',
    parapublique: '#06b6d4',
    'privélucratif': '#ef4444',
    confessionnel: '#8b5cf6',
    associatif: '#10b981',
    'non lucratif': '#f59e0b',
    'privée': '#f97316',
  };
  const statutLabelMap: Record<string, string> = {
    publique: 'Publique',
    parapublique: 'Parapublique',
    'privélucratif': 'Privé lucratif',
    confessionnel: 'Confessionnel',
    associatif: 'Associatif',
    'non lucratif': 'Non lucratif',
    'privée': 'Privé (non précisé)',
  };
  const statutData = recensementStatut
    .filter(r => r.n_structures > 0)
    .map(r => ({
      name: statutLabelMap[r.label.toLowerCase()] || r.label,
      value: r.n_structures,
      fill: statutColorMap[r.label.toLowerCase()] || '#9ca3af',
    }));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-gray-900">Vue d'ensemble</h2>
        <div className="flex items-center gap-1 text-xs text-gray-400">
          <Clock size={12} />
          Dernière synchro : {lastSyncDate}
        </div>
      </div>

      {/* Row 1: Main KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <KpiCard
          title="Structures analysées"
          value={summary.n_structures}
          icon={<Activity size={20} />}
        />
        <KpiCard
          title="Taux de rapportage"
          value={reportingGlobal ? `${reportingGlobal.pct.toFixed(1)}%` : '-'}
          subtitle={reportingGlobal ? `${reportingGlobal.n_reported} / ${reportingGlobal.n_expected}` : ''}
          color={reportingGlobal && reportingGlobal.pct >= 80 ? 'text-green-600' : 'text-yellow-600'}
          icon={<Activity size={20} />}
        />
        <KpiCard
          title="Score qualité moyen"
          value={summary.avg_score.toFixed(1)}
          subtitle={`${summary.n_error} err, ${summary.n_warning} avert.`}
          color={summary.avg_score >= 80 ? 'text-green-600' : summary.avg_score >= 50 ? 'text-yellow-600' : 'text-red-600'}
          icon={<AlertCircle size={20} />}
        />
        <KpiCard
          title="Structures opérationnelles"
          value={`${pctOper.toFixed(0)}%`}
          subtitle={`${summary.n_operationnel.toLocaleString('fr-FR')} sur ${summary.n_structures.toLocaleString('fr-FR')}`}
          color={pctOper >= 90 ? 'text-green-600' : pctOper >= 70 ? 'text-yellow-600' : 'text-red-600'}
          icon={<Activity size={20} />}
        />
      </div>

      {/* Row 2: RH + WASH KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        <KpiCard
          title="Médecins / structure"
          value={rhSummary?.ratio_med_per_structure?.toFixed(2) ?? '-'}
          subtitle={`${rhSummary?.n_structures_sans_medecin ?? 0} structures sans médecin`}
          color={rhSummary && rhSummary.ratio_med_per_structure >= 1 ? 'text-green-600' : 'text-red-600'}
          icon={<Users size={20} />}
        />
        <KpiCard
          title="Effectif RH total"
          value={rhSummary?.total_effectif?.toLocaleString('fr-FR') ?? '0'}
          subtitle={`${rhSummary?.total_fonc ?? 0} fonc. / ${rhSummary?.total_contr ?? 0} contr. / ${rhSummary?.total_benev ?? 0} benev.`}
          icon={<Users size={20} />}
        />
        <KpiCard
          title="Structures avec énergie"
          value={energie ? `${energie.pct.toFixed(0)}%` : '-'}
          subtitle={energie ? `${energie.n_oui} / ${energie.n_total}` : ''}
          color={energie && energie.pct >= 80 ? 'text-green-600' : 'text-yellow-600'}
          icon={<Zap size={20} />}
        />
        <KpiCard
          title="Eau aux points critiques"
          value={eau ? `${eau.pct.toFixed(0)}%` : '-'}
          subtitle={eau ? `${eau.n_oui} / ${eau.n_total}` : ''}
          color={eau && eau.pct >= 80 ? 'text-green-600' : 'text-yellow-600'}
          icon={<Droplets size={20} />}
        />
      </div>

      {/* Row 3: Map + Donut + Top/Bottom */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Mini map */}
        {mapData && mapData.features.length > 0 && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <h3 className="text-sm font-semibold text-gray-700 mb-2">Rapportage par district</h3>
            <div style={{ height: '260px', borderRadius: '6px', overflow: 'hidden' }}>
              <MapContainer
                center={[10.0, -11.5]}
                zoom={5.7}
                style={{ height: '100%', width: '100%', background: '#ffffff' }}
                scrollWheelZoom={false}
                dragging={false}
                zoomControl={false}
                attributionControl={false}
              >
                <GeoJSON
                  data={mapData as unknown as GeoJSON.FeatureCollection}
                  style={(feature?: Feature<Geometry>) => {
                    const pct = feature?.properties?.rapportage_pct;
                    let fillColor = '#d1d5db';
                    if (pct !== null && pct !== undefined) {
                      fillColor = pct >= 80 ? '#22c55e' : pct >= 50 ? '#eab308' : pct >= 30 ? '#f97316' : '#ef4444';
                    }
                    return { fillColor, weight: 1, color: '#374151', fillOpacity: 0.7 } as PathOptions;
                  }}
                />
              </MapContainer>
            </div>
            <div className="flex justify-center gap-3 mt-2 text-[10px] text-gray-500">
              <span><span className="inline-block w-2.5 h-2.5 rounded-sm mr-1" style={{ background: '#22c55e' }} />&gt;80%</span>
              <span><span className="inline-block w-2.5 h-2.5 rounded-sm mr-1" style={{ background: '#eab308' }} />50-80%</span>
              <span><span className="inline-block w-2.5 h-2.5 rounded-sm mr-1" style={{ background: '#f97316' }} />30-50%</span>
              <span><span className="inline-block w-2.5 h-2.5 rounded-sm mr-1" style={{ background: '#ef4444' }} />&lt;30%</span>
            </div>
          </div>
        )}

        {/* Statut juridique */}
        {(nPublic > 0 || nPrivate > 0) && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <h3 className="text-sm font-semibold text-gray-700 mb-3">Répartition par statut</h3>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {/* Public: KPI + detail */}
              <div>
                <div className="bg-blue-50 rounded-lg p-3 text-center mb-2">
                  <div className="text-xs text-gray-500">Publique</div>
                  <div className="text-xl font-bold text-blue-600">{nPublic.toLocaleString('fr-FR')}</div>
                </div>
                {statutData.filter(s => ['Publique', 'Parapublique'].includes(s.name)).sort((a, b) => b.value - a.value).map(s => (
                  <div key={s.name} className="flex items-center justify-between text-xs py-0.5">
                    <span className="text-gray-600">{s.name}</span>
                    <span className="font-medium text-gray-800">{s.value.toLocaleString('fr-FR')}</span>
                  </div>
                ))}
              </div>
              {/* Private: KPI + detail */}
              <div>
                <div className="bg-red-50 rounded-lg p-3 text-center mb-2">
                  <div className="text-xs text-gray-500">Privée</div>
                  <div className="text-xl font-bold text-red-600">{nPrivate.toLocaleString('fr-FR')}</div>
                </div>
                {statutData.filter(s => !['Publique', 'Parapublique'].includes(s.name)).sort((a, b) => b.value - a.value).map(s => (
                  <div key={s.name} className="flex items-center justify-between text-xs py-0.5">
                    <span className="text-gray-600">{s.name}</span>
                    <span className="font-medium text-gray-800">{s.value.toLocaleString('fr-FR')}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Top 5 / Bottom 5 */}
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <h3 className="text-sm font-semibold text-gray-700 mb-3">Districts — rapportage</h3>

          <div className="mb-4">
            <div className="flex items-center gap-1 text-xs font-medium text-green-700 mb-2">
              <TrendingUp size={14} /> Top 5
            </div>
            {top5.map((d, i) => (
              <div key={d.key} className="flex items-center justify-between text-xs py-1 border-b border-gray-50">
                <span className="text-gray-700">{i + 1}. {d.label}</span>
                <span className="font-semibold text-green-600">{d.pct.toFixed(0)}%</span>
              </div>
            ))}
          </div>

          <div>
            <div className="flex items-center gap-1 text-xs font-medium text-red-700 mb-2">
              <TrendingDown size={14} /> Bottom 5
            </div>
            {bottom5.map((d, i) => (
              <div key={d.key} className="flex items-center justify-between text-xs py-1 border-b border-gray-50">
                <span className="text-gray-700">{i + 1}. {d.label}</span>
                <span className="font-semibold text-red-600">{d.pct.toFixed(0)}%</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Row 4: Score by district (existing) */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Top 5 / Bottom 5 by quality score */}
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <h3 className="text-sm font-semibold text-gray-700 mb-3">Districts — score qualité</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <div className="flex items-center gap-1 text-xs font-medium text-green-700 mb-2">
                <TrendingUp size={14} /> Meilleurs scores
              </div>
              {[...districtScores].sort((a, b) => b.avg_score - a.avg_score).slice(0, 5).map((d, i) => (
                <div key={d.key} className="flex items-center justify-between text-xs py-1 border-b border-gray-50">
                  <span className="text-gray-700">{i + 1}. {d.label}</span>
                  <span className="font-semibold text-green-600">{d.avg_score.toFixed(0)}</span>
                </div>
              ))}
            </div>
            <div>
              <div className="flex items-center gap-1 text-xs font-medium text-red-700 mb-2">
                <TrendingDown size={14} /> Scores les plus bas
              </div>
              {[...districtScores].sort((a, b) => a.avg_score - b.avg_score).slice(0, 5).map((d, i) => (
                <div key={d.key} className="flex items-center justify-between text-xs py-1 border-b border-gray-50">
                  <span className="text-gray-700">{i + 1}. {d.label}</span>
                  <span className="font-semibold text-red-600">{d.avg_score.toFixed(0)}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Issues summary */}
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <h3 className="text-sm font-semibold text-gray-700 mb-3">Problèmes qualité</h3>
          <div className="grid grid-cols-3 gap-4 mb-4">
            <div className="text-center">
              <div className="flex items-center justify-center gap-1">
                <AlertCircle size={16} className="text-red-500" />
                <span className="text-2xl font-bold text-red-600">{summary.n_error.toLocaleString('fr-FR')}</span>
              </div>
              <div className="text-xs text-gray-500">Erreurs</div>
            </div>
            <div className="text-center">
              <div className="flex items-center justify-center gap-1">
                <AlertTriangle size={16} className="text-yellow-500" />
                <span className="text-2xl font-bold text-yellow-600">{summary.n_warning.toLocaleString('fr-FR')}</span>
              </div>
              <div className="text-xs text-gray-500">Avertissements</div>
            </div>
            <div className="text-center">
              <div className="flex items-center justify-center gap-1">
                <Activity size={16} className="text-blue-500" />
                <span className="text-2xl font-bold text-blue-600">{summary.n_info.toLocaleString('fr-FR')}</span>
              </div>
              <div className="text-xs text-gray-500">Infos</div>
            </div>
          </div>
          <div className="text-xs text-gray-400 text-center">
            Score = 100 - (15 x erreurs) - (5 x avertissements) - (1 x infos)
          </div>
        </div>
      </div>

      <MethodNote title="Méthodologie - Score qualité">
        <p><strong>Score qualité</strong> = 100 - (15 x erreurs) - (5 x avertissements) - (1 x infos), plancher à 0.</p>
        <p><strong>Erreurs</strong> (-15 pts) : date absente, fonctionnel &gt; total pour un équipement.</p>
        <p><strong>Avertissements</strong> (-5 pts) : champ obligatoire manquant, service sans support, énergie sans source, doublons.</p>
        <p><strong>Infos</strong> (-1 pt) : eau sans source, structure coquille vide.</p>
        <p><strong>Taux de rapportage</strong> = structures ayant soumis / structures assignées au programme (hors fermées).</p>
        <p><strong>Médecins / structure</strong> = total médecins (généralistes + spécialistes) / nombre de structures.</p>
      </MethodNote>
    </div>
  );
}
