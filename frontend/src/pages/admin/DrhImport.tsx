import { useCallback, useEffect, useRef, useState } from 'react';
import { Upload, Download, Trash2, CheckCircle, AlertTriangle, Users } from 'lucide-react';
import { api } from '../../api/client';
import type { DrhImport as DrhImportRow, DrhImportResult, DrhInconnu, NormeLineError } from '../../types';

const sourceLabels: Record<string, string> = {
  fichier: "identifiant DHIS2 du fichier",
  inconnu: 'sans identifiant exploitable',
};

/** Écran d'import du fichier annuel du personnel de l'État (DRH/CNPS).
 *  Le fichier source est nominatif : seul le CSV normalisé décrit dans
 *  docs/drh-format.md est accepté, et seuls des agrégats sont conservés. */
export default function DrhImport() {
  const [imports, setImports] = useState<DrhImportRow[]>([]);
  const [correspondances, setCorrespondances] = useState(0);
  const [annee, setAnnee] = useState(new Date().getFullYear());
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<DrhImportResult | null>(null);
  const [inconnus, setInconnus] = useState<DrhInconnu[]>([]);
  const [error, setError] = useState('');
  const [lineErrors, setLineErrors] = useState<NormeLineError[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);
  const corrRef = useRef<HTMLInputElement>(null);

  const refresh = useCallback(() => {
    api.getDrhImports().then(setImports).catch(() => {});
    api.getDrhCorrespondances().then((c) => setCorrespondances(c.length)).catch(() => {});
  }, []);

  useEffect(refresh, [refresh]);

  const active = imports.find((i) => i.status === 'active');

  useEffect(() => {
    if (!active) { setInconnus([]); return; }
    api.getDrhNonReconnus(active.id).then(setInconnus).catch(() => {});
  }, [active]);

  const reset = () => { setError(''); setLineErrors([]); setResult(null); };

  const handleImport = async (file: File) => {
    reset();
    setBusy(true);
    try {
      setResult(await api.importDrhFile(file, { annee }));
      refresh();
    } catch (e) {
      const err = e as Error & { errors?: NormeLineError[] };
      setError(err.message);
      setLineErrors(err.errors ?? []);
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  const handleCorrespondances = async (file: File) => {
    reset();
    setBusy(true);
    try {
      const r = await api.importDrhCorrespondances(file);
      setCorrespondances(r.imported);
      refresh();
    } catch (e) {
      const err = e as Error & { errors?: NormeLineError[] };
      setError(err.message);
      setLineErrors(err.errors ?? []);
    } finally {
      setBusy(false);
      if (corrRef.current) corrRef.current.value = '';
    }
  };

  const pct = (n: number, total: number) => (total ? ((100 * n) / total).toFixed(1) : '0.0');

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <h3 className="font-medium text-gray-900 flex items-center gap-2"><Users size={16} /> Personnel de l'État (DRH/CNPS)</h3>
        <p className="text-sm text-gray-500 mt-1">
          Effectif <strong>inscrit au fichier DRH/CNPS</strong>, à ne pas confondre avec l'effectif présent déclaré par les
          structures dans ISS. Le fichier attendu est le CSV normalisé décrit dans <code>docs/drh-format.md</code> :
          il ne contient ni matricule, ni nom, ni date de naissance exacte, et seuls des effectifs agrégés sont
          conservés en base. Le <code>.csv.gz</code> est accepté : utile quand le serveur web limite la taille
          des envois (le fichier annuel fait ~1,4 Mo, ~70 Ko compressé).
        </p>

        <div className="flex flex-wrap items-end gap-3 mt-4">
          <label className="text-sm">
            <span className="block text-gray-600 mb-1">Millésime</span>
            <input
              type="number" value={annee} min={2000} max={2100}
              onChange={(e) => setAnnee(Number(e.target.value))}
              className="w-24 border border-gray-300 rounded px-2 py-1.5 text-sm"
            />
          </label>
          <input
            ref={fileRef} type="file" accept=".csv,.gz,text/csv,application/gzip" className="hidden"
            onChange={(e) => { const f = e.target.files?.[0]; if (f) handleImport(f); }}
          />
          <button
            onClick={() => fileRef.current?.click()} disabled={busy}
            className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            <Upload size={16} /> {busy ? 'Import en cours…' : 'Importer un fichier'}
          </button>
          <span className="text-sm text-gray-500">
            {correspondances} correspondances DRH → ISS
          </span>
          <input
            ref={corrRef} type="file" accept=".csv,.gz,text/csv,application/gzip" className="hidden"
            onChange={(e) => { const f = e.target.files?.[0]; if (f) handleCorrespondances(f); }}
          />
          <button
            onClick={() => corrRef.current?.click()} disabled={busy}
            className="flex items-center gap-1 px-3 py-2 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200 disabled:opacity-50"
          >
            <Upload size={14} /> Remplacer les correspondances
          </button>
          <button
            onClick={() => api.downloadDrhCSV('/api/admin/drh/correspondances/export.csv', 'drh_correspondances.csv')}
            className="flex items-center gap-1 px-3 py-2 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
          >
            <Download size={14} /> Exporter
          </button>
        </div>

        {error && (
          <div className="mt-4 bg-red-50 border border-red-200 rounded p-3 text-sm text-red-700">
            <p className="font-medium flex items-center gap-1"><AlertTriangle size={14} /> {error}</p>
            {lineErrors.length > 0 && (
              <ul className="mt-2 space-y-0.5 max-h-40 overflow-auto font-mono text-xs">
                {lineErrors.slice(0, 20).map((e, i) => <li key={i}>ligne {e.line} : {e.message}</li>)}
                {lineErrors.length > 20 && <li>… {lineErrors.length - 20} autres</li>}
              </ul>
            )}
          </div>
        )}

        {result && (
          <div className="mt-4 bg-green-50 border border-green-200 rounded p-3 text-sm text-green-800">
            <p className="font-medium flex items-center gap-1">
              <CheckCircle size={14} /> {result.import.label} importé en {(result.duree_ms / 1000).toFixed(1)} s
            </p>
            <p className="mt-1 text-gray-700">
              {result.report.n_agents} agents ·{' '}
              {result.report.n_structure} en structure de soins ({result.report.n_structures_couvertes} structures) ·{' '}
              {result.report.n_bureau} en bureau de district ·{' '}
              {result.report.n_bureau_regional} en bureau régional ·{' '}
              {result.report.n_centrale} en administration centrale ·{' '}
              <strong>{result.report.n_non_rattache} non rattachés</strong>
            </p>
            <p className="mt-1 text-xs text-gray-600">
              {Object.entries(result.report.par_source)
                .sort((a, b) => b[1] - a[1])
                .map(([k, v]) => `${sourceLabels[k] ?? k} : ${v}`)
                .join(' · ')}
            </p>
          </div>
        )}
      </div>

      {imports.length > 0 && (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <h3 className="font-medium text-gray-900 px-4 py-3 border-b border-gray-200">Millésimes</h3>
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-gray-600">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Millésime</th>
                <th className="text-right px-4 py-2 font-medium">Agents</th>
                <th className="text-right px-4 py-2 font-medium">En structure</th>
                <th className="text-right px-4 py-2 font-medium">Bureau district</th>
                <th className="text-right px-4 py-2 font-medium">Bureau régional</th>
                <th className="text-right px-4 py-2 font-medium">Centrale</th>
                <th className="text-right px-4 py-2 font-medium">Non rattachés</th>
                <th className="text-left px-4 py-2 font-medium">Importé</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody>
              {imports.map((im) => (
                <tr key={im.id} className={`border-t border-gray-100 ${im.status === 'active' ? 'bg-blue-50/50' : ''}`}>
                  <td className="px-4 py-2">
                    {im.label}
                    {im.status === 'active' && <span className="ml-2 text-xs bg-blue-600 text-white rounded px-1.5 py-0.5">actif</span>}
                    <span className="block text-xs text-gray-500">retraite à {im.age_retraite} ans</span>
                  </td>
                  <td className="px-4 py-2 text-right tabular-nums">{im.n_agents.toLocaleString('fr-FR')}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{im.n_structure.toLocaleString('fr-FR')} <span className="text-gray-400">({pct(im.n_structure, im.n_agents)} %)</span></td>
                  <td className="px-4 py-2 text-right tabular-nums">{im.n_bureau.toLocaleString('fr-FR')}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{(im.n_bureau_regional ?? 0).toLocaleString('fr-FR')}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{im.n_centrale.toLocaleString('fr-FR')}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{im.n_non_rattache.toLocaleString('fr-FR')} <span className="text-gray-400">({pct(im.n_non_rattache, im.n_agents)} %)</span></td>
                  <td className="px-4 py-2 text-gray-500 text-xs">
                    {new Date(im.imported_at).toLocaleString('fr-FR')}
                    {im.imported_by && ` — ${im.imported_by}`}
                  </td>
                  <td className="px-4 py-2 text-right whitespace-nowrap">
                    {im.status !== 'active' && (
                      <button
                        onClick={() => api.activateDrhImport(im.id).then(refresh)}
                        className="text-blue-600 hover:underline text-xs mr-3"
                      >
                        Activer
                      </button>
                    )}
                    <button
                      onClick={() => { if (confirm(`Supprimer ${im.label} et tous ses agrégats ?`)) api.deleteDrhImport(im.id).then(refresh); }}
                      className="text-red-600 hover:text-red-800"
                      title="Supprimer ce millésime"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {active && inconnus.length > 0 && (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
            <div>
              <h3 className="font-medium text-gray-900">Libellés non reconnus</h3>
              <p className="text-xs text-gray-500 mt-0.5">
                À arbitrer dans la table de correspondance, ou à renvoyer à la DRH pour le prochain millésime.
              </p>
            </div>
            <button
              onClick={() => api.downloadDrhCSV(`/api/admin/drh/imports/${active.id}/non-reconnus.csv`, `drh_non_reconnus_${active.annee}.csv`)}
              className="flex items-center gap-1 px-3 py-1.5 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
            >
              <Download size={14} /> CSV
            </button>
          </div>
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-gray-600">
              <tr>
                <th className="text-left px-4 py-2 font-medium">Libellé DRH</th>
                <th className="text-left px-4 py-2 font-medium">Préfecture</th>
                <th className="text-right px-4 py-2 font-medium">Agents</th>
              </tr>
            </thead>
            <tbody>
              {inconnus.map((u, i) => (
                <tr key={i} className="border-t border-gray-100">
                  <td className="px-4 py-2 font-mono text-xs">{u.libelle || <span className="text-gray-400">(vide)</span>}</td>
                  <td className="px-4 py-2 text-gray-600">{u.prefecture}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{u.n_agents}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
