import {Link, Outlet} from 'react-router';
import {useLogout} from '../hooks/auth';

export default function Layout() {
  const logout = useLogout();
  return (
    <div className="bg-base-200 min-h-screen">
      <nav className="navbar bg-base-100 shadow">
        <div className="flex-1">
          <Link to="/" className="btn btn-ghost text-xl">
            BandWidth
          </Link>
        </div>
        <div className="flex-none gap-2">
          <Link to="/profile" className="btn btn-ghost">
            Profile
          </Link>
          <button
            className="btn btn-ghost"
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
          >
            Log out
          </button>
        </div>
      </nav>
      <main className="mx-auto max-w-3xl p-4">
        <Outlet />
      </main>
    </div>
  );
}
