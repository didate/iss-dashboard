import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell, Legend } from 'recharts';
import { Users, AlertTriangle, TrendingDown } from 'lucide-react';
import { api } from '../api/client';
import { useUrlState } from '../hooks/useUrlState';
import type { DrhComparaison, DrhEffectif, DrhPyramide, DrhStructureRow, DrhSummary, Filters } from '../types';
import { typologieLabel } from '../utils/typologie';
import KpiCard from '../components/KpiCard';
import DataTable from '../components/DataTable';
import ExportCSV from '../components/ExportCSV';
import MethodNote from '../components/MethodNote';

const fmt = (v: number | null | undefined, d = 0) => (v === null || v === undefined ? '—' : v.toLocaleString('fr-FR', { minimumFractionDigits: d, maximumFractionDigits: d }));
const pct = (n: number, d: number) => (d > 0 ? `${((100 * n) / d).toFixed(1)} %` : '—');

const DIM_LABELS: Record<string, string> = {
  region: 'Région', district: 'District', sous_prefecture: 'Sous-préfecture', type: 'Type de structure',
};

/** Ratio ISS / DRH : au-dessus de 1 c'est la part hors fonction publique,
 *  en dessous l'État paie plus d'agents que les structures n'en déclarent. */
const ratioColor = (r: number | null | undefined) =>
  r === null || r === undefined ? '#d1d5db' : r < 1 ? '#ef4444' : r < 1.5 ? '#f97316' : '#22c55e';

