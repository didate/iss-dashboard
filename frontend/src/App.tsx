import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './api/auth';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Quality from './pages/Quality';
import Usage from './pages/Usage';
import Structures from './pages/Structures';
import StructureDetail from './pages/StructureDetail';
import Comparison from './pages/Comparison';
import DistrictReport from './pages/DistrictReport';
import NationalReport from './pages/NationalReport';
import MapView from './pages/MapView';
import Admin from './pages/Admin';
import Geolocalisation from './pages/Geolocalisation';
import Login from './pages/Login';
import PublicLayout from './components/PublicLayout';
import PublicMap from './pages/public/PublicMap';
import PublicFiche from './pages/public/PublicFiche';
import About from './pages/public/About';

export default function App() {
  const { user, isLoggedIn, login, logout } = useAuth();

  return (
    <Routes>
      <Route path="/login" element={
        isLoggedIn ? <Navigate to="/admin" /> : <Login onLogin={login} />
      } />
      {/* Espace public : carte, fiche, à propos — aucune donnée sensible */}
      <Route element={<PublicLayout isLoggedIn={isLoggedIn} />}>
        <Route path="/" element={<PublicMap />} />
        <Route path="/fs/:uid" element={<PublicFiche />} />
        <Route path="/a-propos" element={<About />} />
      </Route>

      {/* Espace planification */}
      <Route element={<Layout user={user} onLogout={logout} />}>
        <Route path="/tableau-de-bord" element={<Dashboard />} />
        <Route path="/quality" element={<Quality />} />
        <Route path="/usage" element={<Usage />} />
        <Route path="/structures" element={<Structures />} />
        <Route path="/structure/:uid" element={<StructureDetail />} />
        <Route path="/comparaison" element={<Comparison />} />
        <Route path="/rapport/:district" element={<DistrictReport />} />
        <Route path="/rapport-national" element={
          isLoggedIn ? <NationalReport /> : <Navigate to="/login" />
        } />
        <Route path="/carte" element={<MapView />} />
        <Route path="/geolocalisation" element={<Geolocalisation />} />
        <Route path="/admin" element={
          isLoggedIn ? <Admin /> : <Navigate to="/login" />
        } />
      </Route>
    </Routes>
  );
}
