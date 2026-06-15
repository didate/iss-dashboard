import { Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Quality from './pages/Quality';
import Usage from './pages/Usage';
import Admin from './pages/Admin';

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/quality" element={<Quality />} />
        <Route path="/usage" element={<Usage />} />
        <Route path="/admin" element={<Admin />} />
      </Route>
    </Routes>
  );
}
