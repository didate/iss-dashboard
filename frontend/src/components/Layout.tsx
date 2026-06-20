import { useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, ShieldAlert, BarChart3, Building2, ArrowLeftRight, Map, Settings, LogOut, User, Menu, X } from 'lucide-react';
import type { AuthUser } from '../api/auth';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Vue d\'ensemble' },
  { to: '/quality', icon: ShieldAlert, label: 'Qualite' },
  { to: '/usage', icon: BarChart3, label: 'Utilisation' },
  { to: '/structures', icon: Building2, label: 'Structures' },
  { to: '/comparaison', icon: ArrowLeftRight, label: 'Comparaison' },
  { to: '/carte', icon: Map, label: 'Carte' },
  { to: '/admin', icon: Settings, label: 'Admin' },
];

interface Props {
  user: AuthUser | null;
  onLogout: () => void;
}

export default function Layout({ user, onLogout }: Props) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="flex flex-col h-screen">
      {/* Header */}
      <header className="bg-gray-900 text-gray-300 shrink-0 z-40">
        <div className="flex items-center justify-between px-4 h-14">
          {/* Logo */}
          <NavLink to="/" className="flex items-center gap-2">
            <h1 className="text-white font-bold text-lg">ISS Dashboard</h1>
            <span className="text-[10px] text-gray-500 hidden sm:inline">Qualite & Utilisation</span>
          </NavLink>

          {/* Desktop nav */}
          <nav className="hidden md:flex items-center gap-1">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === '/'}
                className={({ isActive }) =>
                  `flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded transition-colors ${
                    isActive
                      ? 'bg-gray-700 text-white'
                      : 'text-gray-400 hover:bg-gray-800 hover:text-white'
                  }`
                }
              >
                <item.icon size={14} />
                {item.label}
              </NavLink>
            ))}
          </nav>

          {/* User + Mobile menu button */}
          <div className="flex items-center gap-3">
            {user ? (
              <div className="hidden sm:flex items-center gap-2 text-xs text-gray-400">
                <User size={14} />
                <span>{user.name || user.username}</span>
                <button onClick={onLogout} className="text-gray-500 hover:text-gray-300 ml-1" title="Deconnexion">
                  <LogOut size={14} />
                </button>
              </div>
            ) : (
              <NavLink to="/login" className="hidden sm:flex items-center gap-1 text-xs text-gray-400 hover:text-white">
                <User size={14} />
                Se connecter
              </NavLink>
            )}
            <button className="md:hidden text-gray-400 hover:text-white" onClick={() => setMenuOpen(!menuOpen)}>
              {menuOpen ? <X size={22} /> : <Menu size={22} />}
            </button>
          </div>
        </div>

        {/* Mobile nav dropdown */}
        {menuOpen && (
          <div className="md:hidden border-t border-gray-700 px-2 py-2">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === '/'}
                onClick={() => setMenuOpen(false)}
                className={({ isActive }) =>
                  `flex items-center gap-2 px-3 py-2 text-sm rounded transition-colors ${
                    isActive
                      ? 'bg-gray-700 text-white'
                      : 'text-gray-400 hover:bg-gray-800 hover:text-white'
                  }`
                }
              >
                <item.icon size={16} />
                {item.label}
              </NavLink>
            ))}
            {/* Mobile user actions */}
            <div className="border-t border-gray-700 mt-2 pt-2">
              {user ? (
                <button onClick={() => { onLogout(); setMenuOpen(false); }} className="flex items-center gap-2 px-3 py-2 text-sm text-gray-400 hover:text-white w-full">
                  <LogOut size={16} />
                  Deconnexion ({user.name || user.username})
                </button>
              ) : (
                <NavLink to="/login" onClick={() => setMenuOpen(false)} className="flex items-center gap-2 px-3 py-2 text-sm text-gray-400 hover:text-white">
                  <User size={16} />
                  Se connecter
                </NavLink>
              )}
            </div>
          </div>
        )}
      </header>

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-gray-50 p-3 sm:p-6">
        <Outlet />
      </main>
    </div>
  );
}
