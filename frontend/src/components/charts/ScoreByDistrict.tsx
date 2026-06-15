import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import type { QualitySummaryRow } from '../../types';

function getColor(score: number) {
  if (score >= 80) return '#22c55e';
  if (score >= 50) return '#eab308';
  return '#ef4444';
}

export default function ScoreByDistrict({ data }: { data: QualitySummaryRow[] }) {
  const sorted = [...data].sort((a, b) => a.avg_score - b.avg_score);

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <h3 className="text-sm font-medium text-gray-500 mb-4">Score qualité par district</h3>
      <ResponsiveContainer width="100%" height={Math.max(300, sorted.length * 30)}>
        <BarChart data={sorted} layout="vertical" margin={{ left: 120 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis type="number" domain={[0, 100]} />
          <YAxis type="category" dataKey="label" tick={{ fontSize: 11 }} width={110} />
          <Tooltip formatter={(v: number) => v.toFixed(1)} />
          <Bar dataKey="avg_score" name="Score moyen">
            {sorted.map((entry, i) => (
              <Cell key={i} fill={getColor(entry.avg_score)} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
