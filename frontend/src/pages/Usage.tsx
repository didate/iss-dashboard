import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileDown } from 'lucide-react';
import { useUrlState } from '../hooks/useUrlState';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell, PieChart, Pie, Legend } from 'recharts';
import { api } from '../api/client';
import type {
  UsageRecensement,
  UsageService,
  UsageEquipement,
  UsageRH,
  UsageCommodite,
  PlateauItem,
  ServiceMatrixRow,
  RHSummaryResult,
  ReportingRate,
  ClosedOUItem,
  Filters,
} from '../types';
import DataTable from '../components/DataTable';
import ExportCSV from '../components/ExportCSV';
import MethodNote from '../components/MethodNote';

const tabs = [
  { key: 'rapportage', label: 'Rapportage' },
  { key: 'recensement', label: 'Recensement' },
  { key: 'plateau', label: 'Plateau technique' },
  { key: 'services', label: 'Services' },
  { key: 'matrice', label: 'Matrice' },
  { key: 'equipements', label: 'Équipements' },
  { key: 'rh', label: 'Ressources humaines' },
  { key: 'commodites', label: 'Commodités' },
  { key: 'fermees', label: 'Structures fermées' },
];

export default function Usage() {
  const navigate = useNavigate();
  const [tab, setTab] = useUrlState('tab', 'rapportage');
  const [district, setDistrict] = useUrlState('district');
  const [filters, setFilters] = useState<Filters | null>(null);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
  }, []);

  const showDistrictFilter = !['recensement', 'matrice', 'rapportage'].includes(tab);

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Utilisation & Analyse</h2>

      <div className="flex items-center gap-4 flex-wrap">
        <div className="flex flex-wrap bg-white rounded-lg border border-gray-200 p-0.5">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-3 py-1.5 text-sm rounded ${
                tab === t.key ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100'
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {showDistrictFilter && (
          <select
            className="border border-gray-300 rounded px-2 py-1.5 text-sm"
            value={district}
            onChange={(e) => setDistrict(e.target.value)}
          >
            <option value="">Tous districts</option>
            {filters?.districts.map((d) => (
              <option key={d} value={d}>{d}</option>
            ))}
          </select>
        )}

        {district && (
          <button
            onClick={() => {
              const uid = filters?.district_uids ? Object.entries(filters.district_uids).find(([, name]) => name === district)?.[0] : null;
              navigate(`/rapport/${uid || encodeURIComponent(district)}`);
            }}
            className="flex items-center gap-1 px-3 py-1.5 text-sm bg-red-600 text-white rounded hover:bg-red-700"
          >
            <FileDown size={16} />
            Rapport PDF
          </button>
        )}
      </div>

      <div className="bg-white rounded-lg border border-gray-200 p-4">
        {tab === 'rapportage' && <RapportageTab filters={filters} />}
        {tab === 'recensement' && <RecensementTab />}
        {tab === 'plateau' && <PlateauTab district={district} />}
        {tab === 'services' && <ServicesTab district={district} />}
        {tab === 'matrice' && <MatriceTab />}
        {tab === 'equipements' && <EquipementsTab district={district} />}
        {tab === 'rh' && <RHTab district={district} />}
        {tab === 'commodites' && <CommoditesTab district={district} />}
        {tab === 'fermees' && <ClosedOUsTab district={district} />}
      </div>
    </div>
  );
}

// ==================== RAPPORTAGE ====================

