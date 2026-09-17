import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet.markercluster';
import 'leaflet.markercluster/dist/MarkerCluster.css';
import 'leaflet.markercluster/dist/MarkerCluster.Default.css';
import type { PublicPointFeature } from '../../types';
import { typeColor, opLabel } from '../../api/public';

interface Props {
  features: PublicPointFeature[];
  selectedUid?: string | null;
  onSelect?: (uid: string) => void;
}

const FICHE_BASE = `${import.meta.env.BASE_URL.replace(/\/$/, '')}/fs/`;

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);
}

function popupHtml(p: PublicPointFeature['properties']): string {
  const where = [p.sp, p.district].filter(Boolean).join(' · ');
  return `
    <div style="min-width:180px">
      <div style="font-weight:600;font-size:14px;margin-bottom:2px">${escapeHtml(p.name)}</div>
      <div style="font-size:12px;color:#374151">
        <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${typeColor(p.type)};margin-right:4px"></span>${escapeHtml(p.type_label)}
      </div>
      <div style="font-size:12px;color:#6b7280">${escapeHtml(where)}</div>
      <div style="font-size:12px;color:#6b7280;margin-bottom:6px">${escapeHtml(opLabel(p.op))}</div>
      <a href="${FICHE_BASE}${encodeURIComponent(p.uid)}" style="font-size:12px;color:#047857;font-weight:500">Voir la fiche →</a>
    </div>`;
}

// Regroupe les points en clusters (leaflet.markercluster) ; les marqueurs sont
// des cercles colorés par type. Pur affichage : les données arrivent filtrées.
export default function ClusterLayer({ features, selectedUid, onSelect }: Props) {
  const map = useMap();
  const groupRef = useRef<L.MarkerClusterGroup | null>(null);
  const markersRef = useRef<Map<string, L.CircleMarker>>(new Map());

  useEffect(() => {
    // Pas de chargement par tranches : ~3 000 cercles s'ajoutent en quelques
    // dizaines de ms, et le mode chunked plante si le groupe est retiré de la
    // carte avant la fin (double montage StrictMode, changement de filtre rapide).
    const group = L.markerClusterGroup({
      chunkedLoading: false,
      maxClusterRadius: 45,
      spiderfyOnMaxZoom: true,
      showCoverageOnHover: false,
    });
    const markers = new Map<string, L.CircleMarker>();
    for (const f of features) {
      const [lng, lat] = f.geometry.coordinates;
      const p = f.properties;
      const m = L.circleMarker([lat, lng], {
        radius: 7,
        color: '#ffffff',
        weight: 1.5,
        fillColor: typeColor(p.type),
        fillOpacity: 0.9,
      });
      m.bindPopup(popupHtml(p), { closeButton: true });
      m.on('click', () => onSelect?.(p.uid));
      markers.set(p.uid, m);
      group.addLayer(m);
    }
    map.addLayer(group);
    groupRef.current = group;
    markersRef.current = markers;
    return () => {
      group.clearLayers();
      map.removeLayer(group);
      groupRef.current = null;
      markersRef.current = new Map();
    };
  }, [map, features, onSelect]);

  useEffect(() => {
    if (!selectedUid) return;
    const marker = markersRef.current.get(selectedUid);
    const group = groupRef.current;
    if (!marker || !group) return;
    group.zoomToShowLayer(marker, () => {
      const ll = marker.getLatLng();
      if (map.getZoom() < 13) map.setView(ll, 13);
      marker.openPopup();
    });
  }, [selectedUid, map, features]);

  return null;
}
