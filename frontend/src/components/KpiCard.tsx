import type { ReactNode } from 'react';

interface Props {
  title: string;
  value: string | number;
  subtitle?: string;
  icon?: ReactNode;
  color?: string;
}

function fmt(v: string | number): string {
  if (typeof v === 'number') return v.toLocaleString('fr-FR');
  return v;
}

export default function KpiCard({ title, value, subtitle, icon, color = 'text-gray-900' }: Props) {
  return (
    <div className="bg-white rounded-lg border border-gray-200 p-3 sm:p-5">
      <div className="flex items-center justify-between">
        <p className="text-xs sm:text-sm font-medium text-gray-500">{title}</p>
        {icon && <span className="text-gray-400 hidden sm:inline">{icon}</span>}
      </div>
      <p className={`text-lg sm:text-2xl font-bold mt-1 sm:mt-2 ${color}`}>{fmt(value)}</p>
      {subtitle && <p className="text-xs text-gray-400 mt-1">{subtitle}</p>}
    </div>
  );
}
