import { useState } from 'react';
import { ChevronDown, ChevronUp, Info } from 'lucide-react';

interface Props {
  title: string;
  lines: string[];
}

// Panneau pliable « Comprendre l'indicateur », en surimpression en haut à
// droite d'une carte : définition, calcul, lecture des couleurs, limites.
export default function IndicatorHelp({ title, lines }: Props) {
  const [open, setOpen] = useState(false);
  return (
    <div className="absolute top-3 right-3 z-[1000]" style={{ maxWidth: 'min(24rem, calc(100% - 4.5rem))' }}>
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1.5 bg-white/95 rounded-lg shadow px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-white"
        title={open ? "Masquer l'explication" : "Comprendre l'indicateur"}
      >
        <Info size={14} className="text-blue-600" />
        Comprendre l'indicateur
        {open ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
      </button>
      {open && (
        <div className="mt-1 bg-white/95 rounded-lg shadow-lg p-3 text-xs text-gray-700 space-y-1.5">
          <div className="font-semibold text-gray-900">{title}</div>
          {lines.map((l, i) => (
            <p key={i}>{l}</p>
          ))}
        </div>
      )}
    </div>
  );
}
