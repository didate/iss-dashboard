import { Info } from 'lucide-react';
import { useState } from 'react';

interface Props {
  title: string;
  children: React.ReactNode;
}

export default function MethodNote({ title, children }: Props) {
  const [open, setOpen] = useState(false);

  return (
    <div className="mt-8 border border-gray-200 rounded-lg bg-gray-50">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 px-4 py-3 text-sm text-gray-600 hover:text-gray-800"
      >
        <Info size={16} />
        <span className="font-medium">{title}</span>
        <span className="ml-auto text-xs text-gray-400">{open ? 'Masquer' : 'Afficher'}</span>
      </button>
      {open && (
        <div className="px-4 pb-4 text-sm text-gray-600 space-y-2 border-t border-gray-200 pt-3">
          {children}
        </div>
      )}
    </div>
  );
}
