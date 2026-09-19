import { useEffect } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';
import type { MapGeoFeature } from '../../types';

interface Props {
  features: MapGeoFeature[];
  /** Étiquette affichée sous le nom (valeur de la métrique). */
  text: (p: MapGeoFeature['properties']) => string;
  /** En dessous de ce zoom, les étiquettes sont masquées (sous-préfectures : trop denses au niveau national). */
  minZoom?: number;
  /** Police réduite (mini-carte). */
  small?: boolean;
}

// Nom + valeur au centre de chaque polygone, sans interaction (les clics
// traversent vers le polygone). Même rendu que les étiquettes de la carte
// thématique par district.
export default function GeoLabels({ features, text, minZoom = 0, small = false }: Props) {
  const map = useMap();

  useEffect(() => {
    const group = L.layerGroup();
    for (const f of features) {
      try {
        const center = L.geoJSON(f as unknown as GeoJSON.Feature).getBounds().getCenter();
        const icon = L.divIcon({
          className: 'district-label',
          html: `<div style="text-align:center;pointer-events:none;text-shadow:1px 1px 2px white,-1px -1px 2px white,1px -1px 2px white,-1px 1px 2px white;">
            <div style="font-size:${small ? 9 : 10}px;font-weight:700;color:#1f2937;line-height:1.2;">${f.properties.name}</div>
            <div style="font-size:${small ? 10 : 11}px;font-weight:800;color:#1e3a5f;line-height:1.2;">${text(f.properties)}</div>
          </div>`,
          iconSize: [small ? 70 : 90, small ? 26 : 30],
          iconAnchor: [small ? 35 : 45, small ? 13 : 15],
        });
        L.marker(center, { icon, interactive: false }).addTo(group);
      } catch {
        // géométrie invalide : pas d'étiquette
      }
    }

    const sync = () => {
      const show = map.getZoom() >= minZoom;
      if (show && !map.hasLayer(group)) map.addLayer(group);
      if (!show && map.hasLayer(group)) map.removeLayer(group);
    };
    sync();
    map.on('zoomend', sync);
    return () => {
      map.off('zoomend', sync);
      if (map.hasLayer(group)) map.removeLayer(group);
    };
  }, [map, features, text, minZoom, small]);

  return null;
}
