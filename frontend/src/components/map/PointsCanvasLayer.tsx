import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';

/** Un point à dessiner : position, couleur et contenu HTML (déjà échappé) de la popup. */
export interface MarkerSpec {
  uid: string;
  lat: number;
  lng: number;
  color: string;
  /** HTML de la popup, ou fonction appelée au clic. */
  popup: string | (() => string);
}

interface Props {
  points: MarkerSpec[];
  /** Regrouper les points proches (grille de ~60 px) ; sinon tous les points sont dessinés. */
  cluster: boolean;
  /** Point à mettre en avant depuis une liste : recentrage (sans zoom) + popup. */
  focus?: { uid: string; nonce: number } | null;
  onMarkerClick?: (uid: string) => void;
}

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string);
}

const CELL = 60; // taille de la grille de regroupement, en pixels écran
const POINT_R = 6;

type Drawn =
  | { kind: 'point'; x: number; y: number; r: number; p: MarkerSpec }
  | { kind: 'cluster'; x: number; y: number; r: number; count: number; lat: number; lng: number; members: MarkerSpec[] };

function clusterColor(n: number): { fill: string; ring: string } {
  if (n < 10) return { fill: 'rgba(110, 204, 57, 0.85)', ring: 'rgba(181, 226, 140, 0.6)' };
  if (n < 100) return { fill: 'rgba(240, 194, 12, 0.85)', ring: 'rgba(241, 211, 87, 0.6)' };
  return { fill: 'rgba(241, 128, 23, 0.85)', ring: 'rgba(253, 156, 115, 0.6)' };
}

