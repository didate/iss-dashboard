import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { CheckCircle, Copy, Download, Plus, RefreshCw, Save, Trash2, Upload, X } from 'lucide-react';
import { api } from '../../api/client';
import type { NormeLineError, NormeRule, NormeSet, NormeTargets, NormesMeta } from '../../types';
import { typologieLabel } from '../../utils/typologie';

const TYPE_CODES = ['*', 'PS', 'CS', 'CSA', 'CMC', 'HP', 'HR', 'HN', 'CABINET', 'CLINIQUE', 'AUTRE_PRIVE'];
const KIND_LABELS: Record<string, string> = { service: 'Service', rh: 'Ressources humaines', equipement: 'Équipement', infra: 'Infrastructure' };
const STATUS_LABELS: Record<string, string> = { draft: 'Brouillon', active: 'Actif', archived: 'Archivé' };
const STATUS_CLS: Record<string, string> = { draft: 'bg-amber-100 text-amber-800', active: 'bg-green-100 text-green-800', archived: 'bg-gray-100 text-gray-600' };

// Onglet Admin → Normes : versions du référentiel, éditeur en grille, import/export CSV.
export default function NormesEditor() {
  const [sets, setSets] = useState<NormeSet[]>([]);
  const [meta, setMeta] = useState<NormesMeta | null>(null);
  const [targets, setTargets] = useState<NormeTargets | null>(null);
  const [selected, setSelected] = useState<NormeSet | null>(null);
  const [rules, setRules] = useState<NormeRule[]>([]);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [lineErrors, setLineErrors] = useState<NormeLineError[]>([]);
  const [info, setInfo] = useState('');
  const [filterType, setFilterType] = useState('');
  const [newName, setNewName] = useState('');
  const fileRef = useRef<HTMLInputElement>(null);

  const refresh = useCallback(async () => {
    const [s, m] = await Promise.all([api.getNormeSets(), api.getNormesMeta()]);
    setSets(s);
    setMeta(m);
    return s;
  }, []);

  useEffect(() => {
    refresh().catch((e) => setError(e.message));
    api.getNormeTargets().then(setTargets).catch(() => setTargets(null));
  }, [refresh]);

  const loadRules = useCallback(async (set: NormeSet) => {
    setSelected(set);
    setDirty(false);
    setLineErrors([]);
    setRules(await api.getNormeRules(set.id));
  }, []);

  const run = async (label: string, fn: () => Promise<unknown>) => {
    setBusy(label);
    setError('');
    setInfo('');
    setLineErrors([]);
    try {
      await fn();
    } catch (e) {
      const err = e as Error & { errors?: NormeLineError[] };
      setError(err.message);
      if (err.errors) setLineErrors(err.errors);
    } finally {
      setBusy('');
    }
  };

  const create = () =>
    run('create', async () => {
      const s = await api.createNormeSet({ name: newName || 'Nouveau référentiel', notes: '' });
      setNewName('');
      await refresh();
      await loadRules(s);
    });

  const duplicate = (s: NormeSet) =>
    run('dup', async () => {
      const d = await api.duplicateNormeSet(s.id);
      await refresh();
      await loadRules(d);
      setInfo(`Version ${d.version} créée à partir de la v${s.version}.`);
    });

  const activate = (s: NormeSet) => {
    if (!window.confirm(`Activer le référentiel « ${s.name} » v${s.version} (${s.n_rules} règles) ? La conformité sera recalculée.`)) return;
    run('activate', async () => {
      await api.activateNormeSet(s.id);
      const list = await refresh();
      const cur = list.find((x) => x.id === s.id);
      if (cur) setSelected(cur);
      setInfo('Référentiel activé et conformité recalculée.');
    });
  };

  const remove = (s: NormeSet) => {
    if (!window.confirm(`Supprimer le brouillon « ${s.name} » v${s.version} ?`)) return;
    run('delete', async () => {
      await api.deleteNormeSet(s.id);
      if (selected?.id === s.id) {
        setSelected(null);
        setRules([]);
      }
      await refresh();
    });
  };

  const save = () =>
    run('save', async () => {
      if (!selected) return;
      const saved = await api.putNormeRules(selected.id, rules);
      setRules(saved);
      setDirty(false);
      await refresh();
      setInfo(`${saved.length} règles enregistrées${selected.status === 'active' ? ' — conformité recalculée' : ''}.`);
    });

  const importCSV = (file: File) =>
    run('import', async () => {
      if (!selected) return;
      const mode = rules.length > 0 && !window.confirm('Remplacer les règles existantes par le fichier ? (Annuler = ajouter au lieu de remplacer)') ? 'append' : 'replace';
      const res = await api.importNormeRules(selected.id, file, mode);
      await loadRules(selected);
      await refresh();
      setInfo(`${res.imported} règle(s) importée(s), ${res.total} au total.`);
    });

  const recompute = () =>
    run('recompute', async () => {
      await api.recomputeConformite();
      await refresh();
      setInfo('Conformité recalculée.');
    });

  const updateRule = (i: number, patch: Partial<NormeRule>) => {
    setRules((rs) => rs.map((r, j) => (j === i ? { ...r, ...patch } : r)));
    setDirty(true);
  };
  const addRule = () => {
    setRules((rs) => [{ type_code: filterType || 'CS', kind: 'service', target: '', label: '', min_value: 1, level: 'essentiel' }, ...rs]);
    setDirty(true);
  };
  const removeRule = (i: number) => {
    setRules((rs) => rs.filter((_, j) => j !== i));
    setDirty(true);
  };

  const targetsByKind = useMemo(() => {
    const m: Record<string, { code: string; label: string }[]> = {};
    for (const t of targets?.targets ?? []) (m[t.kind] ||= []).push(t);
    return m;
  }, [targets]);

  const visible = rules.map((r, i) => ({ r, i })).filter(({ r }) => !filterType || r.type_code === filterType);
  const editable = selected?.status !== 'archived';
  const inputCls = 'border border-gray-300 rounded px-1.5 py-1 text-xs bg-white w-full';

  return (
    <div className="space-y-4">
      {/* Bandeau référentiel actif */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 flex flex-wrap items-center gap-3">
        <div className="flex-1 min-w-0">
          <h3 className="font-medium text-gray-900">Référentiel de normes</h3>
          {meta?.active ? (
            <p className="text-sm text-gray-600">
              Actif : <strong>{meta.active.name}</strong> v{meta.active.version} — {meta.active.n_rules} règles, activé le{' '}
              {meta.active.activated_at ? new Date(meta.active.activated_at).toLocaleDateString('fr-FR') : '—'}
              {meta.last_run && (
                <span className="text-gray-500">
                  {' '}· dernier calcul {new Date(meta.last_run.computed_at).toLocaleString('fr-FR')} : {meta.last_run.n_evaluees} structures évaluées,{' '}
                  {meta.last_run.n_conformes} conformes
                </span>
              )}
            </p>
          ) : (
            <p className="text-sm text-amber-700">Aucun référentiel actif : la conformité n'est pas calculée.</p>
          )}
        </div>
        <button onClick={recompute} disabled={!!busy || !meta?.active} className="flex items-center gap-1 text-xs px-3 py-1.5 border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-40">
          <RefreshCw size={14} className={busy === 'recompute' ? 'animate-spin' : ''} /> Recalculer
        </button>
      </div>

      {(error || info) && (
        <div className={`text-sm rounded px-3 py-2 ${error ? 'bg-red-50 text-red-700 border border-red-100' : 'bg-green-50 text-green-700 border border-green-100'}`}>
          {error || info}
          {lineErrors.length > 0 && (
            <ul className="mt-1 text-xs list-disc pl-5 max-h-40 overflow-auto">
              {lineErrors.map((e, i) => (
                <li key={i}>Ligne {e.line} : {e.message}</li>
              ))}
            </ul>
          )}
        </div>
      )}

      <div className="grid lg:grid-cols-3 gap-4">
        {/* Versions */}
        <div className="bg-white rounded-lg border border-gray-200">
          <div className="p-3 border-b flex items-center gap-2">
            <input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="Nom du nouveau référentiel" className={inputCls} />
            <button onClick={create} disabled={!!busy} className="flex items-center gap-1 text-xs px-2 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 shrink-0">
              <Plus size={14} /> Créer
            </button>
          </div>
          <ul className="divide-y divide-gray-100 max-h-[60vh] overflow-auto">
            {sets.map((s) => (
              <li key={s.id} className={`p-3 cursor-pointer hover:bg-gray-50 ${selected?.id === s.id ? 'bg-blue-50' : ''}`} onClick={() => loadRules(s).catch((e) => setError(e.message))}>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-medium text-gray-900 truncate">v{s.version} — {s.name}</span>
                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${STATUS_CLS[s.status]}`}>{STATUS_LABELS[s.status]}</span>
                </div>
                <div className="text-xs text-gray-500 mt-0.5">
                  {s.n_rules} règles · créé le {new Date(s.created_at).toLocaleDateString('fr-FR')} {s.created_by && `par ${s.created_by}`}
                </div>
                <div className="flex gap-2 mt-1.5" onClick={(e) => e.stopPropagation()}>
                  {s.status !== 'active' && (
                    <button onClick={() => activate(s)} disabled={!!busy || s.n_rules === 0} className="flex items-center gap-1 text-[11px] text-green-700 hover:underline disabled:opacity-40 disabled:no-underline">
                      <CheckCircle size={12} /> Activer
                    </button>
                  )}
                  <button onClick={() => duplicate(s)} disabled={!!busy} className="flex items-center gap-1 text-[11px] text-gray-600 hover:underline">
                    <Copy size={12} /> Dupliquer
                  </button>
                  <button onClick={() => api.exportNormeRulesCSV(s.id, s.version).catch((e) => setError(e.message))} className="flex items-center gap-1 text-[11px] text-gray-600 hover:underline">
                    <Download size={12} /> CSV
                  </button>
                  {s.status === 'draft' && (
                    <button onClick={() => remove(s)} disabled={!!busy} className="flex items-center gap-1 text-[11px] text-red-600 hover:underline ml-auto">
                      <Trash2 size={12} /> Supprimer
                    </button>
                  )}
                </div>
              </li>
            ))}
            {sets.length === 0 && <li className="p-4 text-sm text-gray-400">Aucun référentiel. Créez-en un, puis importez un CSV (voir docs/normes-exemple.csv).</li>}
          </ul>
        </div>

        {/* Éditeur */}
        <div className="lg:col-span-2 bg-white rounded-lg border border-gray-200">
          {!selected ? (
            <div className="p-8 text-sm text-gray-400 text-center">Sélectionnez une version pour voir ou modifier ses règles.</div>
          ) : (
            <>
              <div className="p-3 border-b flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium text-gray-900">
                  v{selected.version} — {selected.name} <span className={`ml-1 text-[10px] px-1.5 py-0.5 rounded ${STATUS_CLS[selected.status]}`}>{STATUS_LABELS[selected.status]}</span>
                </span>
                <select value={filterType} onChange={(e) => setFilterType(e.target.value)} className="border border-gray-300 rounded px-2 py-1 text-xs">
                  <option value="">Tous les types</option>
                  {TYPE_CODES.map((t) => <option key={t} value={t}>{t === '*' ? '* (toutes)' : `${t} — ${typologieLabel(t)}`}</option>)}
                </select>
                <span className="text-xs text-gray-500">{visible.length} / {rules.length} règles</span>
                <div className="ml-auto flex items-center gap-2">
                  {editable && (
                    <>
                      <input ref={fileRef} type="file" accept=".csv,text/csv" className="hidden" onChange={(e) => { const f = e.target.files?.[0]; if (f) importCSV(f); e.target.value = ''; }} />
                      <button onClick={() => fileRef.current?.click()} disabled={!!busy} className="flex items-center gap-1 text-xs px-2 py-1.5 border border-gray-300 rounded hover:bg-gray-50">
                        <Upload size={14} /> Importer CSV
                      </button>
                      <button onClick={addRule} className="flex items-center gap-1 text-xs px-2 py-1.5 border border-gray-300 rounded hover:bg-gray-50">
                        <Plus size={14} /> Règle
                      </button>
                      <button onClick={save} disabled={!dirty || !!busy} className="flex items-center gap-1 text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-40">
                        <Save size={14} /> {busy === 'save' ? 'Enregistrement…' : 'Enregistrer'}
                      </button>
                    </>
                  )}
                </div>
              </div>
              {selected.status === 'active' && dirty && (
                <div className="px-3 py-1.5 text-xs bg-amber-50 text-amber-800 border-b border-amber-100">
                  Ce référentiel est actif : enregistrer recalcule immédiatement la conformité. Pour préparer une nouvelle version sans impact, dupliquez-le.
                </div>
              )}
              <div className="overflow-auto max-h-[60vh]">
                <table className="w-full text-xs">
                  <thead className="sticky top-0 bg-gray-50">
                    <tr className="border-b text-left text-gray-500">
                      <th className="px-2 py-2 font-medium w-24">Type</th>
                      <th className="px-2 py-2 font-medium w-32">Famille</th>
                      <th className="px-2 py-2 font-medium">Cible</th>
                      <th className="px-2 py-2 font-medium w-16">Min.</th>
                      <th className="px-2 py-2 font-medium w-28">Niveau</th>
                      <th className="px-2 py-2 w-8" />
                    </tr>
                  </thead>
                  <tbody>
                    {visible.map(({ r, i }) => (
                      <tr key={r.id ?? `n${i}`} className="border-b border-gray-100">
                        <td className="px-2 py-1">
                          <select value={r.type_code} disabled={!editable} onChange={(e) => updateRule(i, { type_code: e.target.value })} className={inputCls}>
                            {TYPE_CODES.map((t) => <option key={t} value={t}>{t}</option>)}
                          </select>
                        </td>
                        <td className="px-2 py-1">
                          <select value={r.kind} disabled={!editable} onChange={(e) => updateRule(i, { kind: e.target.value, target: '', label: '', min_value: 1 })} className={inputCls}>
                            {(targets?.kinds ?? Object.keys(KIND_LABELS)).map((k) => <option key={k} value={k}>{KIND_LABELS[k] ?? k}</option>)}
                          </select>
                        </td>
                        <td className="px-2 py-1">
                          {targetsByKind[r.kind] ? (
                            <select
                              value={r.target}
                              disabled={!editable}
                              onChange={(e) => {
                                const t = targetsByKind[r.kind].find((x) => x.code === e.target.value);
                                updateRule(i, { target: e.target.value, label: t?.label ?? r.label });
                              }}
                              className={inputCls}
                              title={r.target}
                            >
                              <option value="">— choisir —</option>
                              {r.kind === 'rh' && <option value="ISS_RH_MED_">Médecin (tout profil) — ISS_RH_MED_</option>}
                              {targetsByKind[r.kind].map((t) => <option key={t.code} value={t.code}>{t.label} — {t.code}</option>)}
                            </select>
                          ) : (
                            <input value={r.target} disabled={!editable} onChange={(e) => updateRule(i, { target: e.target.value })} className={inputCls} placeholder="code cible" />
                          )}
                        </td>
                        <td className="px-2 py-1">
                          {r.kind === 'service' ? (
                            <span className="text-gray-400">oui</span>
                          ) : (
                            <input type="number" min={1} step={1} value={r.min_value} disabled={!editable} onChange={(e) => updateRule(i, { min_value: Number(e.target.value) })} className={inputCls} />
                          )}
                        </td>
                        <td className="px-2 py-1">
                          <select value={r.level} disabled={!editable} onChange={(e) => updateRule(i, { level: e.target.value })} className={inputCls}>
                            <option value="essentiel">Essentiel</option>
                            <option value="recommande">Recommandé</option>
                          </select>
                        </td>
                        <td className="px-1 py-1 text-center">
                          {editable && (
                            <button onClick={() => removeRule(i)} className="text-gray-400 hover:text-red-600" title="Supprimer">
                              <X size={14} />
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                    {visible.length === 0 && (
                      <tr><td colSpan={6} className="px-3 py-6 text-center text-gray-400">Aucune règle{filterType ? ` pour ${filterType}` : ''}. Importez un CSV ou ajoutez une règle.</td></tr>
                    )}
                  </tbody>
                </table>
              </div>
              <div className="px-3 py-2 text-[11px] text-gray-400 border-t">
                Format CSV : <code>type_code;kind;target;label;min_value;level</code> — lignes <code>#</code> ignorées. Une règle <code>*</code> s'applique à tous les types ; une règle spécifique au type prime. Équipements : le minimum porte sur les unités <em>fonctionnelles</em>.
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
