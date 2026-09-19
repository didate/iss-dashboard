import { AlertTriangle } from 'lucide-react';

// Bandeau « prototype » commun aux deux espaces. À retirer (ou vider) quand le
// MSHP aura validé les données et le référentiel de normes.
export default function PrototypeBanner() {
  return (
    <div className="shrink-0 bg-amber-50 border-b border-amber-200 text-amber-900 text-xs px-4 py-1.5 flex items-center gap-2">
      <AlertTriangle size={14} className="shrink-0" />
      <span>
        <strong>Prototype</strong> — données du recensement ISS en cours de validation ; le référentiel de normes utilisé pour la conformité est un exemple non officiel. Les indicateurs ne constituent pas une position du Ministère.
      </span>
    </div>
  );
}
