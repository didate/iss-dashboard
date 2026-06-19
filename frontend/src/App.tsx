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
import MapView from './pages/MapView';
import Admin from './pages/Admin';
import Login from './pages/Login';

export default function App() {
  const { user, isLoggedIn, login, logout } = useAuth();

  return (
    <Routes>
      <Route path="/login" element={
        isLoggedIn ? <Navigate to="/admin" /> : <Login onLogin={login} />
      } />
      <Route element={<Layout user={user} onLogout={logout} />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/quality" element={<Quality />} />
        <Route path="/usage" element={<Usage />} />
        <Route path="/structures" element={<Structures />} />
        <Route path="/structure/:uid" element={<StructureDetail />} />
        <Route path="/comparaison" element={<Comparison />} />
        <Route path="/rapport/:district" element={<DistrictReport />} />
        <Route path="/carte" element={<MapView />} />
        <Route path="/admin" element={
          isLoggedIn ? <Admin /> : <Navigate to="/login" />
        } />
      </Route>
    </Routes>
  );
}
