import { useEffect, useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { MapPin, Info, LockKeyhole, List, LayoutDashboard } from 'lucide-react';
import { publicApi } from '../api/public';

interface Props {
  isLoggedIn: boolean;
}

// En-tête léger de l'espace grand public : pas de barre latérale, trois entrées.
export default function PublicLayout({ isLoggedIn }: Props) {
  // L'espace planification est-il ouvert en lecture (DASHBOARD_PUBLIC) ? Si oui, on y va
  // directement ; sinon on passe par la connexion.
  const [dashboardPublic, setDashboardPublic] = useState(true);
  useEffect(() => {
    // champ absent (réponse encore en cache d'une version antérieure) = ouvert
    publicApi.getSummary().then((s) => setDashboardPublic(s.dashboard_public !== false)).catch(() => {});
  }, []);
  const proOpen = isLoggedIn || dashboardPublic;
  const link = ({ isActive }: { isActive: boolean }) =>
    `flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition-colors ${
      isActive ? 'bg-emerald-50 text-emerald-800 font-medium' : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
    }`;

  return (
    <div className="flex flex-col h-screen bg-white">
      <header className="shrink-0 border-b border-gray-200 bg-white z-40">
        <div className="flex items-center justify-between px-4 h-14">
          <NavLink to="/" className="flex items-center gap-2">
            <span className="inline-flex items-center justify-center w-8 h-8 rounded-lg bg-emerald-600 text-white">
              <MapPin size={18} />
            </span>
            <span className="leading-tight">
              <span className="block text-gray-900 font-semibold">Carte sanitaire</span>
              <span className="block text-[11px] text-gray-500">Structures de santé de Guinée</span>
            </span>
          </NavLink>

          <nav className="flex items-center gap-1">
            <NavLink to="/" end className={link}>
              <MapPin size={15} />
              <span className="hidden sm:inline">Carte</span>
            </NavLink>
            <NavLink to="/annuaire" className={link}>
              <List size={15} />
              <span className="hidden sm:inline">Annuaire</span>
            </NavLink>
            <NavLink to="/a-propos" className={link}>
              <Info size={15} />
              <span className="hidden sm:inline">À propos</span>
            </NavLink>
            <NavLink
              to={proOpen ? '/tableau-de-bord' : '/login'}
              className="ml-2 flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50"
            >
              {proOpen ? <LayoutDashboard size={14} /> : <LockKeyhole size={14} />}
              <span className="hidden sm:inline">Espace planification</span>
            </NavLink>
          </nav>
        </div>
      </header>

      <main className="flex-1 min-h-0 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
