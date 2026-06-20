import { useEffect, useState, useRef } from 'react';
import { useUrlStateList } from '../hooks/useUrlState';
import { api } from '../api/client';
import type { CompareResult, CompareDistrictData, Filters } from '../types';
import ExportCSV from '../components/ExportCSV';
import MethodNote from '../components/MethodNote';

const DISTRICT_COLORS = [
  { bg: 'bg-blue-50', text: 'text-blue-700', border: 'border-blue-200', bar: 'bg-blue-500', header: 'bg-blue-100' },
  { bg: 'bg-orange-50', text: 'text-orange-700', border: 'border-orange-200', bar: 'bg-orange-500', header: 'bg-orange-100' },
  { bg: 'bg-emerald-50', text: 'text-emerald-700', border: 'border-emerald-200', bar: 'bg-emerald-500', header: 'bg-emerald-100' },
  { bg: 'bg-purple-50', text: 'text-purple-700', border: 'border-purple-200', bar: 'bg-purple-500', header: 'bg-purple-100' },
];

const BAR_COLORS = ['#3b82f6', '#f97316', '#10b981', '#8b5cf6'];

function MultiSelect({ options, selected, onChange, max }: {
  options: string[];
  selected: string[];
  onChange: (v: string[]) => void;
  max: number;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  const toggle = (item: string) => {
    if (selected.includes(item)) {
      onChange(selected.filter(s => s !== item));
    } else if (selected.length < max) {
      onChange([...selected, item]);
    }
  };

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="border border-gray-300 rounded px-3 py-1.5 text-sm bg-white min-w-[200px] text-left flex items-center justify-between gap-2"
      >
        <span className="truncate">
          {selected.length === 0 ? 'Selectionner des districts...' : `${selected.length} district${selected.length > 1 ? 's' : ''}`}
        </span>
        <span className="text-gray-400 text-xs">{selected.length}/{max}</span>
      </button>
      {open && (
        <div className="absolute top-full left-0 mt-1 bg-white border border-gray-200 rounded-lg shadow-lg z-50 max-h-60 overflow-y-auto w-64">
          {options.map(opt => {
            const checked = selected.includes(opt);
            const disabled = !checked && selected.length >= max;
            return (
              <label
                key={opt}
                className={`flex items-center gap-2 px-3 py-1.5 text-sm hover:bg-gray-50 cursor-pointer ${disabled ? 'opacity-40 cursor-not-allowed' : ''}`}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={disabled}
                  onChange={() => toggle(opt)}
                  className="rounded"
                />
                {opt}
              </label>
            );
          })}
        </div>
      )}
      {/* Selected tags */}
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1 mt-1">
          {selected.map((s, i) => (
            <span key={s} className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${DISTRICT_COLORS[i]?.bg} ${DISTRICT_COLORS[i]?.text}`}>
              {s}
              <button onClick={() => onChange(selected.filter(x => x !== s))} className="hover:opacity-70">&times;</button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

const pctColor = (v: number) => v >= 80 ? 'text-green-600' : v >= 50 ? 'text-yellow-600' : 'text-red-600';

function buildCsvColumns(label: string, districtNames: string[]) {
  return [
    { key: 'label', header: label },
    ...districtNames.map(n => ({ key: n, header: n })),
    { key: 'national', header: 'National' },
  ];
}

export default function Comparison() {
  const [filters, setFilters] = useState<Filters | null>(null);
  const [selected, setSelected] = useUrlStateList('districts');
  const [result, setResult] = useState<CompareResult | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
  }, []);

  const compare = () => {
    if (selected.length < 2) return;
    setLoading(true);
    api.getCompare(selected)
      .then(setResult)
      .catch(console.error)
      .finally(() => setLoading(false));
  };

  const districts = result?.districts ?? [];
  const nat = result?.national;

  // Build service comparison data
  const serviceRows = result ? (() => {
    const allCodes = new Set<string>();
    districts.forEach(d => d.services.forEach(s => allCodes.add(s.service_code)));
    const maps = districts.map(d => new Map(d.services.map(s => [s.service_code, s])));
    const natMap = new Map(nat!.services.map(s => [s.service_code, s]));
    return Array.from(allCodes).map(code => {
      const label = maps.find(m => m.get(code))?.get(code)?.service_label || code;
      const values = maps.map(m => m.get(code)?.n_oui ?? 0);
      const natVal = natMap.get(code)?.n_oui ?? 0;
      return { label, values, natVal };
    }).sort((a, b) => a.label.localeCompare(b.label));
  })() : [];

  // Build equipment comparison data
  const equipRows = result ? (() => {
    const allRoots = new Set<string>();
    districts.forEach(d => d.equipements.forEach(e => allRoots.add(e.equip_root)));
    const maps = districts.map(d => new Map(d.equipements.map(e => [e.equip_root, e])));
    const natMap = new Map(nat!.equipements.map(e => [e.equip_root, e]));
    return Array.from(allRoots).map(root => {
      const label = maps.find(m => m.get(root))?.get(root)?.label || root;
      const totals = maps.map(m => m.get(root)?.sum_total ?? 0);
      const foncts = maps.map(m => m.get(root)?.sum_fonct ?? 0);
      const natTotal = natMap.get(root)?.sum_total ?? 0;
      const natFonct = natMap.get(root)?.sum_fonct ?? 0;
      return { label, totals, foncts, natTotal, natFonct };
    }).sort((a, b) => a.label.localeCompare(b.label));
  })() : [];

  // Build commodites comparison data
  const commoRows = result ? (() => {
    const labels: Record<string, string> = {
      energie: 'Energie', eau_pts_critiques: 'Eau pts critiques',
      energie_solaire: 'Solaire', energie_reseau: 'Reseau elec.', energie_generateur: 'Generateur',
    };
    const allInds = new Set<string>();
    districts.forEach(d => d.commodites.filter(c => !c.indicator.startsWith('source_eau_')).forEach(c => allInds.add(c.indicator)));
    const maps = districts.map(d => new Map(d.commodites.map(c => [c.indicator, c])));
    const natMap = new Map(nat!.commodites.map(c => [c.indicator, c]));
    return Array.from(allInds).map(ind => ({
      label: labels[ind] || ind,
      values: maps.map(m => m.get(ind)?.pct ?? 0),
      natVal: natMap.get(ind)?.pct ?? 0,
    }));
  })() : [];

  // Build RH comparison data
  const rhRows = result ? (() => {
    const allProfiles = new Set<string>();
    districts.forEach(d => d.rh.forEach(r => allProfiles.add(r.profil_code)));
    const maps = districts.map(d => new Map(d.rh.map(r => [r.profil_code, r])));
    const natMap = new Map(nat!.rh.map(r => [r.profil_code, r]));
    return Array.from(allProfiles).map(code => {
      const label = maps.find(m => m.get(code))?.get(code)?.label || natMap.get(code)?.label || code;
      const values = maps.map(m => m.get(code)?.effectif_total ?? 0);
      const natVal = natMap.get(code)?.effectif_total ?? 0;
      return { label, values, natVal };
    });
  })() : [];

  const districtHeaders = (extra?: string) => (
    <tr className="border-b bg-gray-50">
      <th className="text-left px-3 py-2 font-medium text-gray-500">{extra || ''}</th>
      {districts.map((d, i) => (
        <th key={d.name} className={`text-right px-3 py-2 font-medium ${DISTRICT_COLORS[i].text} ${DISTRICT_COLORS[i].header}`}>
          {d.name}
        </th>
      ))}
      <th className="text-right px-3 py-2 font-medium text-gray-400">National</th>
    </tr>
  );

  const coloredCell = (value: string, idx: number) => (
    <td className={`text-right px-3 py-1.5 font-medium ${DISTRICT_COLORS[idx].bg}`}>{value}</td>
  );

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Comparaison de districts</h2>

      {/* Selectors */}
      <div className="flex flex-wrap gap-3 bg-white p-4 rounded-lg border border-gray-200 items-end">
        <div>
          <label className="block text-xs text-gray-500 mb-1">Districts (2 a 4)</label>
          <MultiSelect
            options={filters?.districts ?? []}
            selected={selected}
            onChange={setSelected}
            max={4}
          />
        </div>
        <button
          onClick={compare}
          disabled={selected.length < 2 || loading}
          className="px-4 py-1.5 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-40"
        >
          {loading ? 'Chargement...' : 'Comparer'}
        </button>
      </div>

      {result && districts.length > 0 && nat && (
        <>
          {/* KPIs table */}
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-gray-700">Indicateurs cles</h3>
              <ExportCSV
                filename="comparaison_kpis"
                columns={buildCsvColumns('Indicateur', districts.map(d => d.name))}
                data={[
                  { label: 'Score qualite', ...Object.fromEntries(districts.map(d => [d.name, d.avg_score.toFixed(1)])), national: nat!.avg_score.toFixed(1) },
                  { label: 'Taux rapportage', ...Object.fromEntries(districts.map(d => [d.name, d.reporting_pct.toFixed(1) + '%'])), national: nat!.reporting_pct.toFixed(1) + '%' },
                  { label: 'Structures', ...Object.fromEntries(districts.map(d => [d.name, d.n_structures])), national: nat!.n_structures },
                  { label: 'Med./structure', ...Object.fromEntries(districts.map(d => [d.name, d.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-'])), national: nat!.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-' },
                  { label: 'Effectif RH', ...Object.fromEntries(districts.map(d => [d.name, d.rh_summary?.total_effectif ?? 0])), national: nat!.rh_summary?.total_effectif ?? 0 },
                ] as Record<string, unknown>[]}
              />
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>{districtHeaders('Indicateur')}</thead>
                <tbody>
                  <tr className="border-b border-gray-100">
                    <td className="px-3 py-1.5 text-gray-700">Score qualite</td>
                    {districts.map((d, i) => (
                      <td key={d.name} className={`text-right px-3 py-1.5 font-bold ${pctColor(d.avg_score)} ${DISTRICT_COLORS[i].bg}`}>
                        {d.avg_score.toFixed(1)}
                      </td>
                    ))}
                    <td className="text-right px-3 py-1.5 text-gray-400">{nat.avg_score.toFixed(1)}</td>
                  </tr>
                  <tr className="border-b border-gray-100">
                    <td className="px-3 py-1.5 text-gray-700">Taux rapportage</td>
                    {districts.map((d, i) => (
                      <td key={d.name} className={`text-right px-3 py-1.5 font-bold ${pctColor(d.reporting_pct)} ${DISTRICT_COLORS[i].bg}`}>
                        {d.reporting_pct.toFixed(1)}%
                      </td>
                    ))}
                    <td className="text-right px-3 py-1.5 text-gray-400">{nat.reporting_pct.toFixed(1)}%</td>
                  </tr>
                  <tr className="border-b border-gray-100">
                    <td className="px-3 py-1.5 text-gray-700">Structures</td>
                    {districts.map((d, i) => coloredCell(String(d.n_structures), i))}
                    <td className="text-right px-3 py-1.5 text-gray-400">{nat.n_structures}</td>
                  </tr>
                  <tr className="border-b border-gray-100">
                    <td className="px-3 py-1.5 text-gray-700">Med./structure</td>
                    {districts.map((d, i) => coloredCell(d.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-', i))}
                    <td className="text-right px-3 py-1.5 text-gray-400">{nat.rh_summary?.ratio_med_per_structure?.toFixed(2) ?? '-'}</td>
                  </tr>
                  <tr className="border-b border-gray-100">
                    <td className="px-3 py-1.5 text-gray-700">Effectif RH total</td>
                    {districts.map((d, i) => coloredCell(String(d.rh_summary?.total_effectif ?? 0), i))}
                    <td className="text-right px-3 py-1.5 text-gray-400">{nat.rh_summary?.total_effectif ?? 0}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          {/* Services */}
          {serviceRows.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold text-gray-700">Services — nombre de structures</h3>
                <ExportCSV
                  filename="comparaison_services"
                  columns={buildCsvColumns('Service', districts.map(d => d.name))}
                  data={serviceRows.map(r => ({ label: r.label, ...Object.fromEntries(districts.map((d, i) => [d.name, r.values[i]])), national: r.natVal })) as Record<string, unknown>[]}
                />
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>{districtHeaders('Service')}</thead>
                  <tbody>
                    {serviceRows.map((row) => (
                      <tr key={row.label} className="border-b border-gray-100">
                        <td className="px-3 py-1.5 text-gray-700">{row.label}</td>
                        {row.values.map((v, i) => coloredCell(String(v), i))}
                        <td className="text-right px-3 py-1.5 text-gray-400">{row.natVal}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Equipements */}
          {equipRows.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold text-gray-700">Equipements — total / fonctionnel</h3>
                <ExportCSV
                  filename="comparaison_equipements"
                  columns={[
                    { key: 'label', header: 'Equipement' },
                    ...districts.flatMap(d => [{ key: `${d.name}_total`, header: `${d.name} Total` }, { key: `${d.name}_fonct`, header: `${d.name} Fonct` }]),
                    { key: 'national_total', header: 'National Total' },
                    { key: 'national_fonct', header: 'National Fonct' },
                  ]}
                  data={equipRows.map(r => ({
                    label: r.label,
                    ...Object.fromEntries(districts.flatMap((d, i) => [[`${d.name}_total`, r.totals[i]], [`${d.name}_fonct`, r.foncts[i]]])),
                    national_total: r.natTotal, national_fonct: r.natFonct,
                  })) as Record<string, unknown>[]}
                />
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b bg-gray-50">
                      <th className="text-left px-3 py-2 font-medium text-gray-500">Equipement</th>
                      {districts.map((d, i) => (
                        <th key={d.name} className={`text-center px-2 py-2 font-medium ${DISTRICT_COLORS[i].text} ${DISTRICT_COLORS[i].header}`} colSpan={2}>
                          {d.name}
                        </th>
                      ))}
                      <th className="text-center px-2 py-2 font-medium text-gray-400" colSpan={2}>National</th>
                    </tr>
                    <tr className="border-b bg-gray-50">
                      <th></th>
                      {districts.map((d) => (
                        <>{/* eslint-disable-next-line react/jsx-key */}
                          <th key={`${d.name}-t`} className="text-right px-1 py-1 text-[10px] text-gray-400">Tot</th>
                          <th key={`${d.name}-f`} className="text-right px-1 py-1 text-[10px] text-gray-400">Fonc</th>
                        </>
                      ))}
                      <th className="text-right px-1 py-1 text-[10px] text-gray-400">Tot</th>
                      <th className="text-right px-1 py-1 text-[10px] text-gray-400">Fonc</th>
                    </tr>
                  </thead>
                  <tbody>
                    {equipRows.map((row) => (
                      <tr key={row.label} className="border-b border-gray-100">
                        <td className="px-3 py-1.5 text-gray-700">{row.label}</td>
                        {row.totals.map((t, i) => (
                          <>{/* eslint-disable-next-line react/jsx-key */}
                            <td key={`${row.label}-${i}-t`} className={`text-right px-1 py-1.5 font-medium ${DISTRICT_COLORS[i].bg}`}>{t}</td>
                            <td key={`${row.label}-${i}-f`} className={`text-right px-1 py-1.5 text-green-600 ${DISTRICT_COLORS[i].bg}`}>{row.foncts[i]}</td>
                          </>
                        ))}
                        <td className="text-right px-1 py-1.5 text-gray-400">{row.natTotal}</td>
                        <td className="text-right px-1 py-1.5 text-gray-400">{row.natFonct}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Commodites */}
          {commoRows.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold text-gray-700">Commodites — % de structures</h3>
                <ExportCSV
                  filename="comparaison_commodites"
                  columns={buildCsvColumns('Indicateur', districts.map(d => d.name))}
                  data={commoRows.map(r => ({ label: r.label, ...Object.fromEntries(districts.map((d, i) => [d.name, r.values[i].toFixed(1) + '%'])), national: r.natVal.toFixed(1) + '%' })) as Record<string, unknown>[]}
                />
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>{districtHeaders('Indicateur')}</thead>
                  <tbody>
                    {commoRows.map((row) => (
                      <tr key={row.label} className="border-b border-gray-100">
                        <td className="px-3 py-1.5 text-gray-700">{row.label}</td>
                        {row.values.map((v, i) => (
                          <td key={i} className={`text-right px-3 py-1.5 font-medium ${DISTRICT_COLORS[i].bg}`}>{v.toFixed(1)}%</td>
                        ))}
                        <td className="text-right px-3 py-1.5 text-gray-400">{row.natVal.toFixed(1)}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* RH */}
          {rhRows.length > 0 && (
            <div className="bg-white rounded-lg border border-gray-200 p-4">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold text-gray-700">Ressources humaines — effectifs par profil</h3>
                <ExportCSV
                  filename="comparaison_rh"
                  columns={buildCsvColumns('Profil', districts.map(d => d.name))}
                  data={rhRows.map(r => ({ label: r.label, ...Object.fromEntries(districts.map((d, i) => [d.name, r.values[i]])), national: r.natVal })) as Record<string, unknown>[]}
                />
              </div>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>{districtHeaders('Profil')}</thead>
                  <tbody>
                    {rhRows.map((row) => (
                      <tr key={row.label} className="border-b border-gray-100">
                        <td className="px-3 py-1.5 text-gray-700">{row.label}</td>
                        {row.values.map((v, i) => coloredCell(String(v), i))}
                        <td className="text-right px-3 py-1.5 text-gray-400">{row.natVal}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}

      <MethodNote title="Methodologie - Comparaison de districts">
        <p>Cet outil permet de comparer jusqu'a 4 districts cote a cote sur l'ensemble des indicateurs ISS.</p>
        <p><strong>Score qualite</strong> : moyenne des scores de toutes les structures du district (0-100, penalites : -15/erreur, -5/avertissement, -1/info).</p>
        <p><strong>Taux de rapportage</strong> : structures ayant soumis / structures attendues x 100.</p>
        <p><strong>Med./structure</strong> : nombre total de medecins (generalistes + specialistes) divise par le nombre de structures du district.</p>
        <p><strong>Services</strong> : nombre de structures disposant de chaque service (fonctionnel).</p>
        <p><strong>Equipements</strong> : nombres bruts total et fonctionnel par type d'equipement.</p>
        <p><strong>Commodites</strong> : pourcentage de structures disposant de chaque commodite (energie, eau).</p>
        <p><strong>Ressources humaines</strong> : effectifs par profil et statut d'emploi.</p>
        <p>La <strong>moyenne nationale</strong> est affichee comme reference dans la derniere colonne.</p>
      </MethodNote>
    </div>
  );
}
