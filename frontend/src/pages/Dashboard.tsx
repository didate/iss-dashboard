import { useEffect, useState } from 'react';
import { Activity, AlertTriangle, AlertCircle, Info, Clock } from 'lucide-react';
import MethodNote from '../components/MethodNote';
import { api } from '../api/client';
import type { Summary, QualitySummaryRow } from '../types';
import KpiCard from '../components/KpiCard';
import ScoreByDistrict from '../components/charts/ScoreByDistrict';
import IssuesByRule from '../components/charts/IssuesByRule';

export default function Dashboard() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [districtScores, setDistrictScores] = useState<QualitySummaryRow[]>([]);
  const [regionScores, setRegionScores] = useState<QualitySummaryRow[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getSummary(),
      api.getQualitySummary('district'),
      api.getQualitySummary('region'),
    ])
      .then(([s, d, r]) => {
        setSummary(s);
        setDistrictScores(d);
        setRegionScores(r);
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className="text-gray-400 p-8">Chargement...</div>;
  }

  if (!summary) {
    return <div className="text-gray-400 p-8">Aucune donnée. Lancez une synchronisation.</div>;
  }

  const lastSyncDate = summary.last_sync?.finished_at
    ? new Date(summary.last_sync.finished_at).toLocaleString('fr-FR')
    : 'Jamais';

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-bold text-gray-900">Vue d'ensemble</h2>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <KpiCard
          title="Structures analysées"
          value={summary.n_structures}
          icon={<Activity size={20} />}
        />
        <KpiCard
          title="Score qualité moyen"
          value={summary.avg_score.toFixed(1)}
          color={summary.avg_score >= 80 ? 'text-green-600' : summary.avg_score >= 50 ? 'text-yellow-600' : 'text-red-600'}
          icon={<Activity size={20} />}
        />
        <KpiCard
          title="Erreurs"
          value={summary.n_error}
          color="text-red-600"
          icon={<AlertCircle size={20} />}
        />
        <KpiCard
          title="Avertissements"
          value={summary.n_warning}
          color="text-yellow-600"
          icon={<AlertTriangle size={20} />}
        />
        <KpiCard
          title="Infos"
          value={summary.n_info}
          color="text-blue-600"
          icon={<Info size={20} />}
        />
        <KpiCard
          title="Structures opérationnelles"
          value={summary.n_operationnel}
          subtitle={`sur ${summary.n_structures}`}
        />
        <KpiCard
          title="Dernière synchro"
          value={lastSyncDate}
          subtitle={summary.last_sync ? `${summary.last_sync.duration_ms}ms` : undefined}
          icon={<Clock size={20} />}
        />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <ScoreByDistrict data={districtScores} />
        <IssuesByRule data={regionScores} />
      </div>

      <MethodNote title="Methodologie - Score qualite">
        <p><strong>Score qualite</strong> = 100 - (15 x erreurs) - (5 x avertissements) - (1 x infos), plancher a 0.</p>
        <p><strong>Erreurs</strong> (-15 pts) : date absente, fonctionnel &gt; total pour un equipement.</p>
        <p><strong>Avertissements</strong> (-5 pts) : champ obligatoire manquant, service sans support, energie sans source, doublons.</p>
        <p><strong>Infos</strong> (-1 pt) : valeur aberrante, eau sans source, structure coquille vide.</p>
        <p>Le score moyen est la moyenne arithmetique des scores de toutes les structures de la dimension (district, region, etc.).</p>
      </MethodNote>
    </div>
  );
}