// Couche de points sur un seul <canvas> : pas d'objet Leaflet par structure,
// pas de plugin. Tout est redessiné à chaque déplacement (2 600 cercles ≈ 2 ms),
// le regroupement se calcule par grille au moment du dessin. Le clic cherche
// l'élément dessiné sous le curseur. Choisi après constat que le rendu par
// marqueurs individuels (leaflet.markercluster + CircleMarker) mettait Firefox
// et Safari à genoux.
export default function PointsCanvasLayer({ points, cluster, focus, onMarkerClick }: Props) {
  const map = useMap();
  const drawnRef = useRef<Drawn[]>([]);
  const popupRef = useRef<L.Popup | null>(null);

  useEffect(() => {
    const canvas = L.DomUtil.create('canvas', 'leaflet-zoom-hide') as HTMLCanvasElement;
    canvas.style.position = 'absolute';
    canvas.style.pointerEvents = 'none';
    const pane = map.getPanes().overlayPane;
    pane.appendChild(canvas);
    const dpr = window.devicePixelRatio || 1;

    const draw = () => {
      const size = map.getSize();
      canvas.width = size.x * dpr;
      canvas.height = size.y * dpr;
      canvas.style.width = `${size.x}px`;
      canvas.style.height = `${size.y}px`;
      L.DomUtil.setPosition(canvas, map.containerPointToLayerPoint([0, 0]));
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, size.x, size.y);

      const drawn: Drawn[] = [];
      const bounds = map.getBounds().pad(0.1);

      if (cluster) {
        const cells = new Map<string, { sx: number; sy: number; sLat: number; sLng: number; members: MarkerSpec[] }>();
        for (const p of points) {
          if (!bounds.contains([p.lat, p.lng])) continue;
          const c = map.latLngToContainerPoint([p.lat, p.lng]);
          const key = `${Math.floor(c.x / CELL)}|${Math.floor(c.y / CELL)}`;
          let cell = cells.get(key);
          if (!cell) {
            cell = { sx: 0, sy: 0, sLat: 0, sLng: 0, members: [] };
            cells.set(key, cell);
          }
          cell.sx += c.x;
          cell.sy += c.y;
          cell.sLat += p.lat;
          cell.sLng += p.lng;
          cell.members.push(p);
        }
        for (const cell of cells.values()) {
          const n = cell.members.length;
          if (n === 1) {
            const p = cell.members[0];
            const c = map.latLngToContainerPoint([p.lat, p.lng]);
            drawPoint(ctx, c.x, c.y, p.color);
            drawn.push({ kind: 'point', x: c.x, y: c.y, r: POINT_R, p });
          } else {
            const x = cell.sx / n, y = cell.sy / n;
            const r = n < 10 ? 14 : n < 100 ? 17 : 20;
            const col = clusterColor(n);
            ctx.beginPath();
            ctx.arc(x, y, r + 4, 0, Math.PI * 2);
            ctx.fillStyle = col.ring;
            ctx.fill();
            ctx.beginPath();
            ctx.arc(x, y, r, 0, Math.PI * 2);
            ctx.fillStyle = col.fill;
            ctx.fill();
            ctx.fillStyle = '#1f2937';
            ctx.font = 'bold 12px system-ui, sans-serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(String(n), x, y);
            drawn.push({ kind: 'cluster', x, y, r: r + 4, count: n, lat: cell.sLat / n, lng: cell.sLng / n, members: cell.members });
          }
        }
      } else {
        for (const p of points) {
          if (!bounds.contains([p.lat, p.lng])) continue;
          const c = map.latLngToContainerPoint([p.lat, p.lng]);
          drawPoint(ctx, c.x, c.y, p.color);
          drawn.push({ kind: 'point', x: c.x, y: c.y, r: POINT_R, p });
        }
      }
      drawnRef.current = drawn;
    };

    const drawPoint = (ctx: CanvasRenderingContext2D, x: number, y: number, color: string) => {
      ctx.beginPath();
      ctx.arc(x, y, POINT_R, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.fill();
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = '#ffffff';
      ctx.stroke();
    };

    const onClick = (e: L.LeafletMouseEvent) => {
      const c = e.containerPoint;
      let best: Drawn | null = null;
      let bestD = Infinity;
      for (const d of drawnRef.current) {
        const dist = Math.hypot(d.x - c.x, d.y - c.y);
        if (dist <= d.r + 3 && dist < bestD) {
          best = d;
          bestD = dist;
        }
      }
      if (!best) return;
      if (best.kind === 'cluster') {
        // Zoom sur l'emprise des membres (comme un cluster classique), sans dépasser le zoom max utile.
        const b = L.latLngBounds(best.members.map((m) => [m.lat, m.lng] as [number, number]));
        const target = Math.min(map.getBoundsZoom(b.pad(0.2)), 16);
        map.setView([best.lat, best.lng], Math.max(target, map.getZoom() + 1));
        return;
      }
      openPopup(best.p);
      onMarkerClick?.(best.p.uid);
    };

    const openPopup = (p: MarkerSpec) => {
      const html = typeof p.popup === 'function' ? p.popup() : p.popup;
      popupRef.current?.remove();
      popupRef.current = L.popup({ closeButton: true }).setLatLng([p.lat, p.lng]).setContent(html).openOn(map);
    };

    // Curseur main sur un élément dessiné (sans hit-test coûteux : quelques dizaines d'items visibles au plus)
    const onMouseMove = (e: L.LeafletMouseEvent) => {
      const c = e.containerPoint;
      const hit = drawnRef.current.some((d) => Math.hypot(d.x - c.x, d.y - c.y) <= d.r + 3);
      map.getContainer().style.cursor = hit ? 'pointer' : '';
    };

    map.on('moveend zoomend resize viewreset', draw);
    map.on('move', draw);
    map.on('click', onClick);
    map.on('mousemove', onMouseMove);
    draw();

    // Exposé pour l'effet « focus » ci-dessous
    (canvas as HTMLCanvasElement & { __openPopup?: (p: MarkerSpec) => void }).__openPopup = openPopup;
    canvasRef.current = canvas;

    return () => {
      map.off('moveend zoomend resize viewreset', draw);
      map.off('move', draw);
      map.off('click', onClick);
      map.off('mousemove', onMouseMove);
      map.getContainer().style.cursor = '';
      popupRef.current?.remove();
      popupRef.current = null;
      pane.removeChild(canvas);
      canvasRef.current = null;
      drawnRef.current = [];
    };
  }, [map, points, cluster, onMarkerClick]);

  const canvasRef = useRef<(HTMLCanvasElement & { __openPopup?: (p: MarkerSpec) => void }) | null>(null);

  useEffect(() => {
    if (!focus) return;
    const p = points.find((x) => x.uid === focus.uid);
    if (!p) return;
    // On recentre sans changer le zoom : l'utilisateur garde son niveau de lecture,
    // la popup s'ouvre sur la position de la structure (même si elle est dans un regroupement).
    const ll = L.latLng(p.lat, p.lng);
    const open = () => canvasRef.current?.__openPopup?.(p);
    if (map.getCenter().distanceTo(ll) < 1) {
      open();
    } else {
      map.once('moveend', open);
      map.panTo(ll);
    }
  }, [focus, map, points]);

  return null;
}
