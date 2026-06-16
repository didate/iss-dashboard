import { useEffect, useState, useCallback } from 'react';
import { RefreshCw, CheckCircle, XCircle, Clock, UserPlus, Trash2, Download } from 'lucide-react';
import { api } from '../api/client';
import { getToken } from '../api/auth';
import type { SyncStatus } from '../types';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

export default function Admin() {
  const [status, setStatus] = useState<SyncStatus | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState('');
  const [exporting, setExporting] = useState(false);

  // Users
  const [users, setUsers] = useState<{ id: number; username: string; name: string; role: string }[]>([]);
  const [showAddUser, setShowAddUser] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', password: '', name: '', role: 'viewer' });
  const [userError, setUserError] = useState('');

  const fetchStatus = useCallback(() => {
    api.getSyncStatus().then(setStatus).catch((err) => {
      console.error('fetchStatus error:', err);
    });
  }, []);

  const fetchUsers = useCallback(() => {
    api.getUsers().then((u) => setUsers(u ?? [])).catch(() => {});
  }, []);

  const isRunning = status?.current?.status === 'running';

  useEffect(() => {
    fetchStatus();
    fetchUsers();
  }, [fetchStatus, fetchUsers]);

  // Poll only when sync is running or just triggered
  useEffect(() => {
    if (!isRunning && !syncing) return;
    const interval = setInterval(fetchStatus, 3000);
    return () => clearInterval(interval);
  }, [isRunning, syncing, fetchStatus]);

  const triggerSync = async () => {
    setSyncing(true);
    setError('');
    try {
      await api.triggerSync();
      // Poll more frequently right after triggering
      setTimeout(fetchStatus, 2000);
      setTimeout(fetchStatus, 5000);
      setTimeout(fetchStatus, 10000);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Erreur');
    } finally {
      setSyncing(false);
    }
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      const token = getToken();
      const res = await fetch(`${BASE_URL}/api/export/excel`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`Export failed: ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `extraction_iss_${new Date().toISOString().slice(0, 10)}.xlsx`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Erreur export');
    } finally {
      setExporting(false);
    }
  };

  const handleAddUser = async () => {
    setUserError('');
    try {
      await api.createUser(newUser);
      setNewUser({ username: '', password: '', name: '', role: 'viewer' });
      setShowAddUser(false);
      fetchUsers();
    } catch (e: unknown) {
      setUserError(e instanceof Error ? e.message : 'Erreur');
    }
  };

  const handleDeleteUser = async (id: number, username: string) => {
    if (!confirm(`Supprimer l'utilisateur ${username} ?`)) return;
    try {
      await api.deleteUser(id);
      fetchUsers();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Erreur');
    }
  };

  const statusIcon = (s: string) => {
    if (s === 'success') return <CheckCircle size={16} className="text-green-500" />;
    if (s === 'error') return <XCircle size={16} className="text-red-500" />;
    return <Clock size={16} className="text-yellow-500 animate-spin" />;
  };

  return (
    <div className="space-y-6 max-w-4xl">
      <h2 className="text-xl font-bold text-gray-900">Administration</h2>

      {/* Sync + Export */}
      <div className="bg-white rounded-lg border border-gray-200 p-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="font-medium text-gray-900">Synchronisation</h3>
            {isRunning && (
              <p className="text-sm text-yellow-600 mt-1 flex items-center gap-1">
                <Clock size={14} className="animate-spin" /> Synchro en cours...
              </p>
            )}
            {status?.last && !isRunning && (
              <p className="text-sm text-gray-500 mt-1">
                Derniere : {new Date(status.last.finished_at || status.last.started_at).toLocaleString('fr-FR')}
                {' — '}{status.last.events_pulled} events, {status.last.issues_found} issues, {(status.last.duration_ms / 1000).toFixed(1)}s
                {status.last.status === 'success' && <CheckCircle size={14} className="inline ml-1 text-green-500" />}
                {status.last.status === 'error' && <XCircle size={14} className="inline ml-1 text-red-500" />}
              </p>
            )}
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleExport}
              disabled={exporting}
              className="flex items-center gap-2 bg-gray-100 text-gray-700 px-4 py-2 rounded text-sm font-medium hover:bg-gray-200 disabled:opacity-50"
            >
              <Download size={16} />
              {exporting ? 'Export...' : 'Export Excel'}
            </button>
            <button
              onClick={triggerSync}
              disabled={syncing || isRunning}
              className="flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <RefreshCw size={16} className={isRunning ? 'animate-spin' : ''} />
              {isRunning ? 'En cours...' : 'Synchroniser'}
            </button>
          </div>
        </div>
        {error && <p className="text-sm text-red-600 mt-2">{error}</p>}
      </div>

      {/* Users management */}
      <div className="bg-white rounded-lg border border-gray-200">
        <div className="flex items-center justify-between p-4 border-b">
          <h3 className="font-medium text-gray-900">Utilisateurs</h3>
          <button
            onClick={() => setShowAddUser(!showAddUser)}
            className="flex items-center gap-1 text-sm text-blue-600 hover:text-blue-800"
          >
            <UserPlus size={16} />
            Ajouter
          </button>
        </div>

        {showAddUser && (
          <div className="p-4 border-b bg-gray-50 space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <input
                type="text" placeholder="Nom d'utilisateur"
                className="border border-gray-300 rounded px-3 py-1.5 text-sm"
                value={newUser.username}
                onChange={(e) => setNewUser({ ...newUser, username: e.target.value })}
              />
              <input
                type="password" placeholder="Mot de passe"
                className="border border-gray-300 rounded px-3 py-1.5 text-sm"
                value={newUser.password}
                onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
              />
              <input
                type="text" placeholder="Nom complet"
                className="border border-gray-300 rounded px-3 py-1.5 text-sm"
                value={newUser.name}
                onChange={(e) => setNewUser({ ...newUser, name: e.target.value })}
              />
              <select
                className="border border-gray-300 rounded px-3 py-1.5 text-sm"
                value={newUser.role}
                onChange={(e) => setNewUser({ ...newUser, role: e.target.value })}
              >
                <option value="viewer">Viewer</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            {userError && <p className="text-sm text-red-600">{userError}</p>}
            <button
              onClick={handleAddUser}
              className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm hover:bg-blue-700"
            >
              Creer
            </button>
          </div>
        )}

        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-gray-50">
              <th className="text-left px-3 py-2 font-medium text-gray-500">Utilisateur</th>
              <th className="text-left px-3 py-2 font-medium text-gray-500">Nom</th>
              <th className="text-left px-3 py-2 font-medium text-gray-500">Role</th>
              <th className="px-3 py-2 w-16"></th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id} className="border-b border-gray-100">
                <td className="px-3 py-2 font-medium text-gray-800">{u.username}</td>
                <td className="px-3 py-2 text-gray-600">{u.name}</td>
                <td className="px-3 py-2">
                  <span className={`px-2 py-0.5 rounded text-xs font-medium ${u.role === 'admin' ? 'bg-purple-100 text-purple-700' : 'bg-gray-100 text-gray-600'}`}>
                    {u.role}
                  </span>
                </td>
                <td className="px-3 py-2">
                  <button
                    onClick={() => handleDeleteUser(u.id, u.username)}
                    className="text-red-400 hover:text-red-600"
                    title="Supprimer"
                  >
                    <Trash2 size={14} />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Sync history */}
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
                <th className="text-left px-3 py-2 font-medium text-gray-500">Duree</th>
                <th className="text-left px-3 py-2 font-medium text-gray-500">Erreur</th>
              </tr>
            </thead>
            <tbody>
              {status.history.map((run) => (
                <tr key={run.id} className="border-b border-gray-100">
                  <td className="px-3 py-2 text-gray-700">{new Date(run.started_at).toLocaleString('fr-FR')}</td>
                  <td className="px-3 py-2"><span className="flex items-center gap-1">{statusIcon(run.status)} {run.status}</span></td>
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
