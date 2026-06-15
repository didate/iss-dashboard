import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import type { QualitySummaryRow } from '../../types';

interface Props {
  data: QualitySummaryRow[];
}

export default function IssuesByRule({ data }: Props) {
  // Use global summary row which has aggregated counts
  const chartData = data.map((r) => ({
    name: r.label,
    errors: r.n_error,
    warnings: r.n_warning,
    infos: r.n_info,
  }));

  return (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <h3 className="text-sm font-medium text-gray-500 mb-4">Problèmes par dimension</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="name" tick={{ fontSize: 11 }} />
          <YAxis />
          <Tooltip />
          <Legend />
          <Bar dataKey="errors" name="Erreurs" fill="#ef4444" />
          <Bar dataKey="warnings" name="Avertissements" fill="#eab308" />
          <Bar dataKey="infos" name="Infos" fill="#3b82f6" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