function RapportageTab({ filters }: { filters: Filters | null }) {
  const [by, setBy] = useUrlState('by', 'district');
  const [regionFilter, setRegionFilter] = useUrlState('region');
  const [data, setData] = useState<ReportingRate[]>([]);
  const [globalRate, setGlobalRate] = useState<ReportingRate | null>(null);

  useEffect(() => {
    api.getReportingRate(by).then((d) => setData(d ?? [])).catch(console.error);
    api.getReportingRate('global').then((r) => {
      const g = (r ?? []).find((x) => x.key === 'all');
      if (g) setGlobalRate(g);
    }).catch(console.error);
  }, [by]);

  // Filter districts by region
  const filteredData = by === 'district' && regionFilter && filters?.district_regions
    ? data.filter(d => filters.district_regions[d.key] === regionFilter)
    : data;

  const columns = [
    { key: 'label', header: by.charAt(0).toUpperCase() + by.slice(1) },
    { key: 'n_expected', header: 'Attendu' },
    { key: 'n_reported', header: 'Soumis' },
    { key: 'pct', header: '% Rapportage', render: (row: Record<string, unknown>) => `${(row.pct as number).toFixed(1)}%` },
  ];

  return (
    <div className="space-y-4">
      {/* Global KPI */}
      {globalRate && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-4">
          <div className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Rapports attendus</p>
            <p className="text-xl sm:text-3xl font-bold text-gray-900 mt-1 sm:mt-2">{globalRate.n_expected.toLocaleString('fr-FR')}</p>
            <p className="text-xs text-gray-400">structures assignées au programme</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Rapports soumis</p>
            <p className="text-xl sm:text-3xl font-bold text-blue-600 mt-1 sm:mt-2">{globalRate.n_reported.toLocaleString('fr-FR')}</p>
            <p className="text-xs text-gray-400">structures ayant soumis</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Taux de rapportage</p>
            <p className={`text-xl sm:text-3xl font-bold mt-1 sm:mt-2 ${globalRate.pct >= 80 ? 'text-green-600' : globalRate.pct >= 50 ? 'text-yellow-600' : 'text-red-600'}`}>
              {globalRate.pct.toFixed(1)}%
            </p>
            <div className="mt-2 w-full bg-gray-200 rounded-full h-2">
              <div className={`h-2 rounded-full ${globalRate.pct >= 80 ? 'bg-green-500' : globalRate.pct >= 50 ? 'bg-yellow-500' : 'bg-red-500'}`}
                style={{ width: `${Math.min(globalRate.pct, 100)}%` }} />
            </div>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between flex-wrap gap-2">
        <div className="flex gap-2 items-center">
          {['district', 'region'].map((v) => (
            <button key={v} onClick={() => { setBy(v); setRegionFilter(''); }}
              className={`px-2 py-1 text-xs rounded ${by === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}>
              {v.charAt(0).toUpperCase() + v.slice(1)}
            </button>
          ))}
          {by === 'district' && filters?.regions && (
            <select
              className="border border-gray-300 rounded px-2 py-1 text-xs"
              value={regionFilter}
              onChange={(e) => setRegionFilter(e.target.value)}
            >
              <option value="">Toutes les régions</option>
              {filters.regions.map((r) => (
                <option key={r} value={r}>{r}</option>
              ))}
            </select>
          )}
        </div>
        <ExportCSV data={filteredData as unknown as Record<string, unknown>[]} columns={columns} filename={`rapportage_${by}`} />
      </div>

      {/* Bar chart */}
      {filteredData.length > 0 && (
        <ResponsiveContainer width="100%" height={Math.max(300, filteredData.length * 30)}>
          <BarChart data={[...filteredData].sort((a, b) => a.pct - b.pct)} layout="vertical" margin={{ left: 100 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis type="number" domain={[0, 100]} unit="%" />
            <YAxis type="category" dataKey="label" tick={{ fontSize: 10 }} width={90} />
            <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
            <Bar dataKey="pct" name="% rapportage">
              {[...filteredData].sort((a, b) => a.pct - b.pct).map((d, i) => (
                <Cell key={i} fill={d.pct >= 80 ? '#22c55e' : d.pct >= 50 ? '#eab308' : '#ef4444'} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      )}

      <DataTable columns={columns} data={filteredData as unknown as Record<string, unknown>[]} />

      <MethodNote title="Méthodologie - Taux de rapportage">
        <p><strong>Attendu</strong> = nombre d'unités organisationnelles assignées au programme ISS dans DHIS2.</p>
        <p><strong>Soumis</strong> = nombre d'unités organisationnelles distinctes ayant au moins un événement soumis.</p>
        <p><strong>Taux de rapportage</strong> = (soumis / attendu) x 100.</p>
      </MethodNote>
    </div>
  );
}

// ==================== RECENSEMENT ====================

function RecensementTab() {
  const [by, setBy] = useUrlState('by', 'district');
  const [data, setData] = useState<UsageRecensement[]>([]);

  useEffect(() => {
    api.getUsageRecensement(by).then((d) => setData(d ?? [])).catch(console.error);
  }, [by]);

  const columns = [
    { key: 'label', header: by.charAt(0).toUpperCase() + by.slice(1) },
    { key: 'n_structures', header: 'Total' },
    { key: 'n_operationnel', header: 'Opérationnel' },
    { key: 'n_non_operationnel', header: 'Non opérationnel' },
    { key: 'n_ferme_temp', header: 'Fermé temp.' },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex gap-2">
          {['district', 'region', 'statut_juridique'].map((v) => (
            <button key={v} onClick={() => setBy(v)}
              className={`px-2 py-1 text-xs rounded ${by === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}>
              {v === 'statut_juridique' ? 'Statut juridique' : v.charAt(0).toUpperCase() + v.slice(1)}
            </button>
          ))}
        </div>
        <ExportCSV data={data as unknown as Record<string, unknown>[]} columns={columns} filename={`recensement_${by}`} />
      </div>
      <DataTable columns={columns} data={data as unknown as Record<string, unknown>[]} />
      <MethodNote title="Méthodologie - Recensement">
        <p>Nombre de structures sanitaires par dimension géographique ou administrative, extraites du programme DHIS2 ISS.</p>
        <p><strong>Opérationnel</strong> : statut opérationnel = "operationnel". <strong>Non opérationnel</strong> : statut = "non_operationnel". <strong>Fermé temporairement</strong> : statut = "ferme_temporairement".</p>
        <p>Le rattachement district/région est déduit de la hiérarchie des org units DHIS2.</p>
      </MethodNote>
    </div>
  );
}

// ==================== PLATEAU TECHNIQUE ====================

function PlateauTab({ district }: { district: string }) {
  const [data, setData] = useState<PlateauItem[]>([]);

  useEffect(() => {
    api.getPlateauTechnique(district).then((d) => setData(d ?? [])).catch(console.error);
  }, [district]);

  const columns = [
    { key: 'service_label', header: 'Service' },
    { key: 'n_oui', header: 'Fonctionnel' },
    { key: 'n_total', header: 'Total' },
    { key: 'pct', header: '% Disponibilité', render: (row: Record<string, unknown>) => `${(row.pct as number).toFixed(1)}%` },
  ];

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <ExportCSV data={data as unknown as Record<string, unknown>[]} columns={columns} filename="plateau_technique" />
      </div>

      <ResponsiveContainer width="100%" height={Math.max(300, data.length * 35)}>
        <BarChart data={data} layout="vertical" margin={{ left: 120 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis type="number" domain={[0, 100]} unit="%" />
          <YAxis type="category" dataKey="service_label" tick={{ fontSize: 10 }} width={110} />
          <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
          <Bar dataKey="pct" name="% disponibilité">
            {data.map((d, i) => (
              <Cell key={i} fill={d.pct >= 75 ? '#22c55e' : d.pct >= 50 ? '#eab308' : d.pct >= 25 ? '#f97316' : '#ef4444'} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>

      <DataTable columns={columns} data={data as unknown as Record<string, unknown>[]} />

      <MethodNote title="Méthodologie - Plateau technique">
        <p>Le plateau technique mesure la disponibilité des services clés dans les structures sanitaires.</p>
        <p>Un service est considéré <strong>disponible</strong> quand sa valeur dans le formulaire ISS est "oui" (fonctionnel).</p>
        <p>Les services affichés sont : laboratoire, maternité, chirurgie, imagerie, pharmacie, urgences, pédiatrie, médecine générale, hémodialyse, néonatologie, dentaire, anesthésie-réanimation.</p>
        <p><strong>% Disponibilité</strong> = nombre de structures où le service est fonctionnel / nombre total de structures ayant renseigné ce champ x 100.</p>
      </MethodNote>
    </div>
  );
}

// ==================== SERVICES ====================

function ServicesTab({ district }: { district: string }) {
  const [data, setData] = useState<UsageService[]>([]);

  useEffect(() => {
    api.getUsageServices(district).then((d) => setData(d ?? [])).catch(console.error);
  }, [district]);

  const columns = [
    { key: 'service_label', header: 'Service' },
    { key: 'n_oui', header: 'Fonctionnel' },
    { key: 'n_total', header: 'Total' },
    { key: 'pct_fonctionnel', header: '% Fonctionnel', render: (row: Record<string, unknown>) => `${(row.pct_fonctionnel as number).toFixed(1)}%` },
  ];

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <ExportCSV data={data as unknown as Record<string, unknown>[]} columns={columns} filename="services" />
      </div>

      <ResponsiveContainer width="100%" height={Math.max(300, data.length * 25)}>
        <BarChart data={data} layout="vertical" margin={{ left: 120 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis type="number" domain={[0, 100]} unit="%" />
          <YAxis type="category" dataKey="service_label" tick={{ fontSize: 10 }} width={110} />
          <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
          <Bar dataKey="pct_fonctionnel" name="% fonctionnel">
            {data.map((_, i) => (
              <Cell key={i} fill={data[i].pct_fonctionnel >= 50 ? '#22c55e' : '#ef4444'} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>

      <DataTable columns={columns} data={data as unknown as Record<string, unknown>[]} />

      <MethodNote title="Méthodologie - Services">
        <p>Disponibilité de chaque service offert par les structures sanitaires.</p>
        <p>Les valeurs possibles sont : <strong>Oui, fonctionnel</strong> (code "oui"), <strong>Prévu mais non fonctionnel</strong> (code "oui_pas_fonctionnel"), <strong>Non</strong> (code "non").</p>
        <p><strong>% Fonctionnel</strong> = nombre de structures ayant répondu "oui" / nombre total de structures ayant renseigné ce service x 100.</p>
      </MethodNote>
    </div>
  );
}

// ==================== MATRICE SERVICES x DISTRICT ====================

function MatriceTab() {
  const [data, setData] = useState<ServiceMatrixRow[]>([]);
  const [districts, setDistricts] = useState<string[]>([]);

  useEffect(() => {
    api.getServiceMatrix().then((rows) => {
      const safeRows = rows ?? [];
      setData(safeRows);
      const allDistricts = new Set<string>();
      safeRows.forEach((r) => Object.keys(r.districts ?? {}).forEach((d) => allDistricts.add(d)));
      setDistricts([...allDistricts].sort());
    }).catch(console.error);
  }, []);

  const cellColor = (pct: number) => {
    if (pct >= 75) return 'bg-green-100 text-green-800';
    if (pct >= 50) return 'bg-yellow-100 text-yellow-800';
    if (pct >= 25) return 'bg-orange-100 text-orange-800';
    if (pct > 0) return 'bg-red-100 text-red-800';
    return 'bg-gray-50 text-gray-400';
  };

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500">% de structures où le service est fonctionnel, par district. Survolez pour voir la valeur.</p>
      <div className="overflow-x-auto">
        <table className="text-xs border-collapse">
          <thead>
            <tr>
              <th className="sticky left-0 bg-white px-2 py-1 text-left font-medium text-gray-500 border-b min-w-[200px]">Service</th>
              <th className="px-2 py-1 font-medium text-gray-500 border-b text-center">Global</th>
              {districts.map((d) => (
                <th key={d} className="px-1 py-1 font-medium text-gray-500 border-b text-center" style={{ writingMode: 'vertical-rl', maxHeight: 120 }}>
                  {d}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((row) => (
              <tr key={row.service_code}>
                <td className="sticky left-0 bg-white px-2 py-1 border-b text-gray-700 font-medium">{row.service_label}</td>
                <td className={`px-2 py-1 border-b text-center font-medium ${cellColor(row.overall)}`} title={`${row.overall.toFixed(1)}%`}>
                  {row.overall.toFixed(0)}
                </td>
                {districts.map((d) => {
                  const val = row.districts[d] ?? 0;
                  return (
                    <td key={d} className={`px-1 py-1 border-b text-center ${cellColor(val)}`} title={`${row.service_label} - ${d}: ${val.toFixed(1)}%`}>
                      {val > 0 ? val.toFixed(0) : '-'}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <MethodNote title="Méthodologie - Matrice services x district">
        <p>Tableau croisé montrant le pourcentage de structures où chaque service est déclaré fonctionnel, ventilé par district.</p>
        <p>Code couleur : <span className="text-green-700">vert (&ge;75%)</span>, <span className="text-yellow-700">jaune (50-74%)</span>, <span className="text-orange-700">orange (25-49%)</span>, <span className="text-red-700">rouge (&lt;25%)</span>.</p>
      </MethodNote>
    </div>
  );
}

// ==================== EQUIPEMENTS ====================

function EquipementsTab({ district }: { district: string }) {
  const [focus, setFocus] = useUrlState('focus', 'all');
  const [data, setData] = useState<UsageEquipement[]>([]);

  useEffect(() => {
    api.getUsageEquipements(focus, district).then((d) => setData(d ?? [])).catch(console.error);
  }, [focus, district]);

  const columns = [
    { key: 'label', header: 'Équipement' },
    { key: 'category', header: 'Catégorie' },
    { key: 'sum_total', header: 'Total' },
    { key: 'sum_fonct', header: 'Fonctionnel' },
    { key: 'pct_fonct', header: '% Fonctionnel', render: (row: Record<string, unknown>) => `${(row.pct_fonct as number).toFixed(1)}%` },
  ];

  // Stacked bar chart data
  const chartData = data.map((d) => ({
    name: d.label,
    fonctionnel: d.sum_fonct,
    non_fonctionnel: Math.max(0, d.sum_total - d.sum_fonct),
  }));

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex gap-2 flex-wrap">
          {['all', 'chaine_froid', 'imagerie', 'transport', 'hospitalisation', 'laboratoire', 'autre'].map((v) => (
            <button key={v} onClick={() => setFocus(v)}
              className={`px-2 py-1 text-xs rounded ${focus === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}>
              {v === 'all' ? 'Tous' : v.replace('_', ' ')}
            </button>
          ))}
        </div>
        <ExportCSV data={data as unknown as Record<string, unknown>[]} columns={columns} filename={`equipements_${focus}`} />
      </div>

      {/* Stacked bar chart */}
      {chartData.length > 0 && chartData.length <= 20 && (
        <ResponsiveContainer width="100%" height={Math.max(300, chartData.length * 30)}>
          <BarChart data={chartData} layout="vertical" margin={{ left: 120 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis type="number" />
            <YAxis type="category" dataKey="name" tick={{ fontSize: 10 }} width={110} />
            <Tooltip />
            <Legend />
            <Bar dataKey="fonctionnel" name="Fonctionnel" stackId="a" fill="#22c55e" />
            <Bar dataKey="non_fonctionnel" name="Non fonctionnel" stackId="a" fill="#ef4444" />
          </BarChart>
        </ResponsiveContainer>
      )}

      <DataTable columns={columns} data={data as unknown as Record<string, unknown>[]} />

      <MethodNote title="Méthodologie - Équipements">
        <p>Fonctionnalité des équipements par catégorie.</p>
        <p><strong>Total</strong> = somme des quantités totales déclarées. <strong>Fonctionnel</strong> = somme des quantités fonctionnelles (en service).</p>
        <p><strong>% Fonctionnel</strong> = total fonctionnel / total x 100. Un taux &gt;100% indique une incohérence dans les données (fonctionnel &gt; total).</p>
        <p>Catégories : <strong>Chaîne du froid</strong> (réfrigérateurs, congélateurs, porte-vaccins, glacières), <strong>Imagerie</strong> (échographes, radio, scanner, IRM), <strong>Transport</strong> (ambulances, motos, véhicules, tricycles), <strong>Hospitalisation</strong> (lits), <strong>Laboratoire</strong> (microscopes).</p>
      </MethodNote>
    </div>
  );
}

// ==================== RESSOURCES HUMAINES ====================

function RHTab({ district }: { district: string }) {
  const [data, setData] = useState<UsageRH[]>([]);
  const [summary, setSummary] = useState<RHSummaryResult | null>(null);

  useEffect(() => {
    api.getUsageRH(district).then((d) => setData(d ?? [])).catch(console.error);
    api.getRHSummary(district).then(setSummary).catch(console.error);
  }, [district]);

  const columns = [
    { key: 'label', header: 'Profil' },
    { key: 'effectif_fonc', header: 'Fonctionnaires' },
    { key: 'effectif_contr', header: 'Contractuels' },
    { key: 'effectif_benev', header: 'Bénévoles' },
    { key: 'effectif_total', header: 'Total' },
  ];

  const pieData = summary ? [
    { name: 'Fonctionnaires', value: summary.total_fonc, fill: '#3b82f6' },
    { name: 'Contractuels', value: summary.total_contr, fill: '#f59e0b' },
    { name: 'Bénévoles', value: summary.total_benev, fill: '#22c55e' },
  ].filter((d) => d.value > 0) : [];

  return (
    <div className="space-y-4">
      {/* KPI Cards */}
      {summary && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
          <div className="bg-gray-50 rounded-lg p-3 sm:p-4 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Effectif total</p>
            <p className="text-xl sm:text-2xl font-bold text-gray-900 mt-1">{summary.total_effectif.toLocaleString()}</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 sm:p-4 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Médecins / structure</p>
            <p className="text-xl sm:text-2xl font-bold text-gray-900 mt-1">{summary.ratio_med_per_structure.toFixed(2)}</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 sm:p-4 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Structures avec médecin</p>
            <p className="text-xl sm:text-2xl font-bold text-blue-600 mt-1">{(summary.n_structures - summary.n_structures_sans_medecin).toLocaleString('fr-FR')}</p>
            <p className="text-xs text-gray-400">{(100 - summary.pct_structures_sans_medecin).toFixed(1)}% des structures</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 sm:p-4 text-center">
            <p className="text-xs sm:text-sm text-gray-500">Structures analysées</p>
            <p className="text-xl sm:text-2xl font-bold text-gray-900 mt-1">{summary.n_structures.toLocaleString('fr-FR')}</p>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Donut chart */}
        {pieData.length > 0 && (
          <div>
            <h4 className="text-sm font-medium text-gray-500 mb-3">Répartition par statut</h4>
            <ResponsiveContainer width="100%" height={250}>
              <PieChart>
                <Pie data={pieData} dataKey="value" nameKey="name" cx="50%" cy="50%" innerRadius={50} outerRadius={90}>
                  {pieData.map((entry, i) => (
                    <Cell key={i} fill={entry.fill} />
                  ))}
                </Pie>
                <Tooltip />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </div>
        )}

        {/* Table */}
        <div className="xl:col-span-2">
          <div className="flex justify-end mb-2">
            <ExportCSV data={data as unknown as Record<string, unknown>[]} columns={columns} filename="ressources_humaines" />
          </div>
          <DataTable columns={columns} data={data as unknown as Record<string, unknown>[]} />
        </div>
      </div>

      <MethodNote title="Méthodologie - Ressources humaines">
        <p>Effectifs du personnel de santé par profil et statut d'emploi.</p>
        <p><strong>Fonctionnaires</strong> = agents de la fonction publique. <strong>Contractuels</strong> = personnel sous contrat. <strong>Bénévoles</strong> = personnel bénévole.</p>
        <p><strong>Médecins / structure</strong> = nombre total de médecins (généralistes + spécialistes) / nombre de structures.</p>
        <p><strong>Structures sans médecin</strong> = structures où aucun data element médecin (généraliste, chirurgien, gynécologue, pédiatre, anesthésiste, urgentiste, santé publique, autre spécialiste) n'a une valeur &gt; 0.</p>
      </MethodNote>
    </div>
  );
}

// ==================== COMMODITES ====================

function CommoditesTab({ district }: { district: string }) {
  const [data, setData] = useState<UsageCommodite[]>([]);

  useEffect(() => {
    api.getUsageCommodites(district).then((d) => setData(d ?? [])).catch(console.error);
  }, [district]);

  const labels: Record<string, string> = {
    energie: 'Dispose d\'une source d\'énergie',
    eau_pts_critiques: 'Eau aux points critiques',
    energie_solaire: 'Solaire',
    energie_reseau: 'Réseau électrique',
    energie_generateur: 'Générateur',
    source_eau_réseau: 'Réseau public',
    source_eau_puit: 'Puits / puits amélioré',
    source_eau_FMH: 'Forage motricité humaine',
    source_eau_FEM: 'Forage motricité électrique/solaire',
    source_eau_aucune: 'Aucune source d\'eau',
    source_eau_total: 'Total structures (eau)',
  };

  const mainIndicators = data.filter((c) => ['energie', 'eau_pts_critiques'].includes(c.indicator));
  const energyTypes = data.filter((c) => ['energie_solaire', 'energie_reseau', 'energie_generateur'].includes(c.indicator));
  const waterTypes = data.filter((c) => c.indicator.startsWith('source_eau_') && c.indicator !== 'source_eau_total');
  const waterTotal = data.find((c) => c.indicator === 'source_eau_total');

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
        {mainIndicators.map((c) => {
          const color = c.pct >= 50 ? 'text-green-600' : c.pct >= 25 ? 'text-yellow-600' : 'text-red-600';
          return (
            <div key={c.indicator} className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
              <p className="text-xs sm:text-sm font-medium text-gray-500">{labels[c.indicator] || c.indicator}</p>
              <p className={`text-xl sm:text-3xl font-bold mt-1 sm:mt-2 ${color}`}>{c.pct.toFixed(1)}%</p>
              <p className="text-xs text-gray-400 mt-1">{c.n_oui} / {c.n_total} structures</p>
              <div className="mt-3 w-full bg-gray-200 rounded-full h-2">
                <div className="h-2 rounded-full" style={{ width: `${Math.min(c.pct, 100)}%`, backgroundColor: c.indicator === 'energie' ? '#f59e0b' : '#3b82f6' }} />
              </div>
            </div>
          );
        })}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div>
          <h4 className="text-sm font-semibold text-gray-700 mb-3">Sources d'énergie (par type)</h4>
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead><tr className="border-b bg-gray-50">
                <th className="text-left px-3 py-2 font-medium text-gray-500">Type</th>
                <th className="text-right px-3 py-2 font-medium text-gray-500">Structures</th>
                <th className="text-right px-3 py-2 font-medium text-gray-500">Total</th>
                <th className="text-right px-3 py-2 font-medium text-gray-500">%</th>
                <th className="px-3 py-2 w-32"></th>
              </tr></thead>
              <tbody>
                {energyTypes.map((c) => (
                  <tr key={c.indicator} className="border-b border-gray-100">
                    <td className="px-3 py-2 font-medium text-gray-800">{labels[c.indicator] || c.indicator}</td>
                    <td className="px-3 py-2 text-right text-gray-700">{c.n_oui.toLocaleString('fr-FR')}</td>
                    <td className="px-3 py-2 text-right text-gray-500">{c.n_total.toLocaleString('fr-FR')}</td>
                    <td className="px-3 py-2 text-right font-medium text-gray-900">{c.pct.toFixed(1)}%</td>
                    <td className="px-3 py-2"><div className="w-full bg-gray-200 rounded-full h-2"><div className="h-2 rounded-full bg-amber-500" style={{ width: `${Math.min(c.pct, 100)}%` }} /></div></td>
                  </tr>
                ))}
                {energyTypes.length === 0 && <tr><td colSpan={5} className="px-3 py-4 text-center text-gray-400">Aucune donnée</td></tr>}
              </tbody>
            </table>
          </div>
        </div>

        <div>
          <h4 className="text-sm font-semibold text-gray-700 mb-3">
            Source d'eau principale
            {waterTotal && <span className="font-normal text-gray-400 ml-2">({waterTotal.n_total} structures)</span>}
          </h4>
          <div className="bg-white border border-gray-200 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead><tr className="border-b bg-gray-50">
                <th className="text-left px-3 py-2 font-medium text-gray-500">Type</th>
                <th className="text-right px-3 py-2 font-medium text-gray-500">Structures</th>
                <th className="text-right px-3 py-2 font-medium text-gray-500">%</th>
                <th className="px-3 py-2 w-32"></th>
              </tr></thead>
              <tbody>
                {waterTypes.map((c) => {
                  const pctOfTotal = waterTotal && waterTotal.n_total > 0 ? (c.n_oui / waterTotal.n_total) * 100 : 0;
                  const isAucune = c.indicator === 'source_eau_aucune';
                  return (
                    <tr key={c.indicator} className="border-b border-gray-100">
                      <td className={`px-3 py-2 font-medium ${isAucune ? 'text-red-600' : 'text-gray-800'}`}>{labels[c.indicator] || c.indicator.replace('source_eau_', '')}</td>
                      <td className="px-3 py-2 text-right text-gray-700">{c.n_oui}</td>
                      <td className="px-3 py-2 text-right font-medium text-gray-900">{pctOfTotal.toFixed(1)}%</td>
                      <td className="px-3 py-2"><div className="w-full bg-gray-200 rounded-full h-2"><div className={`h-2 rounded-full ${isAucune ? 'bg-red-500' : 'bg-blue-500'}`} style={{ width: `${Math.min(pctOfTotal, 100)}%` }} /></div></td>
                    </tr>
                  );
                })}
                {waterTypes.length === 0 && <tr><td colSpan={4} className="px-3 py-4 text-center text-gray-400">Aucune donnée</td></tr>}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <MethodNote title="Méthodologie - Commodités (WASH / Énergie)">
        <p><strong>Dispose d'une source d'énergie</strong> : champ booléen ISS_ENERGIE_OUI_NON_DE = true.</p>
        <p><strong>Types de source d'énergie</strong> : champs booléens indépendants (multi-choix possible). Une structure peut avoir réseau + solaire + générateur.</p>
        <p><strong>Eau aux points critiques</strong> : champ booléen ISS_EAU_DISPO_PTS_CRITIQUES = true.</p>
        <p><strong>Source d'eau principale</strong> : champ à choix unique (optionSet). Les pourcentages sont calculés sur le nombre total de structures ayant renseigné ce champ.</p>
      </MethodNote>
    </div>
  );
}

// ==================== STRUCTURES FERMEES ====================

function ClosedOUsTab({ district }: { district: string }) {
  const [data, setData] = useState<ClosedOUItem[]>([]);

  useEffect(() => {
    api.getClosedOUs(district).then((d) => setData(d ?? [])).catch(console.error);
  }, [district]);

  const withData = data.filter((d) => d.has_data);
  const withoutData = data.filter((d) => !d.has_data);

  const columns = [
    { key: 'name', header: 'Structure' },
    { key: 'district', header: 'District' },
    { key: 'region', header: 'Region' },
    { key: 'closed_date', header: 'Date de fermeture' },
    {
      key: 'has_data',
      header: 'Données',
      render: (row: Record<string, unknown>) =>
        row.has_data
          ? <span className="text-red-600 font-medium">Oui (après fermeture)</span>
          : <span className="text-gray-400">Non</span>,
    },
  ];

  return (
    <div className="space-y-6">
      {/* KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-4">
        <div className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
          <p className="text-xs sm:text-sm text-gray-500">Structures fermées</p>
          <p className="text-xl sm:text-3xl font-bold text-gray-900 mt-1 sm:mt-2">{data.length.toLocaleString('fr-FR')}</p>
          <p className="text-xs text-gray-400">assignées au programme ISS</p>
        </div>
        <div className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
          <p className="text-xs sm:text-sm text-gray-500">Avec données soumises</p>
          <p className="text-xl sm:text-3xl font-bold text-red-600 mt-1 sm:mt-2">{withData.length}</p>
          <p className="text-xs text-gray-400">ont rapporté après fermeture</p>
        </div>
        <div className="bg-gray-50 rounded-lg p-3 sm:p-5 text-center">
          <p className="text-xs sm:text-sm text-gray-500">Sans données</p>
          <p className="text-xl sm:text-3xl font-bold text-gray-600 mt-1 sm:mt-2">{withoutData.length}</p>
          <p className="text-xs text-gray-400">fermées, pas de soumission</p>
        </div>
      </div>

      <div className="flex justify-end">
        <ExportCSV
          data={data as unknown as Record<string, unknown>[]}
          columns={[
            { key: 'name', header: 'Structure' },
            { key: 'uid', header: 'UID' },
            { key: 'district', header: 'District' },
            { key: 'region', header: 'Region' },
            { key: 'closed_date', header: 'Date de fermeture' },
            { key: 'has_data', header: 'Données après fermeture' },
          ]}
          filename="structures_fermees"
        />
      </div>

      {/* Table with data (problematic) */}
      {withData.length > 0 && (
        <div>
          <h4 className="text-sm font-semibold text-red-700 mb-2">Structures fermées ayant soumis des données</h4>
          <DataTable columns={columns} data={withData as unknown as Record<string, unknown>[]} />
        </div>
      )}

      {/* Full table */}
      <div>
        <h4 className="text-sm font-semibold text-gray-700 mb-2">Toutes les structures fermées ({data.length})</h4>
        <DataTable columns={columns} data={data as unknown as Record<string, unknown>[]} />
      </div>

      <MethodNote title="Méthodologie - Structures fermées">
        <p>Liste des unités organisationnelles ayant une <strong>closedDate</strong> renseignée dans DHIS2 et étant assignées au programme ISS.</p>
        <p><strong>Avec données</strong> : la structure a soumis au moins un formulaire ISS dont la date est postérieure à sa date de fermeture.</p>
        <p><strong>Sans données</strong> : la structure est fermée et n'a aucun formulaire ISS actif (soit jamais soumis, soit les données ont été supprimées).</p>
        <p>Ces structures devraient idéalement être désassignées du programme ISS dans DHIS2 pour ne plus être comptées dans le taux de rapportage.</p>
      </MethodNote>
    </div>
  );
}
