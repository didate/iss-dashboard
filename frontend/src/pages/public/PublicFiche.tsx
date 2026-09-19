import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { MapContainer, TileLayer, CircleMarker } from 'react-leaflet';
import { QRCodeSVG } from 'qrcode.react';
import { ArrowLeft, Check, Copy, ExternalLink, MapPin, FlaskConical, Baby, ScanLine, Siren, Pill, Scissors, Loader2 } from 'lucide-react';
import { publicApi, typeColor, opLabel, opColor } from '../../api/public';
import InvalidateOnResize from '../../components/map/InvalidateOnResize';
import type { PublicStructure } from '../../types';

const PLATEAU: { key: string; label: string; icon: typeof FlaskConical }[] = [
  { key: 'labo', label: 'Laboratoire', icon: FlaskConical },
  { key: 'maternite', label: 'Maternité', icon: Baby },
  { key: 'imagerie', label: 'Imagerie', icon: ScanLine },
  { key: 'urgences', label: 'Urgences', icon: Siren },
  { key: 'pharmacie', label: 'Pharmacie', icon: Pill },
  { key: 'chirurgie', label: 'Chirurgie', icon: Scissors },
];

function statutLabel(s: PublicStructure): string {
  const base = s.statut === 'privée' ? 'Privée' : s.statut === 'publique' ? 'Publique' : '';
  const detail = s.statut_detail && s.statut_detail !== s.statut ? ` · ${s.statut_detail}` : '';
  return base ? base + detail : 'Statut juridique non renseigné';
}

