import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, ShieldAlert, BarChart3, Map, Settings, LogOut, User } from 'lucide-react';
import type { AuthUser } from '../api/auth';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Vue d\'ensemble' },
  { to: '/quality', icon: ShieldAlert, label: 'Qualite' },
  { to: '/usage', icon: BarChart3, label: 'Utilisation' },
  { to: '/carte', icon: Map, label: 'Carte' },
  { to: '/admin', icon: Settings, label: 'Admin' },
];

interface Props {
  user: AuthUser | null;
  onLogout: () => void;
}

export default function Layout({ user, onLogout }: Props) {
  return (
    <div className="flex h-screen">
      <nav className="w-56 bg-gray-900 text-gray-300 flex flex-col shrink-0">
        <div className="p-4 border-b border-gray-700">
          <h1 className="text-white font-bold text-lg">ISS Dashboard</h1>
          <p className="text-xs text-gray-500 mt-1">Qualite & Utilisation</p>
        </div>
        <ul className="flex-1 py-2">
          {navItems.map((item) => (
            <li key={item.to}>
              <NavLink
                to={item.to}
                end={item.to === '/'}
                className={({ isActive }) =>
                  `flex items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
                    isActive
                      ? 'bg-gray-800 text-white border-l-2 border-blue-500'
                      : 'hover:bg-gray-800 hover:text-white border-l-2 border-transparent'
                  }`
                }
              >
                <item.icon size={18} />
                {item.label}
              </NavLink>
            </li>
          ))}
        </ul>

        {/* User info */}
        <div className="border-t border-gray-700 p-3">
          {user ? (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs">
                <User size={14} />
                <span className="text-gray-400">{user.name || user.username}</span>
              </div>
              <button onClick={onLogout} className="text-gray-500 hover:text-gray-300" title="Deconnexion">
                <LogOut size={14} />
              </button>
            </div>
          ) : (
            <NavLink to="/login" className="flex items-center gap-2 text-xs text-gray-400 hover:text-white">
              <User size={14} />
              Se connecter
            </NavLink>
          )}
        </div>
      </nav>
      <main className="flex-1 overflow-auto bg-gray-50 p-6">
        <Outlet />
      </main>
    </div>
  );
}
