import {Route, Routes} from 'react-router';
import Layout from './components/Layout';
import Footer from './components/Footer';
import UpdateToast from './components/UpdateToast';
import RequireAdmin from './components/RequireAdmin';
import RequireAuth from './components/RequireAuth';
import AdminPage from './pages/AdminPage';
import BandPage from './pages/BandPage';
import BandSongPage from './pages/BandSongPage';
import BandsPage from './pages/BandsPage';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import HomePage from './pages/HomePage';
import JoinPage from './pages/JoinPage';
import LoginPage from './pages/LoginPage';
import ProfilePage from './pages/ProfilePage';
import ResetPasswordPage from './pages/ResetPasswordPage';
import SignupPage from './pages/SignupPage';
import SongPage from './pages/SongPage';

export default function App() {
  return (
    <>
      <UpdateToast />
      <Footer />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/join/:token" element={<JoinPage />} />
        <Route element={<RequireAuth />}>
          <Route element={<Layout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/profile" element={<ProfilePage />} />
            <Route path="/songs/:id" element={<SongPage />} />
            <Route path="/bands" element={<BandsPage />} />
            <Route path="/bands/:id" element={<BandPage />} />
            <Route path="/bands/:id/songs/:songId" element={<BandSongPage />} />
            <Route element={<RequireAdmin />}>
              <Route path="/admin" element={<AdminPage />} />
            </Route>
          </Route>
        </Route>
      </Routes>
    </>
  );
}
