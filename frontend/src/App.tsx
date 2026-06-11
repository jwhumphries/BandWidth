import {Route, Routes} from 'react-router';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';
import HomePage from './pages/HomePage';
import LoginPage from './pages/LoginPage';
import SignupPage from './pages/SignupPage';

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<Layout />}>
          <Route path="/" element={<HomePage />} />
        </Route>
      </Route>
    </Routes>
  );
}
