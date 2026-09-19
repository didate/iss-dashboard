import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet.markercluster';
import 'leaflet.markercluster/dist/MarkerCluster.css';
import 'leaflet.markercluster/dist/MarkerCluster.Default.css';

/** Un point à dessiner : position, couleur et contenu HTML (déjà échappé) de la popup. */
export interface MarkerSpec {
  uid: string;
  lat: number;
  lng: number;
  color: string;
  /** HTML de la popup, ou fonction appelée au premier clic (évite de construire 3 000 chaînes d'avance). */
  popup: string | (() => string);
}

interface Props {
  points: MarkerSpec[];
  /** Regrouper les points en clusters (sinon tous les cercles sont dessinés). */
  cluster: boolean;
  /**
   * Point à mettre en avant depuis une liste : la carte se déplace jusqu'à lui
   * et ouvre sa popup. Un clic direct sur un marqueur ne passe pas par ici —
   * il ouvre seulement la popup, sans zoom.
   */
  focus?: { uid: string; nonce: number } | null;
  onMarkerClick?: (uid: string) => void;
}

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);
}

// Taille des lots : ~400 cercles ≈ 30-60 ms, assez court pour laisser respirer l'interface.
const BATCH = 400;

// Dessine des cercles colorés, regroupés en clusters (leaflet.markercluster) ou
// non. Pur affichage : les données arrivent déjà filtrées et colorées.
//
// Les marqueurs sont ajoutés par lots (setTimeout) pour ne pas bloquer le
// navigateur, et la popup n'est construite qu'au premier clic. Le nettoyage
// annule les lots restants : le groupe n'est jamais alimenté après son retrait
// (c'est le scénario qui faisait planter le chunkedLoading natif du plugin).
export default function ClusterLayer({ points, cluster, focus, onMarkerClick }: Props) {
  const map = useMap();
  const groupRef = useRef<L.MarkerClusterGroup | L.LayerGroup | null>(null);
  const markersRef = useRef<Map<string, L.CircleMarker>>(new Map());

  useEffect(() => {
    const group: L.MarkerClusterGroup | L.LayerGroup = cluster
      ? L.markerClusterGroup({
          chunkedLoading: false,
          maxClusterRadius: 45,
          spiderfyOnMaxZoom: true,
          showCoverageOnHover: false,
          removeOutsideVisibleBounds: true,
        })
      : L.layerGroup();
    const markers = new Map<string, L.CircleMarker>();
    map.addLayer(group);
    groupRef.current = group;
    markersRef.current = markers;

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let offset = 0;

    const addBatch = () => {
      if (cancelled) return;
      const batch: L.CircleMarker[] = [];
      const end = Math.min(offset + BATCH, points.length);
      for (; offset < end; offset++) {
        const p = points[offset];
        const m = L.circleMarker([p.lat, p.lng], {
          radius: 7,
          color: '#ffffff',
          weight: 1.5,
          fillColor: p.color,
          fillOpacity: 0.9,
        });
        m.on('click', () => {
          if (!m.getPopup()) {
            m.bindPopup(typeof p.popup === 'function' ? p.popup() : p.popup, { closeButton: true });
            m.openPopup();
          }
          onMarkerClick?.(p.uid);
        });
        markers.set(p.uid, m);
        batch.push(m);
      }
      if (cluster) (group as L.MarkerClusterGroup).addLayers(batch);
      else batch.forEach((m) => group.addLayer(m));
      if (offset < points.length) timer = setTimeout(addBatch, 0);
    };
    addBatch();

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      group.clearLayers();
      map.removeLayer(group);
      groupRef.current = null;
      markersRef.current = new Map();
    };
  }, [map, points, cluster, onMarkerClick]);

  useEffect(() => {
    if (!focus) return;
    const marker = markersRef.current.get(focus.uid);
    const group = groupRef.current;
    if (!marker || !group) return;
    const spec = points.find((p) => p.uid === focus.uid);
    const reveal = () => {
      const ll = marker.getLatLng();
      if (map.getZoom() < 13) map.setView(ll, 13);
      else map.panTo(ll);
      if (!marker.getPopup() && spec) marker.bindPopup(typeof spec.popup === 'function' ? spec.popup() : spec.popup, { closeButton: true });
      marker.openPopup();
    };
    if ('zoomToShowLayer' in group) group.zoomToShowLayer(marker, reveal);
    else reveal();
  }, [focus, map, points, cluster]);

  return null;
}
