import { useEffect, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import { api } from '../api/client';
import type {
  UsageRecensement,
  UsageService,
  UsageEquipement,
  UsageRH,
  UsageCommodite,
  Filters,
} from '../types';
import DataTable from '../components/DataTable';

const tabs = [
  { key: 'recensement', label: 'Recensement' },
  { key: 'services', label: 'Services' },
  { key: 'equipements', label: 'Équipements' },
  { key: 'rh', label: 'Ressources humaines' },
  { key: 'commodites', label: 'Commodités' },
];

export default function Usage() {
  const [tab, setTab] = useState('recensement');
  const [district, setDistrict] = useState('');
  const [filters, setFilters] = useState<Filters | null>(null);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(console.error);
  }, []);

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-gray-900">Utilisation & Analyse</h2>

      <div className="flex items-center gap-4">
        <div className="flex bg-white rounded-lg border border-gray-200 p-0.5">
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

        {tab !== 'recensement' && (
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
      </div>

      <div className="bg-white rounded-lg border border-gray-200 p-4">
        {tab === 'recensement' && <RecensementTab />}
        {tab === 'services' && <ServicesTab district={district} />}
        {tab === 'equipements' && <EquipementsTab district={district} />}
        {tab === 'rh' && <RHTab district={district} />}
        {tab === 'commodites' && <CommoditesTab district={district} />}
      </div>
    </div>
  );
}

function RecensementTab() {
  const [by, setBy] = useState('district');
  const [data, setData] = useState<UsageRecensement[]>([]);

  useEffect(() => {
    api.getUsageRecensement(by).then(setData).catch(console.error);
  }, [by]);

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        {['district', 'region', 'statut_juridique'].map((v) => (
          <button
            key={v}
            onClick={() => setBy(v)}
            className={`px-2 py-1 text-xs rounded ${by === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}
          >
            {v === 'statut_juridique' ? 'Statut juridique' : v.charAt(0).toUpperCase() + v.slice(1)}
          </button>
        ))}
      </div>

      <DataTable
        columns={[
          { key: 'label', header: by.charAt(0).toUpperCase() + by.slice(1) },
          { key: 'n_structures', header: 'Total' },
          { key: 'n_operationnel', header: 'Opérationnel' },
          { key: 'n_non_operationnel', header: 'Non opérationnel' },
          { key: 'n_ferme_temp', header: 'Fermé temp.' },
        ]}
        data={data as unknown as Record<string, unknown>[]}
      />
    </div>
  );
}

function ServicesTab({ district }: { district: string }) {
  const [data, setData] = useState<UsageService[]>([]);

  useEffect(() => {
    api.getUsageServices(district).then(setData).catch(console.error);
  }, [district]);

  return (
    <div className="space-y-4">
      <ResponsiveContainer width="100%" height={Math.max(300, data.length * 25)}>
        <BarChart data={data} layout="vertical" margin={{ left: 200 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis type="number" domain={[0, 100]} unit="%" />
          <YAxis type="category" dataKey="service_label" tick={{ fontSize: 10 }} width={190} />
          <Tooltip formatter={(v: number) => `${v.toFixed(1)}%`} />
          <Bar dataKey="pct_fonctionnel" name="% fonctionnel">
            {data.map((_, i) => (
              <Cell key={i} fill={data[i].pct_fonctionnel >= 50 ? '#22c55e' : '#ef4444'} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>

      <DataTable
        columns={[
          { key: 'service_label', header: 'Service' },
          { key: 'n_oui', header: 'Fonctionnel' },
          { key: 'n_total', header: 'Total' },
          {
            key: 'pct_fonctionnel',
            header: '% Fonctionnel',
            render: (row: Record<string, unknown>) => `${(row.pct_fonctionnel as number).toFixed(1)}%`,
          },
        ]}
        data={data as unknown as Record<string, unknown>[]}
      />
    </div>
  );
}

function EquipementsTab({ district }: { district: string }) {
  const [focus, setFocus] = useState('all');
  const [data, setData] = useState<UsageEquipement[]>([]);

  useEffect(() => {
    api.getUsageEquipements(focus, district).then(setData).catch(console.error);
  }, [focus, district]);

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        {['all', 'chaine_froid', 'imagerie', 'transport', 'hospitalisation', 'laboratoire'].map((v) => (
          <button
            key={v}
            onClick={() => setFocus(v)}
            className={`px-2 py-1 text-xs rounded ${focus === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}
          >
            {v === 'all' ? 'Tous' : v.replace('_', ' ')}
          </button>
        ))}
      </div>

      <DataTable
        columns={[
          { key: 'label', header: 'Équipement' },
          { key: 'category', header: 'Catégorie' },
          { key: 'sum_total', header: 'Total' },
          { key: 'sum_fonct', header: 'Fonctionnel' },
          {
            key: 'pct_fonct',
            header: '% Fonctionnel',
            render: (row: Record<string, unknown>) => `${(row.pct_fonct as number).toFixed(1)}%`,
          },
        ]}
        data={data as unknown as Record<string, unknown>[]}
      />
    </div>
  );
}

function RHTab({ district }: { district: string }) {
  const [data, setData] = useState<UsageRH[]>([]);

  useEffect(() => {
    api.getUsageRH(district).then(setData).catch(console.error);
  }, [district]);

  return (
    <DataTable
      columns={[
        { key: 'label', header: 'Profil' },
        { key: 'effectif_fonc', header: 'Fonctionnaires' },
        { key: 'effectif_contr', header: 'Contractuels' },
        { key: 'effectif_benev', header: 'Bénévoles' },
        { key: 'effectif_total', header: 'Total' },
      ]}
      data={data as unknown as Record<string, unknown>[]}
    />
  );
}

function CommoditesTab({ district }: { district: string }) {
  const [data, setData] = useState<UsageCommodite[]>([]);

  useEffect(() => {
    api.getUsageCommodites(district).then(setData).catch(console.error);
  }, [district]);

  const labels: Record<string, string> = {
    energie: 'Source d\'énergie',
    eau_pts_critiques: 'Eau aux points critiques',
    solaire: 'Énergie solaire',
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-3 gap-4">
        {data.map((c) => (
          <div key={c.indicator} className="bg-gray-50 rounded-lg p-4 text-center">
            <p className="text-sm text-gray-500">{labels[c.indicator] || c.indicator}</p>
            <p className="text-3xl font-bold text-gray-900 mt-2">{c.pct.toFixed(1)}%</p>
            <p className="text-xs text-gray-400 mt-1">{c.n_oui} / {c.n_total} structures</p>
          </div>
        ))}
      </div>
    </div>
  );
}
