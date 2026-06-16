import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './api/auth';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Quality from './pages/Quality';
import Usage from './pages/Usage';
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
        <Route path="/carte" element={<MapView />} />
        <Route path="/admin" element={
          isLoggedIn ? <Admin /> : <Navigate to="/login" />
        } />
      </Route>
    </Routes>
  );
}
