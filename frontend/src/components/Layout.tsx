import { NavLink, Outlet } from 'react-router-dom';
import { LayoutDashboard, ShieldAlert, BarChart3, Settings } from 'lucide-react';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Vue d\'ensemble' },
  { to: '/quality', icon: ShieldAlert, label: 'Qualité' },
  { to: '/usage', icon: BarChart3, label: 'Utilisation' },
  { to: '/admin', icon: Settings, label: 'Admin' },
];

export default function Layout() {
  return (
    <div className="flex h-screen">
      <nav className="w-56 bg-gray-900 text-gray-300 flex flex-col shrink-0">
        <div className="p-4 border-b border-gray-700">
          <h1 className="text-white font-bold text-lg">ISS Dashboard</h1>
          <p className="text-xs text-gray-500 mt-1">Qualité & Utilisation</p>
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
      </nav>
      <main className="flex-1 overflow-auto bg-gray-50 p-6">
        <Outlet />
      </main>
    </div>
  );
}
