import { useEffect, useState, useCallback } from 'react';
import { RefreshCw, CheckCircle, XCircle, Clock } from 'lucide-react';
import { api } from '../api/client';
import type { SyncStatus } from '../types';

export default function Admin() {
  const [token, setToken] = useState('');
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState('');

  const fetchStatus = useCallback(() => {
    if (!token) return;
    api.getSyncStatus(token).then(setStatus).catch(() => {});
  }, [token]);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 5000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  const triggerSync = async () => {
    if (!token) {
      setError('Saisissez le token admin');
      return;
    }
    setSyncing(true);
    setError('');
    try {
      await api.triggerSync(token);
      setTimeout(fetchStatus, 1000);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Erreur');
    } finally {
      setSyncing(false);
    }
  };

  const statusIcon = (s: string) => {
    if (s === 'success') return <CheckCircle size={16} className="text-green-500" />;
    if (s === 'error') return <XCircle size={16} className="text-red-500" />;
    return <Clock size={16} className="text-yellow-500 animate-spin" />;
  };

  return (
    <div className="space-y-6 max-w-3xl">
      <h2 className="text-xl font-bold text-gray-900">Administration</h2>

      {/* Token input */}
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <label className="block text-sm font-medium text-gray-700 mb-2">Token admin</label>
        <input
          type="password"
          className="border border-gray-300 rounded px-3 py-2 text-sm w-full"
          placeholder="Saisissez le token admin..."
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
      </div>

      {/* Sync control */}
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="font-medium text-gray-900">Synchronisation</h3>
            {status?.current?.status === 'running' && (
              <p className="text-sm text-yellow-600 mt-1">Synchro en cours...</p>
            )}
            {status?.last && status.last.status !== 'running' && (
              <p className="text-sm text-gray-500 mt-1">
                Dernière : {new Date(status.last.finished_at || status.last.started_at).toLocaleString('fr-FR')}
                {' — '}{status.last.events_pulled} events, {status.last.issues_found} issues, {status.last.duration_ms}ms
              </p>
            )}
          </div>
          <button
            onClick={triggerSync}
            disabled={syncing || status?.current?.status === 'running'}
            className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw size={16} className={status?.current?.status === 'running' ? 'animate-spin' : ''} />
            Synchroniser maintenant
          </button>
        </div>
        {error && <p className="text-sm text-red-600 mt-2">{error}</p>}
      </div>

      {/* History */}
      {status?.history && status.history.length > 0 && (
        <div className="bg-white rounded-lg border border-gray-200">
          <h3 className="font-medium text-gray-900 p-4 border-b">Historique des synchronisations</h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-gray-50">
                <th className="text-left px-3 py-2 font-medium text-gray-500">Date</th>
                <th className="text-left px-3 py-2 font-medium text-gray-500">Statut</th>
                <th className="text-left px-3 py-2 font-medium text-gray-500">Events</th>
                <th className="text-left px-3 py-2 font-medium text-gray-500">Issues</th>
                <th className="text-left px-3 py-2 font-medium text-gray-500">Durée</th>
                <th className="text-left px-3 py-2 font-medium text-gray-500">Erreur</th>
              </tr>
            </thead>
            <tbody>
              {status.history.map((run) => (
                <tr key={run.id} className="border-b border-gray-100">
                  <td className="px-3 py-2 text-gray-700">
                    {new Date(run.started_at).toLocaleString('fr-FR')}
                  </td>
                  <td className="px-3 py-2">
                    <span className="flex items-center gap-1">
                      {statusIcon(run.status)}
                      {run.status}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-gray-700">{run.events_pulled}</td>
                  <td className="px-3 py-2 text-gray-700">{run.issues_found}</td>
                  <td className="px-3 py-2 text-gray-500">{(run.duration_ms / 1000).toFixed(1)}s</td>
                  <td className="px-3 py-2 text-red-600 text-xs truncate max-w-xs">{run.error_text}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
