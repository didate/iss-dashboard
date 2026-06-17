import { useEffect, useState, useCallback } from 'react';
import { api } from '../api/client';
import type { IssueListResult, IssueListItem, EventDetail, Filters } from '../types';
import SeverityBadge from '../components/SeverityBadge';
import ScoreBar from '../components/ScoreBar';
import ExportCSV from '../components/ExportCSV';
import MethodNote from '../components/MethodNote';

export default function Quality() {
  const [result, setResult] = useState<IssueListResult | null>(null);
  const [filters, setFilters] = useState<Filters | null>(null);
  const [detail, setDetail] = useState<EventDetail | null>(null);
  const [loading, setLoading] = useState(true);

  // Filter state
  const [severity, setSeverity] = useState('');
  const [rule, setRule] = useState('');
  const [district, setDistrict] = useState('');
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);

  const fetchData = useCallback(() => {
    setLoading(true);
    api
      .getQualityIssues({ severity, rule, district, search, page, pageSize: 20 })
      .then(setResult)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [severity, rule, district, search, page]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
  }, []);

  const openDetail = (item: IssueListItem) => {
    api.getEventDetail(item.event_uid).then(setDetail).catch(console.error);
  };

  const items = result?.data ?? [];
  const totalPages = result ? Math.ceil(result.total / result.page_size) : 0;

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Qualité des données</h2>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 bg-white p-3 rounded-lg border border-gray-200 items-center">
        {items.length > 0 && (
          <ExportCSV
            data={items.map((d) => ({ ...d, issues: (d.issues ?? []).map((i) => i.message).join(' | ') })) as unknown as Record<string, unknown>[]}
            columns={[
              { key: 'org_unit_name', header: 'Structure' },
              { key: 'district', header: 'District' },
              { key: 'region', header: 'Region' },
              { key: 'worst_severity', header: 'Severite' },
              { key: 'score', header: 'Score' },
              { key: 'n_error', header: 'Erreurs' },
              { key: 'n_warning', header: 'Avertissements' },
              { key: 'n_info', header: 'Infos' },
              { key: 'issues', header: 'Problemes' },
            ]}
            filename="qualite_issues"
          />
        )}
        <select
          className="border border-gray-300 rounded px-2 py-1.5 text-sm"
          value={severity}
          onChange={(e) => { setSeverity(e.target.value); setPage(1); }}
        >
          <option value="">Toutes sévérités</option>
          <option value="error">Erreur</option>
          <option value="warning">Avertissement</option>
          <option value="info">Info</option>
        </select>

        <select
          className="border border-gray-300 rounded px-2 py-1.5 text-sm"
          value={rule}
          onChange={(e) => { setRule(e.target.value); setPage(1); }}
        >
          <option value="">Toutes regles</option>
          {(filters?.rules ?? []).map((r) => (
            <option key={r.code} value={r.code}>{r.code} — {r.name}</option>
          ))}
        </select>

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
      </div>

      <div className="flex flex-col lg:flex-row gap-4">
        {/* Issues list */}
        <div className="flex-1 min-w-0 bg-white rounded-lg border border-gray-200">
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
                      <th className="text-left px-3 py-2 font-medium text-gray-500">Sévérité</th>
                      <th className="text-left px-3 py-2 font-medium text-gray-500">Score</th>
                      <th className="text-left px-3 py-2 font-medium text-gray-500">E/W/I</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((item) => (
                      <tr
                        key={item.event_uid}
                        className="border-b border-gray-100 cursor-pointer hover:bg-blue-50"
                        onClick={() => openDetail(item)}
                      >
                        <td className="px-3 py-2 font-medium text-gray-800">{item.org_unit_name}</td>
                        <td className="px-3 py-2 text-gray-600">{item.district}</td>
                        <td className="px-3 py-2"><SeverityBadge severity={item.worst_severity} /></td>
                        <td className="px-3 py-2"><ScoreBar score={item.score} /></td>
                        <td className="px-3 py-2 text-xs">
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
                        <td colSpan={5} className="px-3 py-8 text-center text-gray-400">
                          Aucun problème trouvé
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-between px-3 py-2 border-t">
                  <span className="text-xs text-gray-500">
                    {result?.total} résultats — page {page}/{totalPages}
                  </span>
                  <div className="flex gap-1">
                    <button
                      className="px-2 py-1 text-xs border rounded disabled:opacity-40"
                      disabled={page <= 1}
                      onClick={() => setPage(page - 1)}
                    >
                      Préc.
                    </button>
                    <button
                      className="px-2 py-1 text-xs border rounded disabled:opacity-40"
                      disabled={page >= totalPages}
                      onClick={() => setPage(page + 1)}
                    >
                      Suiv.
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Detail panel */}
        {detail && (
          <div className="w-full lg:w-96 bg-white rounded-lg border border-gray-200 p-4 overflow-y-auto max-h-[80vh]">
            <div className="flex justify-between items-start mb-4">
              <div>
                <h3 className="font-bold text-gray-900">{detail.event.org_unit_name}</h3>
                <p className="text-xs text-gray-500">{detail.event.district} — {detail.event.region}</p>
                <p className="text-xs text-gray-400 mt-1">Date: {detail.event.event_date}</p>
              </div>
              <button
                className="text-gray-400 hover:text-gray-600 text-lg"
                onClick={() => setDetail(null)}
              >
                &times;
              </button>
            </div>

            <div className="mb-4">
              <ScoreBar score={detail.quality.score} />
              <div className="flex gap-3 mt-2 text-xs">
                <span className="text-red-600">{detail.quality.n_error} erreurs</span>
                <span className="text-yellow-600">{detail.quality.n_warning} avert.</span>
                <span className="text-blue-600">{detail.quality.n_info} infos</span>
              </div>
            </div>

            <h4 className="font-medium text-gray-700 text-sm mb-2">Problèmes</h4>
            <div className="space-y-2 mb-4">
              {(detail.issues ?? []).map((iss, i) => (
                <div key={i} className="flex gap-2 items-start text-xs">
                  <SeverityBadge severity={iss.severity} />
                  <span className="text-gray-700">{iss.message}</span>
                </div>
              ))}
            </div>

            <h4 className="font-medium text-gray-700 text-sm mb-2">Valeurs clés</h4>
            <div className="max-h-60 overflow-y-auto">
              <table className="w-full text-xs">
                <tbody>
                  {(detail.values ?? []).slice(0, 50).map((v, i) => (
                    <tr key={i} className="border-b border-gray-50">
                      <td className="py-1 text-gray-500 pr-2">{v.de_name}</td>
                      <td className="py-1 font-medium text-gray-800">{v.value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      <MethodNote title="Methodologie - Regles qualite">
        <p><strong>R1 - Champs obligatoires</strong> : date absente (erreur), statut operationnel ou nom du responsable manquant (avertissement).</p>
        <p><strong>R2 - Coherence total/fonctionnel</strong> : pour les 36 couples d'equipements, fonctionnel &gt; total (erreur) ou fonctionnel renseigne mais total manquant (avertissement).</p>
        <p><strong>R3 - Service sans support</strong> : service declare fonctionnel mais equipement/infrastructure de support absent (labo sans microscope, maternite sans table d'accouchement, chirurgie sans table operatoire).</p>
        <p><strong>R4 - Coherence commodites</strong> : energie declaree sans source cochee (avertissement), eau aux points critiques sans source d'eau (info).</p>
        <p><strong>R5 - Valeurs aberrantes</strong> : valeur &gt; mediane + 5xMAD et &gt; 50 en absolu (info).</p>
        <p><strong>R6 - Doublons</strong> : plusieurs evenements actifs sur la meme org unit (avertissement).</p>
        <p><strong>R7 - Completude</strong> : structure « coquille vide » sans aucun equipement ni RH renseigne (info).</p>
      </MethodNote>
    </div>
  );
}