export default function PublicFiche() {
  const { uid = '' } = useParams();
  const [data, setData] = useState<PublicStructure | null>(null);
  const [status, setStatus] = useState<'loading' | 'ok' | 'notfound' | 'error'>('loading');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setStatus('loading');
    publicApi
      .getStructure(uid)
      .then((d) => {
        setData(d);
        setStatus('ok');
      })
      .catch((e: Error) => setStatus(e.message.startsWith('API 404') ? 'notfound' : 'error'));
  }, [uid]);

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* presse-papier indisponible : le QR et l'URL restent visibles */
    }
  };

  if (status === 'loading') {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="animate-spin text-emerald-600" />
      </div>
    );
  }
  if (status !== 'ok' || !data) {
    return (
      <div className="max-w-2xl mx-auto p-6 text-center">
        <p className="text-gray-700 mb-4">
          {status === 'notfound' ? 'Cette structure est inconnue ou n’a pas été recensée.' : 'Impossible de charger la fiche pour le moment.'}
        </p>
        <Link to="/" className="text-emerald-700 hover:underline">
          ← Retour à la carte
        </Link>
      </div>
    );
  }

  const hasPos = data.lat !== null && data.lng !== null;
  const osmUrl = hasPos ? `https://www.openstreetmap.org/?mlat=${data.lat}&mlon=${data.lng}#map=16/${data.lat}/${data.lng}` : '';
  const where = [data.sous_prefecture, data.district, data.region].filter(Boolean).join(' · ');

  return (
    <div className="max-w-4xl mx-auto p-4 sm:p-6">
      <Link to="/" className="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 mb-4">
        <ArrowLeft size={14} /> Carte
      </Link>

      <div className="bg-white border border-gray-200 rounded-xl overflow-hidden">
        {/* En-tête */}
        <div className="p-5 border-b border-gray-100">
          <div className="flex flex-wrap items-start gap-3">
            <span className="mt-1 w-3 h-3 rounded-full shrink-0" style={{ background: typeColor(data.type) }} />
            <div className="min-w-0 flex-1">
              <h1 className="text-xl font-semibold text-gray-900">{data.name}</h1>
              <p className="text-sm text-gray-600">
                {data.type_label} · {statutLabel(data)}
              </p>
              <p className="text-sm text-gray-500 flex items-center gap-1 mt-1">
                <MapPin size={13} /> {where || 'Rattachement non renseigné'}
              </p>
            </div>
            <span className={`text-xs px-2 py-1 rounded-md ${opColor(data.op)}`}>{opLabel(data.op)}</span>
          </div>
        </div>

        <div className="grid md:grid-cols-5">
          {/* Carte */}
          <div className="md:col-span-3 h-64 md:h-80 relative bg-gray-100">
            {hasPos ? (
              <MapContainer center={[data.lat!, data.lng!]} zoom={14} className="absolute inset-0" scrollWheelZoom={false}>
                <InvalidateOnResize />
                <TileLayer
                  attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
                  url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                />
                <CircleMarker
                  center={[data.lat!, data.lng!]}
                  radius={9}
                  pathOptions={{ color: '#fff', weight: 2, fillColor: typeColor(data.type), fillOpacity: 1 }}
                />
              </MapContainer>
            ) : (
              <div className="absolute inset-0 flex items-center justify-center text-sm text-gray-500 px-6 text-center">
                Cette structure n’est pas encore géolocalisée.
              </div>
            )}
          </div>

          {/* Partage */}
          <div className="md:col-span-2 p-5 flex flex-col items-center justify-center gap-3 border-t md:border-t-0 md:border-l border-gray-100">
            <QRCodeSVG value={window.location.href} size={128} level="M" />
            <p className="text-[11px] text-gray-500 text-center">Scannez pour ouvrir cette fiche sur un téléphone</p>
            <button
              onClick={copyLink}
              className="inline-flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50"
            >
              {copied ? <Check size={14} className="text-emerald-600" /> : <Copy size={14} />}
              {copied ? 'Lien copié' : 'Copier le lien'}
            </button>
            {hasPos && (
              <a
                href={osmUrl}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 text-xs text-emerald-700 hover:underline"
              >
                <ExternalLink size={12} /> Itinéraire / OpenStreetMap
              </a>
            )}
          </div>
        </div>

        {/* Ressources humaines et accès (agrégats) */}
        <div className="p-5 border-t border-gray-100 grid sm:grid-cols-2 gap-6">
          <div>
            <h2 className="text-sm font-semibold text-gray-800 mb-2">Ressources humaines</h2>
            <dl className="text-sm divide-y divide-gray-100">
              {[
                ['Total RH', data.rh_total],
                ['Médecins', data.rh_medecins],
                ['Personnel soignant', data.rh_soignants],
              ].map(([l, v]) => (
                <div key={String(l)} className="flex justify-between py-1.5">
                  <dt className="text-gray-600">{l}</dt>
                  <dd className="font-semibold text-gray-900">{v === null || v === undefined ? '—' : String(v)}</dd>
                </div>
              ))}
            </dl>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-gray-800 mb-2">Services & accès</h2>
            <dl className="text-sm divide-y divide-gray-100">
              {[
                ['Eau aux points critiques', data.eau],
                ["Source d'énergie", data.energie],
              ].map(([l, v]) => (
                <div key={String(l)} className="flex justify-between py-1.5">
                  <dt className="text-gray-600">{l}</dt>
                  <dd className={`font-bold ${v === null ? 'text-gray-400' : v ? 'text-green-600' : 'text-red-600'}`}>{v === null || v === undefined ? '—' : v ? '✓' : '✗'}</dd>
                </div>
              ))}
              <div className="py-1.5">
                <div className="flex justify-between">
                  <dt className="text-gray-600">Score disponibilité services</dt>
                  <dd className="font-semibold text-gray-900">{data.score_services === null ? '—' : `${data.score_services} / ${data.score_services_max}`}</dd>
                </div>
                {data.score_services !== null && (
                  <div className="mt-1 h-1.5 rounded bg-gray-200">
                    <div
                      className={`h-1.5 rounded ${data.score_services >= data.score_services_max * 0.7 ? 'bg-green-500' : data.score_services >= data.score_services_max * 0.4 ? 'bg-amber-500' : 'bg-red-500'}`}
                      style={{ width: `${Math.round((100 * data.score_services) / Math.max(1, data.score_services_max))}%` }}
                    />
                  </div>
                )}
                <p className="text-[11px] text-gray-400 mt-1">Services principaux : curatif, CPN, accouchement, PEV, PTME, laboratoire, pharmacie.</p>
              </div>
            </dl>
          </div>
        </div>

        {/* Plateau technique */}
        <div className="p-5 border-t border-gray-100">
          <h2 className="text-sm font-semibold text-gray-800 mb-3">Plateau technique</h2>
          <div className="grid grid-cols-3 sm:grid-cols-6 gap-2">
            {PLATEAU.map(({ key, label, icon: Icon }) => {
              const known = key in data.plateau;
              const ok = data.plateau[key] === true;
              return (
                <div
                  key={key}
                  className={`flex flex-col items-center gap-1 rounded-lg border p-2 text-xs ${
                    ok ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : known ? 'border-gray-200 text-gray-400' : 'border-dashed border-gray-200 text-gray-300'
                  }`}
                  title={ok ? 'Fonctionnel' : known ? 'Non disponible' : 'Non renseigné'}
                >
                  <Icon size={18} />
                  {label}
                </div>
              );
            })}
          </div>
        </div>

        {/* Services */}
        <div className="p-5 border-t border-gray-100">
          <h2 className="text-sm font-semibold text-gray-800 mb-3">
            Services fonctionnels <span className="text-gray-400 font-normal">({data.services.length})</span>
          </h2>
          {data.services.length === 0 ? (
            <p className="text-sm text-gray-500">Aucun service déclaré fonctionnel lors du recensement.</p>
          ) : (
            <ul className="flex flex-wrap gap-1.5">
              {data.services.map((s) => (
                <li key={s.code} className="text-xs px-2 py-1 rounded-md bg-gray-100 text-gray-700">
                  {s.label}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="px-5 py-3 border-t border-gray-100 text-[11px] text-gray-400">
          Source : recensement ISS (DHIS2, Ministère de la Santé){data.recense_le ? `, données du ${data.recense_le}` : ''}. Identifiant : {data.uid}
        </div>
      </div>
    </div>
  );
}