export default function Personnel() {
  const [summary, setSummary] = useState<DrhSummary | null>(null);
  const [absent, setAbsent] = useState(false);
  const [error, setError] = useState('');
  const [filters, setFilters] = useState<Filters | null>(null);

  const [by, setBy] = useUrlState('by', 'district');
  const [categorie, setCategorie] = useUrlState('categorie');
  const [district, setDistrict] = useUrlState('district');

  const [effectifs, setEffectifs] = useState<DrhEffectif[]>([]);
  const [pyramide, setPyramide] = useState<DrhPyramide[]>([]);
  const [comparaison, setComparaison] = useState<DrhComparaison[]>([]);
  const [structures, setStructures] = useState<DrhStructureRow[]>([]);

  useEffect(() => {
    api.getFilters().then(setFilters).catch(() => {});
    api.getDrhSummary().then(setSummary).catch((e: Error) => {
      if (e.message.includes('404')) setAbsent(true);
      else setError(e.message);
    });
  }, []);

  useEffect(() => {
    if (!summary) return;
    api.getDrhEffectifs({ by, categorie, district: by === 'sous_prefecture' ? district : '' })
      .then(setEffectifs).catch((e: Error) => setError(e.message));
  }, [summary, by, categorie, district]);

  // La pyramide suit le district choisi, sinon elle est nationale.
  useEffect(() => {
    if (!summary) return;
    const scope = district ? { by: 'district', key: district } : { by: 'global', key: 'national' };
    api.getDrhPyramide({ ...scope, categorie }).then((r) => setPyramide(r.pyramide ?? []))
      .catch((e: Error) => setError(e.message));
  }, [summary, district, categorie]);

  useEffect(() => {
    if (!summary) return;
    api.getDrhComparaison({ by: district ? 'district' : 'global', key: district || 'national', categorie: categorie || '*' })
      .then(setComparaison).catch((e: Error) => setError(e.message));
  }, [summary, district, categorie]);

  useEffect(() => {
    if (!summary || !district) { setStructures([]); return; }
    api.getDrhStructuresList({ district }).then(setStructures).catch(() => {});
  }, [summary, district]);

  const catLabel = useMemo(() => {
    const m: Record<string, string> = {};
    summary?.catalogue.forEach((c) => { m[c.code] = c.label; });
    return m;
  }, [summary]);

  if (absent) {
    return (
      <div className="space-y-4">
        <h2 className="text-xl font-bold text-gray-900">Personnel de l'État</h2>
        <div className="bg-white rounded-lg border border-gray-200 p-8 text-center text-gray-500">
          <Users size={32} className="mx-auto mb-2 text-gray-300" />
          Aucun fichier de personnel n'a encore été importé. Un administrateur peut le faire dans{' '}
          <Link to="/admin?tab=drh" className="text-blue-600 hover:underline">Admin → Personnel (DRH)</Link>.
        </div>
      </div>
    );
  }
  if (!summary) return <div className="text-sm text-gray-500">Chargement…</div>;

  const n = summary.national;
  const total = n?.n_agents ?? 0;
  const zoneLabel = district || 'National';
  const zone = district ? effectifs.find((r) => r.key === district && by === 'district') : n;

  // --- Répartitions nationales -------------------------------------------
  const parCategorie = summary.categories.filter((c) => c.n_agents > 0);
  const parAffectation = n ? [
    { key: 'Structures de soins', n: n.n_structure, color: '#2563eb' },
    { key: 'Bureaux de district', n: n.n_bureau, color: '#0891b2' },
    { key: 'Administration centrale', n: n.n_centrale, color: '#7c3aed' },
    { key: 'Non rattachés', n: n.n_non_rattache, color: '#9ca3af' },
  ] : [];

  const effectifColumns = [
    { key: 'label', header: DIM_LABELS[by] ?? by, render: (r: Record<string, unknown>) => (by === 'type' ? typologieLabel(String(r.key)) : String(r.label)) },
    { key: 'n_agents', header: 'Agents', render: (r: Record<string, unknown>) => fmt(r.n_agents as number) },
    { key: 'ratio_10k', header: '/10 000 hab.', render: (r: Record<string, unknown>) => fmt(r.ratio_10k as number | null, 2) },
    { key: 'n_femmes', header: 'Femmes', render: (r: Record<string, unknown>) => pct(r.n_femmes as number, r.n_agents as number) },
    { key: 'n_structure', header: 'En structure', render: (r: Record<string, unknown>) => fmt(r.n_structure as number) },
    { key: 'n_bureau', header: 'Bureau district', render: (r: Record<string, unknown>) => fmt(r.n_bureau as number) },
    { key: 'n_depart_5ans', header: 'Départs 5 ans', render: (r: Record<string, unknown>) => (
      <span title={`${r.n_depart_5ans} sur ${r.n_age_connu} âges connus`}>
        {fmt(r.n_depart_5ans as number)} <span className="text-gray-400">({pct(r.n_depart_5ans as number, r.n_age_connu as number)})</span>
      </span>
    ) },
  ];

  const comparaisonColumns = [
    { key: 'label', header: 'Catégorie', render: (r: Record<string, unknown>) => catLabel[String(r.categorie)] ?? (String(r.categorie) || 'Toutes catégories') },
    { key: 'n_drh', header: 'Payés par l\'État', render: (r: Record<string, unknown>) => fmt(r.n_drh as number) },
    { key: 'n_iss', header: 'Déclarés (ISS)', render: (r: Record<string, unknown>) => fmt(r.n_iss as number | null) },
    { key: 'ecart', header: 'Écart', render: (r: Record<string, unknown>) => {
      const e = r.ecart as number | null;
      return e === null ? '—' : <span className={e < 0 ? 'text-red-600 font-medium' : 'text-gray-700'}>{e > 0 ? '+' : ''}{fmt(e)}</span>;
    } },
    { key: 'ratio', header: 'ISS / DRH', render: (r: Record<string, unknown>) => {
      const v = r.ratio as number | null;
      return (
        <span className="inline-flex items-center gap-2">
          <span className="w-2.5 h-2.5 rounded-full" style={{ background: ratioColor(v) }} />
          {v === null || v === undefined ? '—' : `×${v.toFixed(2)}`}
        </span>
      );
    } },
  ];

  const structureColumns = [
    { key: 'name', header: 'Structure', render: (r: Record<string, unknown>) => (
      <Link to={`/structure/${r.event_uid}`} className="text-blue-600 hover:underline">{String(r.name)}</Link>
    ) },
    { key: 'type_code', header: 'Type', render: (r: Record<string, unknown>) => typologieLabel(String(r.type_code)) },
    { key: 'n_agents', header: 'Agents de l\'État', render: (r: Record<string, unknown>) => (
      (r.n_agents as number) === 0
        ? <span className="text-red-600">0</span>
        : fmt(r.n_agents as number)
    ) },
    { key: 'n_femmes', header: 'Femmes', render: (r: Record<string, unknown>) => pct(r.n_femmes as number, r.n_agents as number) },
    { key: 'n_depart_5ans', header: 'Départs 5 ans', render: (r: Record<string, unknown>) => fmt(r.n_depart_5ans as number) },
  ];

  // Le tri doit être fait AVANT de générer les <Cell> : recharts les applique
  // dans l'ordre des données, pas dans celui du tableau d'origine.
  const chartEffectifs = effectifs
    .filter((r) => r.n_agents > 0)
    .slice(0, 45)
    .map((r) => ({ key: r.key, name: r.label, ratio: r.ratio_10k ?? null, agents: r.n_agents }))
    .sort((a, b) => (b.ratio ?? -1) - (a.ratio ?? -1) || b.agents - a.agents);
  // Couleur relative à la densité nationale : moins de la moitié = rouge,
  // sous la moyenne = orange, au-dessus = vert. Un seuil absolu n'aurait
  // pas de sens, aucune norme de densité n'étant fixée.
  const densiteNationale = n?.ratio_10k ?? 0;
  const densiteColor = (r: number | null) =>
    r === null ? '#d1d5db' : r < densiteNationale / 2 ? '#ef4444' : r < densiteNationale ? '#f97316' : '#22c55e';
  // Sans population dans le snapshot DHIS2, aucune densité n'est calculable :
  // le graphe retombe sur les effectifs bruts plutôt que de n'afficher aucune
  // barre, ce qui donnerait l'impression que la page est cassée.
  const hasDensite = chartEffectifs.some((r) => r.ratio !== null);
  const sansAgent = structures.filter((s) => s.n_agents === 0).length;
  const incoherences = comparaison.filter((r) => r.ratio !== null && r.ratio !== undefined && r.ratio < 1);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <h2 className="text-xl font-bold text-gray-900">Personnel de l'État</h2>
        <span className="text-xs text-gray-500">
          {summary.import.label} · {fmt(summary.import.n_agents)} agents · retraite à {summary.import.age_retraite} ans ·
          importé le {new Date(summary.import.imported_at).toLocaleDateString('fr-FR')}
        </span>
      </div>
      {error && <div className="text-sm text-red-600">{error}</div>}

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <KpiCard title={`Agents de l'État — ${zoneLabel}`} value={fmt(zone?.n_agents ?? 0)} subtitle={`${pct(zone?.n_femmes ?? 0, zone?.n_agents ?? 0)} de femmes`} icon={<Users size={18} />} />
        <KpiCard
          title="Densité"
          value={zone?.ratio_10k != null ? fmt(zone.ratio_10k, 2) : '—'}
          subtitle={zone?.ratio_10k != null ? 'agents pour 10 000 habitants' : 'population non renseignée dans le dernier instantané DHIS2'}
        />
        <KpiCard title="En structure de soins" value={pct(zone?.n_structure ?? 0, zone?.n_agents ?? 0)} subtitle={`${fmt(zone?.n_structure ?? 0)} agents · ${fmt(summary.import.n_structures)} structures couvertes`} />
        <KpiCard
          title="Départs d'ici 5 ans"
          value={pct(zone?.n_depart_5ans ?? 0, zone?.n_age_connu ?? 0)}
          subtitle={`${fmt(zone?.n_depart_5ans ?? 0)} agents sur ${fmt(zone?.n_age_connu ?? 0)} âges connus`}
          color="text-orange-600"
          icon={<TrendingDown size={18} />}
        />
      </div>

      {/* Filtres */}
      <div className="flex flex-wrap gap-2 bg-white p-3 rounded-lg border border-gray-200 items-center">
        <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={district} onChange={(e) => setDistrict(e.target.value)}>
          <option value="">Tous districts (national)</option>
          {filters?.districts.map((d) => <option key={d} value={d}>{d}</option>)}
        </select>
        <select className="border border-gray-300 rounded px-2 py-1.5 text-sm" value={categorie} onChange={(e) => setCategorie(e.target.value)}>
          <option value="">Toutes professions</option>
          {summary.catalogue.map((c) => <option key={c.code} value={c.code}>{c.label}</option>)}
        </select>
        {(district || categorie) && (
          <button onClick={() => { setDistrict(''); setCategorie(''); }} className="text-xs text-gray-500 hover:text-gray-800 underline">Réinitialiser</button>
        )}
      </div>

      {/* Répartition nationale */}
      <div className="grid lg:grid-cols-2 gap-4">
        <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
          <h3 className="font-semibold text-gray-800">Par catégorie professionnelle — national</h3>
          <ResponsiveContainer width="100%" height={Math.max(220, parCategorie.length * 18)}>
            <BarChart data={parCategorie.map((c) => ({ name: catLabel[c.categorie] ?? c.categorie, n: c.n_agents }))} layout="vertical" margin={{ left: 10, right: 30 }}>
              <CartesianGrid strokeDasharray="3 3" horizontal={false} />
              <XAxis type="number" fontSize={11} />
              <YAxis type="category" dataKey="name" width={200} fontSize={11} interval={0} />
              <Tooltip formatter={(v: number) => [fmt(v), 'Agents']} />
              <Bar dataKey="n" fill="#2563eb" radius={[0, 3, 3, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
          <h3 className="font-semibold text-gray-800">Où sont affectés les agents — national</h3>
          <div className="space-y-2">
            {parAffectation.map((a) => (
              <div key={a.key}>
                <div className="flex justify-between text-sm">
                  <span className="text-gray-700">{a.key}</span>
                  <span className="text-gray-500">{fmt(a.n)} · {pct(a.n, total)}</span>
                </div>
                <div className="h-2 bg-gray-100 rounded mt-1 overflow-hidden">
                  <div className="h-full rounded" style={{ width: `${(100 * a.n) / Math.max(1, total)}%`, background: a.color }} />
                </div>
              </div>
            ))}
          </div>
          <p className="text-xs text-gray-500 pt-2 border-t border-gray-100">
            Un district compte ses structures, son bureau et ses agents non rattachés. L'administration centrale
            n'est comptée qu'au national. {fmt(summary.import.n_non_rattache)} agents n'ont pas pu être rattachés
            à une structure — la liste est dans l'écran d'import.
          </p>
        </div>
      </div>

      {/* Effectifs par zone */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-800 mr-2">Effectifs par zone</h3>
          {['region', 'district', 'sous_prefecture', 'type'].map((v) => (
            <button key={v} onClick={() => setBy(v)} className={`px-2 py-1 text-xs rounded ${by === v ? 'bg-gray-800 text-white' : 'bg-gray-100 text-gray-600'}`}>
              {DIM_LABELS[v]}
            </button>
          ))}
          <span className="text-xs text-gray-400">
            {categorie ? catLabel[categorie] : 'toutes professions'}
            {by === 'sous_prefecture' && !district && ' · choisissez un district pour réduire la liste'}
          </span>
          <div className="ml-auto"><ExportCSV data={effectifs as unknown as Record<string, unknown>[]} columns={effectifColumns} filename={`personnel_${by}`} /></div>
        </div>
        {by !== 'type' && chartEffectifs.length > 1 && (
          <p className="text-xs text-gray-500">
            {hasDensite ? (
              <>
                Densité pour 10 000 habitants. Couleur relative à la moyenne nationale ({fmt(densiteNationale, 2)}) :
                rouge sous la moitié, orange sous la moyenne, vert au-dessus — aucune norme de densité n'étant fixée.
              </>
            ) : (
              <>
                Effectifs bruts : aucune densité n'est calculable, la population n'étant pas renseignée dans le
                dernier instantané DHIS2 (variable <code>DHIS2_POPULATION_DX</code>, puis une nouvelle synchronisation).
              </>
            )}
          </p>
        )}
        {by !== 'type' && chartEffectifs.length > 1 && (
          <ResponsiveContainer width="100%" height={Math.max(200, chartEffectifs.length * 18)}>
            <BarChart data={chartEffectifs} layout="vertical" margin={{ left: 10, right: 30 }}>
              <CartesianGrid strokeDasharray="3 3" horizontal={false} />
              <XAxis type="number" fontSize={11} />
              <YAxis type="category" dataKey="name" width={150} fontSize={11} interval={0} />
              <Tooltip
                formatter={(v: number, _n, p) =>
                  hasDensite
                    ? [`${v.toFixed(2)} /10 000 hab. (${fmt(p.payload.agents)} agents)`, 'Densité']
                    : [fmt(v), 'Agents']
                }
              />
              <Bar dataKey={hasDensite ? 'ratio' : 'agents'} radius={[0, 3, 3, 0]} fill="#2563eb">
                {hasDensite && chartEffectifs.map((r) => <Cell key={r.key} fill={densiteColor(r.ratio)} />)}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        )}
        <DataTable columns={effectifColumns} data={effectifs as unknown as Record<string, unknown>[]} />
      </div>

      {/* Pyramide des âges */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-800">Pyramide des âges — {zoneLabel}</h3>
          <span className="text-xs text-gray-400">
            {categorie ? catLabel[categorie] : 'toutes professions'} · départ à {summary.import.age_retraite} ans, âges au {summary.import.annee}
          </span>
        </div>
        <ResponsiveContainer width="100%" height={260}>
          <BarChart data={pyramide.map((p) => ({ name: p.tranche, femmes: p.n_femmes, hommes: p.n_agents - p.n_femmes }))}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} />
            <XAxis dataKey="name" fontSize={11} />
            <YAxis fontSize={11} />
            <Tooltip formatter={(v: number, n) => [fmt(v), n === 'femmes' ? 'Femmes' : 'Hommes']} />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            <Bar dataKey="femmes" name="Femmes" stackId="s" fill="#db2777" />
            <Bar dataKey="hommes" name="Hommes" stackId="s" fill="#2563eb" />
          </BarChart>
        </ResponsiveContainer>
        <p className="text-xs text-gray-500">
          La tranche « inconnu » regroupe les agents sans année de naissance : ils comptent dans l'effectif mais
          sont exclus du calcul des départs, sinon le taux serait sous-estimé.
        </p>
      </div>

      {/* Comparaison DRH ↔ ISS */}
      <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="font-semibold text-gray-800 mr-2">Payés par l'État / déclarés dans ISS — {zoneLabel}</h3>
          {incoherences.length > 0 && (
            <span className="inline-flex items-center gap-1 text-xs text-red-700 bg-red-50 border border-red-200 rounded px-2 py-0.5">
              <AlertTriangle size={12} /> {incoherences.length} catégorie{incoherences.length > 1 ? 's' : ''} où l'État paie plus que déclaré
            </span>
          )}
          <div className="ml-auto"><ExportCSV data={comparaison as unknown as Record<string, unknown>[]} columns={comparaisonColumns} filename={`personnel_comparaison_${district || 'national'}`} /></div>
        </div>
        <DataTable columns={comparaisonColumns} data={comparaison as unknown as Record<string, unknown>[]} />
        <p className="text-xs text-gray-500">
          ISS compte le personnel <strong>présent</strong> déclaré par la structure, la DRH ceux qu'elle <strong>paie</strong>.
          Un ratio supérieur à 1 est normal : il mesure la part de personnel hors fonction publique. Un ratio
          <strong> inférieur à 1</strong> est une anomalie — défaut de déclaration ISS, ou agents affectés mais absents.
        </p>
      </div>

      {/* Structures du district */}
      {district && (
        <div className="bg-white rounded-lg border border-gray-200 p-4 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="font-semibold text-gray-800 mr-2">Structures de {district}</h3>
            {sansAgent > 0 && (
              <span className="text-xs text-red-700 bg-red-50 border border-red-200 rounded px-2 py-0.5">
                {sansAgent} structure{sansAgent > 1 ? 's' : ''} sans aucun agent de l'État
              </span>
            )}
            <div className="ml-auto"><ExportCSV data={structures as unknown as Record<string, unknown>[]} columns={structureColumns} filename={`personnel_structures_${district}`} /></div>
          </div>
          <DataTable columns={structureColumns} data={structures as unknown as Record<string, unknown>[]} />
        </div>
      )}

      <MethodNote title="Comment ces chiffres sont calculés">
        <p>
          <strong>Source.</strong> Fichier annuel de la DRH/CNPS du Ministère : les agents <strong>payés par l'État</strong>.
          Il ne couvre ni les contractuels des collectivités, ni les bénévoles, ni le personnel du privé — d'où des
          effectifs bien inférieurs à ceux déclarés par les structures dans ISS.
        </p>
        <p>
          <strong>Rattachement.</strong> Le libellé d'affectation écrit par la DRH est rapproché des structures ISS par
          une table de correspondance validée à la main, puis par nom exact, sigle de bureau, et type + nom propre dans
          le district de l'agent. Ce qui reste ambigu n'est jamais rattaché au hasard : {fmt(summary.import.n_non_rattache)} agents
          sur {fmt(summary.import.n_agents)} restent non rattachés ({pct(summary.import.n_agents - summary.import.n_non_rattache, summary.import.n_agents)} catégorisés).
        </p>
        <p>
          <strong>Densité.</strong> Agents ÷ population de la zone × 10 000, avec la même population DHIS2 que le reste
          du tableau de bord. Une zone sans population connue n'a pas de densité.
        </p>
        <p>
          <strong>Départs.</strong> Agents atteignant {summary.import.age_retraite} ans dans les 5 ans, rapportés aux
          agents dont l'année de naissance est connue. L'âge de départ est un paramètre (<code>DRH_AGE_RETRAITE</code>),
          faute de référence officielle pour la fonction publique guinéenne.
        </p>
        <p>
          <strong>Aucune donnée nominative.</strong> Ni matricule, ni nom, ni date de naissance exacte n'entrent dans
          l'application : le fichier est agrégé à l'import et seuls les effectifs sont conservés. Rien de tout cela
          n'est publié dans l'espace public.
        </p>
      </MethodNote>
    </div>
  );
}
