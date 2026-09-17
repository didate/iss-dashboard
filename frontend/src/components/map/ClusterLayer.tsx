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
  popup: string;
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

// Dessine des cercles colorés, regroupés en clusters (leaflet.markercluster) ou
// non. Pur affichage : les données arrivent déjà filtrées et colorées.
export default function ClusterLayer({ points, cluster, focus, onMarkerClick }: Props) {
  const map = useMap();
  const groupRef = useRef<L.MarkerClusterGroup | L.LayerGroup | null>(null);
  const markersRef = useRef<Map<string, L.CircleMarker>>(new Map());

  useEffect(() => {
    // Pas de chargement par tranches : ~3 000 cercles s'ajoutent en quelques
    // dizaines de ms, et le mode chunked plante si le groupe est retiré de la
    // carte avant la fin (double montage StrictMode, changement de filtre rapide).
    const group: L.MarkerClusterGroup | L.LayerGroup = cluster
      ? L.markerClusterGroup({
          chunkedLoading: false,
          maxClusterRadius: 45,
          spiderfyOnMaxZoom: true,
          showCoverageOnHover: false,
        })
      : L.layerGroup();
    const markers = new Map<string, L.CircleMarker>();
    for (const p of points) {
      const m = L.circleMarker([p.lat, p.lng], {
        radius: 7,
        color: '#ffffff',
        weight: 1.5,
        fillColor: p.color,
        fillOpacity: 0.9,
      });
      m.bindPopup(p.popup, { closeButton: true });
      m.on('click', () => onMarkerClick?.(p.uid));
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
  }, [map, points, cluster, onMarkerClick]);

  useEffect(() => {
    if (!focus) return;
    const marker = markersRef.current.get(focus.uid);
    const group = groupRef.current;
    if (!marker || !group) return;
    const reveal = () => {
      const ll = marker.getLatLng();
      if (map.getZoom() < 13) map.setView(ll, 13);
      else map.panTo(ll);
      marker.openPopup();
    };
    if ('zoomToShowLayer' in group) group.zoomToShowLayer(marker, reveal);
    else reveal();
  }, [focus, map, points, cluster]);

  return null;
}
