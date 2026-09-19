import { useRef, type ReactNode } from 'react';
import { MapContainer } from 'react-leaflet';

interface Props {
  /** Change de clé pour forcer le remontage quand le style des couches change. */
  mapKey: string;
  children: ReactNode;
}

// Mini-carte zoomée sur Conakry (5 communes trop petites pour être lisibles
// à l'échelle nationale), déplaçable par sa barre de titre. Les couches sont
// passées en enfants : même GeoJSON/étiquettes que la carte principale.
export default function ConakryInset({ mapKey, children }: Props) {
  const insetRef = useRef<HTMLDivElement | null>(null);
  const dragState = useRef({ dragging: false, offsetX: 0, offsetY: 0 });

  return (
    <div
      ref={insetRef}
      className="absolute z-[1000] rounded-lg overflow-hidden border-2 border-gray-400 shadow-lg hidden sm:block"
      style={{ width: '300px', height: '250px', bottom: '12px', left: '12px', cursor: 'move' }}
      onMouseDown={(e) => {
        const el = insetRef.current;
        if (!el) return;
        // Only drag from the title bar area (first 22px)
        const rect = el.getBoundingClientRect();
        if (e.clientY - rect.top > 22) return;
        e.preventDefault();
        dragState.current = { dragging: true, offsetX: e.clientX - rect.left, offsetY: e.clientY - rect.top };
        const onMove = (ev: MouseEvent) => {
          if (!dragState.current.dragging || !el.parentElement) return;
          const parent = el.parentElement.getBoundingClientRect();
          let x = ev.clientX - parent.left - dragState.current.offsetX;
          let y = ev.clientY - parent.top - dragState.current.offsetY;
          x = Math.max(0, Math.min(x, parent.width - el.offsetWidth));
          y = Math.max(0, Math.min(y, parent.height - el.offsetHeight));
          el.style.left = `${x}px`;
          el.style.top = `${y}px`;
          el.style.bottom = 'auto';
          el.style.right = 'auto';
        };
        const onUp = () => {
          dragState.current.dragging = false;
          document.removeEventListener('mousemove', onMove);
          document.removeEventListener('mouseup', onUp);
        };
        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
      }}
    >
      <div className="bg-gray-700 text-white text-[10px] font-semibold px-2 py-0.5 text-center select-none" style={{ cursor: 'grab' }}>
        Conakry
      </div>
      <MapContainer
        key={`inset-${mapKey}`}
        center={[9.6, -13.58]}
        zoom={10}
        style={{ height: 'calc(100% - 20px)', width: '100%', background: '#ffffff' }}
        scrollWheelZoom={false}
        dragging={false}
        zoomControl={false}
        doubleClickZoom={false}
        attributionControl={false}
      >
        {children}
      </MapContainer>
    </div>
  );
}

const CONAKRY_NAMES = ['dixinn', 'kaloum', 'matam', 'matoto', 'ratoma'];

/** Vrai si un nom d'unité (district ou son district parent) désigne une commune de Conakry. */
export function isConakry(...names: (string | undefined)[]): boolean {
  return names.some((n) => n && CONAKRY_NAMES.some((c) => n.toLowerCase().includes(c)));
}
