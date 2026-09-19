import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { MapContainer, TileLayer, CircleMarker, useMap } from 'react-leaflet';
import { Search, LocateFixed, X, Loader2 } from 'lucide-react';
import { publicApi, typeColor, opLabel, opColor } from '../../api/public';
import { useUrlState } from '../../hooks/useUrlState';
import type { PublicFilters, PublicPointCollection, PublicStructureItem, PublicSummary } from '../../types';
import PointsCanvasLayer, { escapeHtml, type MarkerSpec } from '../../components/map/PointsCanvasLayer';
import InvalidateOnResize from '../../components/map/InvalidateOnResize';

const GUINEA_CENTER: [number, number] = [10.4, -11.3];
const NEAR_RADIUS_KM = 25;
const FICHE_BASE = `${import.meta.env.BASE_URL.replace(/\/$/, '')}/fs/`;

// Popup de structure : identité, rattachement, niveau/statut, RH, services & accès —
// même lecture que le popup MFL de la carte OpenHEXA, avec les données ISS.
function popupHtml(p: PublicPointCollection['features'][number]['properties']): string {
  const e = escapeHtml;
  const badge = (txt: string, cls: string) => `<span style="display:inline-block;padding:2px 8px;border-radius:999px;font-size:11px;font-weight:600;${cls}">${e(txt)}</span>`;
  const statut = p.statut === 'privée' ? badge('Privé', 'background:#ede9fe;color:#5b21b6') : p.statut === 'publique' ? badge('Public', 'background:#dbeafe;color:#1e40af') : '';
  const crumb = [p.region, p.district, p.sp].filter(Boolean).map((x, i, a) => (i === a.length - 1 ? `<span style="color:#6b7280">${e(x)}</span>` : `<b>${e(x)}</b>`)).join(' <span style="color:#9ca3af">›</span> ');
  // `== null` couvre aussi undefined : un GeoJSON antérieur (cache) ou une synchro pas encore refaite n'a pas ces champs.
  const num = (v: number | null | undefined) => (v == null ? '<span style="color:#9ca3af">—</span>' : `<b>${v}</b>`);
  const yn = (v: boolean | null | undefined) => (v == null ? '<span style="color:#9ca3af">—</span>' : v ? '<span style="color:#16a34a;font-weight:700">✓</span>' : '<span style="color:#dc2626;font-weight:700">✗</span>');
  const row = (label: string, val: string) => `<div style="display:flex;justify-content:space-between;gap:12px;padding:2px 0"><span>${e(label)}</span><span>${val}</span></div>`;
  const section = (t: string) => `<div style="margin:8px 0 2px;font-size:10px;letter-spacing:.06em;color:#6b7280;text-transform:uppercase">${e(t)}</div>`;
  const opCls = p.op === 'operationnel' ? '#15803d' : p.op === 'ferme_temporairement' ? '#b45309' : p.op ? '#b91c1c' : '#6b7280';
  const score = p.score_services == null || !p.score_services_max ? '' : `${row('Score disponibilité services', `<b>${p.score_services} / ${p.score_services_max}</b>`)}
      <div style="height:5px;border-radius:3px;background:#e5e7eb;margin-top:2px"><div style="height:5px;border-radius:3px;width:${Math.round((100 * p.score_services) / Math.max(1, p.score_services_max))}%;background:${p.score_services >= p.score_services_max * 0.7 ? '#16a34a' : p.score_services >= p.score_services_max * 0.4 ? '#f59e0b' : '#dc2626'}"></div></div>`;
  return `
    <div style="min-width:250px;font-size:12px;color:#111827">
      <div style="font-weight:700;font-size:15px;margin-bottom:6px">${e(p.name)}</div>
      <div style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:8px">${badge(p.type_label, `background:#dcfce7;color:#166534;border-left:4px solid ${typeColor(p.type)}`)}${statut}</div>
      <div style="font-size:12px;margin-bottom:6px">${crumb}</div>
      <div style="display:flex;gap:24px;border-top:1px solid #e5e7eb;padding-top:6px">
        <div><div style="font-size:10px;letter-spacing:.06em;color:#6b7280;text-transform:uppercase">Niveau</div><b>${p.niveau || '—'}</b></div>
        <div><div style="font-size:10px;letter-spacing:.06em;color:#6b7280;text-transform:uppercase">Statut</div><b style="color:${opCls}">${e(opLabel(p.op))}</b></div>
      </div>
      <div style="border-top:1px solid #e5e7eb;margin-top:6px">
        ${section('Ressources humaines')}
        ${row('Total RH', num(p.rh_total))}${row('Médecins', num(p.rh_medecins))}${row('Personnel soignant', num(p.rh_soignants))}
      </div>
      <div style="border-top:1px solid #e5e7eb;margin-top:6px">
        ${section('Services & accès')}
        ${row('Eau aux points critiques', yn(p.eau))}${row("Source d'énergie", yn(p.energie))}
        ${score}
      </div>
      <a href="${FICHE_BASE}${encodeURIComponent(p.uid)}" style="display:inline-block;margin-top:10px;font-size:12px;color:#047857;font-weight:600">Voir la fiche →</a>
    </div>`;
}

function FlyTo({ target }: { target: [number, number] | null }) {
  const map = useMap();
  useEffect(() => {
    if (target) map.flyTo(target, 12, { duration: 0.8 });
  }, [target, map]);
  return null;
}

function useDebounced(value: string, ms: number): string {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

export default function PublicMap() {
  const [search, setSearch] = useUrlState('q');
  const [type, setType] = useUrlState('type');
  const [service, setService] = useUrlState('service');
  const [region, setRegion] = useUrlState('region');
  const [district, setDistrict] = useUrlState('district');
  const debouncedSearch = useDebounced(search, 300);

  const [points, setPoints] = useState<PublicPointCollection | null>(null);
  const [filters, setFilters] = useState<PublicFilters | null>(null);
  const [summary, setSummary] = useState<PublicSummary | null>(null);
  const [results, setResults] = useState<PublicStructureItem[]>([]);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState('');
  const [me, setMe] = useState<[number, number] | null>(null);
  const [locating, setLocating] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [focus, setFocus] = useState<{ uid: string; nonce: number } | null>(null);
  const [flyTarget, setFlyTarget] = useState<[number, number] | null>(null);
  const [clusterParam, setClusterParam] = useUrlState('cluster');
  const cluster = clusterParam === 'on'; // désactivé par défaut ; ?cluster=on pour regrouper

  // Chargement unique : points (cache navigateur), filtres, résumé.
  useEffect(() => {
    Promise.all([publicApi.getPoints(), publicApi.getFilters(), publicApi.getSummary()])
      .then(([p, f, s]) => {
        setPoints(p);
        setFilters(f);
        setSummary(s);
      })
      .catch((e) => setError(e.message));
  }, []);

  // Liste de résultats : calculée côté serveur (recherche, filtres, distance).
  useEffect(() => {
    setSearching(true);
    publicApi
      .search({
        search: debouncedSearch,
        type,
        service,
        region,
        district,
        near: me ? `${me[0]},${me[1]}` : undefined,
        radius_km: me ? NEAR_RADIUS_KM : undefined,
        limit: 100,
      })
      .then(setResults)
      .catch((e) => setError(e.message))
      .finally(() => setSearching(false));
  }, [debouncedSearch, type, service, region, district, me]);

  // Points affichés : sous-ensemble du GeoJSON pré-calculé selon les filtres actifs.
  const visibleFeatures = useMemo(() => {
    if (!points) return [];
    const q = debouncedSearch.trim().toLowerCase();
    return points.features.filter((f) => {
      const p = f.properties;
      if (type && p.type !== type) return false;
      if (region && p.region !== region) return false;
      if (district && p.district !== district) return false;
      if (service && !(p.svc || []).includes(service)) return false;
      if (q && !p.name.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [points, type, region, district, service, debouncedSearch]);

  // Marqueurs : position + couleur par type + popup, dérivés du GeoJSON filtré.
  const markers = useMemo<MarkerSpec[]>(
    () =>
      visibleFeatures.map((f) => ({
        uid: f.properties.uid,
        lat: f.geometry.coordinates[1],
        lng: f.geometry.coordinates[0],
        color: typeColor(f.properties.type),
        popup: () => popupHtml(f.properties),
      })),
    [visibleFeatures],
  );

  const districtsOfRegion = useMemo(
    () => (filters ? filters.districts.filter((d) => !region || d.region === region) : []),
    [filters, region],
  );

  const locateMe = useCallback(() => {
    if (!navigator.geolocation) {
      setError('La géolocalisation n’est pas disponible sur cet appareil.');
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const here: [number, number] = [pos.coords.latitude, pos.coords.longitude];
        setMe(here);
        setFlyTarget(here);
        setLocating(false);
      },
      () => {
        setError('Position indisponible : autorisez la géolocalisation dans votre navigateur.');
        setLocating(false);
      },
      { enableHighAccuracy: true, timeout: 10000 },
    );
  }, []);

  // Depuis la liste : on met en avant la structure sur la carte (déplacement + popup).
  const selectResult = useCallback((item: PublicStructureItem) => {
    setSelected(item.uid);
    if (item.lat !== null && item.lng !== null) setFocus({ uid: item.uid, nonce: Date.now() });
  }, []);

  // Depuis la carte : simple surbrillance dans la liste, la popup s'ouvre seule, sans zoom.
  const onMarkerClick = useCallback((uid: string) => setSelected(uid), []);

  const clearAll = () => {
    setSearch('');
    setType('');
    setService('');
    setRegion('');
    setDistrict('');
    setMe(null);
    setSelected(null);
    setFocus(null);
  };

  const hasFilter = !!(search || type || service || region || district || me);
  const selectCls = 'w-full text-sm border border-gray-300 rounded-md px-2 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-emerald-500';

  return (
    <div className="flex flex-col md:flex-row h-full">
      {/* Panneau de recherche */}
      <aside className="md:w-96 shrink-0 flex flex-col border-b md:border-b-0 md:border-r border-gray-200 bg-white max-h-[55vh] md:max-h-none">
        <div className="p-3 space-y-2 border-b border-gray-100">
          <div className="relative">
            <Search size={16} className="absolute left-2.5 top-2.5 text-gray-400" />
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Nom de la structure…"
              className="w-full text-sm border border-gray-300 rounded-md pl-8 pr-2 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-500"
            />
          </div>
          <div className="grid grid-cols-2 gap-2">
            <select value={type} onChange={(e) => setType(e.target.value)} className={selectCls}>
              <option value="">Tous les types</option>
              {filters?.types.map((t) => (
                <option key={t.code} value={t.code}>
                  {t.label} ({t.n})
                </option>
              ))}
            </select>
            <select value={service} onChange={(e) => setService(e.target.value)} className={selectCls}>
              <option value="">Tous les services</option>
              {filters?.services.map((s) => (
                <option key={s.code} value={s.code}>
                  {s.label}
                </option>
              ))}
            </select>
            <select
              value={region}
              onChange={(e) => {
                setRegion(e.target.value);
                setDistrict('');
              }}
              className={selectCls}
            >
              <option value="">Toutes les régions</option>
              {filters?.regions.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
            <select value={district} onChange={(e) => setDistrict(e.target.value)} className={selectCls}>
              <option value="">Tous les districts</option>
              {districtsOfRegion.map((d) => (
                <option key={d.name} value={d.name}>
                  {d.name}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={me ? () => setMe(null) : locateMe}
              disabled={locating}
              className={`flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-md border ${
                me ? 'bg-emerald-600 border-emerald-600 text-white' : 'border-gray-300 text-gray-700 hover:bg-gray-50'
              }`}
            >
              {locating ? <Loader2 size={14} className="animate-spin" /> : <LocateFixed size={14} />}
              {me ? `Autour de moi (${NEAR_RADIUS_KM} km)` : 'Autour de moi'}
            </button>
            {hasFilter && (
              <button onClick={clearAll} className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-800">
                <X size={12} /> Effacer
              </button>
            )}
            <span className="ml-auto text-xs text-gray-500">
              {searching ? '…' : `${results.length}${results.length >= 100 ? '+' : ''} résultat${results.length > 1 ? 's' : ''}`}
            </span>
          </div>
        </div>

        {error && <div className="px-3 py-2 text-xs text-red-700 bg-red-50 border-b border-red-100">{error}</div>}

        <ul className="flex-1 overflow-auto divide-y divide-gray-100">
          {results.map((r) => (
            <li key={r.uid}>
              <button
                onClick={() => selectResult(r)}
                className={`w-full text-left px-3 py-2 hover:bg-emerald-50 ${selected === r.uid ? 'bg-emerald-50' : ''}`}
              >
                <div className="flex items-start gap-2">
                  <span className="mt-1.5 w-2.5 h-2.5 rounded-full shrink-0" style={{ background: typeColor(r.type) }} />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-medium text-gray-900 truncate">{r.name}</div>
                    <div className="text-xs text-gray-500 truncate">
                      {r.type_label} · {r.district}
                    </div>
                  </div>
                  <div className="text-right shrink-0">
                    {r.distance_km !== undefined && (
                      <div className="text-xs font-medium text-gray-700">{r.distance_km.toFixed(1)} km</div>
                    )}
                    {r.lat === null && <div className="text-[10px] text-gray-400">non géolocalisée</div>}
                  </div>
                </div>
                <div className="mt-1 flex items-center gap-2">
                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${opColor(r.op)}`}>{opLabel(r.op)}</span>
                  <Link to={`/fs/${r.uid}`} onClick={(e) => e.stopPropagation()} className="text-[11px] text-emerald-700 hover:underline">
                    Fiche
                  </Link>
                </div>
              </button>
            </li>
          ))}
          {!searching && results.length === 0 && (
            <li className="px-3 py-6 text-sm text-gray-500 text-center">Aucune structure ne correspond.</li>
          )}
        </ul>

        {summary && (
          <div className="px-3 py-2 text-[11px] text-gray-500 border-t border-gray-100">
            {summary.n_structures.toLocaleString('fr-FR')} structures recensées · {summary.pct_gps}% géolocalisées ·{' '}
            {summary.derniere_synchro ? `données du ${new Date(summary.derniere_synchro).toLocaleDateString('fr-FR')}` : ''}
          </div>
        )}
      </aside>

      {/* Carte */}
      <div className="flex-1 relative min-h-[45vh]">
        <MapContainer center={GUINEA_CENTER} zoom={7} className="absolute inset-0" preferCanvas>
          <InvalidateOnResize />
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          {points && <PointsCanvasLayer points={markers} cluster={cluster} focus={focus} onMarkerClick={onMarkerClick} />}
          {me && <CircleMarker center={me} radius={9} pathOptions={{ color: '#1d4ed8', fillColor: '#3b82f6', fillOpacity: 0.6 }} />}
          <FlyTo target={flyTarget} />
        </MapContainer>

        {!points && !error && (
          <div className="absolute inset-0 flex items-center justify-center bg-white/60 z-[500]">
            <Loader2 className="animate-spin text-emerald-600" />
          </div>
        )}

        <div className="absolute bottom-6 left-3 z-[500] bg-white/95 rounded-md shadow px-3 py-2 text-[11px] space-y-1">
          <div className="font-medium text-gray-700 mb-1">
            {visibleFeatures.length.toLocaleString('fr-FR')} structures sur la carte
          </div>
          <label className="flex items-center gap-1.5 text-gray-600 cursor-pointer mb-1">
            <input
              type="checkbox"
              checked={cluster}
              onChange={(e) => setClusterParam(e.target.checked ? 'on' : '')}
              className="accent-emerald-600"
            />
            Regrouper les points
          </label>
          {[
            ['Hôpitaux', 'HP'],
            ['CMC / CSA', 'CMC'],
            ['Centres de santé', 'CS'],
            ['Postes de santé', 'PS'],
            ['Privé', 'CLINIQUE'],
          ].map(([label, code]) => (
            <div key={code} className="flex items-center gap-1.5 text-gray-600">
              <span className="w-2.5 h-2.5 rounded-full" style={{ background: typeColor(code) }} />
              {label}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
