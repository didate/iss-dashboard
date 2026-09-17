import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { publicApi } from '../../api/public';
import type { PublicSummary } from '../../types';

export default function About() {
  const [summary, setSummary] = useState<PublicSummary | null>(null);
  useEffect(() => {
    publicApi.getSummary().then(setSummary).catch(() => setSummary(null));
  }, []);

  const date = summary?.derniere_synchro ? new Date(summary.derniere_synchro).toLocaleDateString('fr-FR') : '—';

  return (
    <div className="max-w-2xl mx-auto p-6 text-gray-700 leading-relaxed">
      <h1 className="text-2xl font-semibold text-gray-900 mb-4">À propos de la carte sanitaire</h1>

      <p className="mb-4">
        Cette carte présente les structures de santé recensées par le Ministère de la Santé et de l’Hygiène Publique
        dans le cadre du recensement <strong>ISS</strong> (Informations des Structures Sanitaires), saisi dans la
        plateforme nationale DHIS2. Pour chaque structure, elle indique son type, son statut, sa localisation et les
        services déclarés fonctionnels au moment du recensement.
      </p>

      {summary && (
        <div className="grid grid-cols-3 gap-3 my-6">
          {[
            [summary.n_structures.toLocaleString('fr-FR'), 'structures recensées'],
            [`${summary.pct_gps}%`, 'géolocalisées'],
            [date, 'dernière mise à jour'],
          ].map(([v, l]) => (
            <div key={l} className="rounded-lg border border-gray-200 p-3 text-center">
              <div className="text-lg font-semibold text-gray-900">{v}</div>
              <div className="text-xs text-gray-500">{l}</div>
            </div>
          ))}
        </div>
      )}

      <h2 className="text-base font-semibold text-gray-900 mt-6 mb-2">Ce que vous pouvez faire</h2>
      <ul className="list-disc pl-5 space-y-1 mb-4">
        <li>Rechercher une structure par son nom, son type, sa région ou son district.</li>
        <li>Trouver les structures proches de vous qui offrent un service donné (maternité, laboratoire, urgences…).</li>
        <li>Partager la fiche d’une structure par lien ou par QR code.</li>
      </ul>

      <h2 className="text-base font-semibold text-gray-900 mt-6 mb-2">Limites</h2>
      <ul className="list-disc pl-5 space-y-1 mb-4">
        <li>Les informations reflètent la situation déclarée lors du recensement, pas la situation en temps réel.</li>
        <li>
          Une partie des structures n’est pas encore géolocalisée ; elles apparaissent dans les résultats de recherche mais
          pas sur la carte.
        </li>
        <li>Les coordonnées peuvent être approximatives. En cas d’erreur, signalez-la à la direction préfectorale de la santé.</li>
      </ul>

      <p className="text-sm text-gray-500 mt-8">
        Les agents du Ministère disposent d’un{' '}
        <Link to="/login" className="text-emerald-700 hover:underline">
          espace planification
        </Link>{' '}
        avec les indicateurs détaillés (qualité des données, ressources humaines, équipements, couverture).
      </p>
    </div>
  );
}
