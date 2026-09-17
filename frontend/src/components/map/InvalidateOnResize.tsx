import { useEffect } from 'react';
import { useMap } from 'react-leaflet';

// Leaflet ne mesure son conteneur qu'à la création et sur `resize` de la
// fenêtre. Quand le conteneur change de taille autrement (panneau replié,
// rotation mobile, onglet affiché après coup), on lui demande de se remesurer.
// Si la carte a été créée dans un conteneur de taille nulle (onglet caché), les
// tracés vectoriels ont été projetés sur rien : `viewreset` les redessine.
export default function InvalidateOnResize() {
  const map = useMap();
  useEffect(() => {
    if (typeof ResizeObserver === 'undefined') return;
    let wasEmpty = map.getSize().x === 0 || map.getSize().y === 0;
    const ro = new ResizeObserver(() => {
      map.invalidateSize({ animate: false });
      const size = map.getSize();
      const empty = size.x === 0 || size.y === 0;
      if (wasEmpty && !empty) map.fire('viewreset');
      wasEmpty = empty;
    });
    ro.observe(map.getContainer());
    return () => ro.disconnect();
  }, [map]);
  return null;
}
