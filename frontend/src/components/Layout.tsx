import {Link, Outlet} from 'react-router';
import {useLogout} from '../hooks/auth';
import {useMyInvites} from '../hooks/invites';

export default function Layout() {
  const logout = useLogout();
  const {data: invites = []} = useMyInvites();
  return (
    <div className="bg-base-200 min-h-screen">
      <nav className="navbar bg-base-100 shadow">
        <div className="flex-1">
          <Link to="/" className="btn btn-ghost text-xl">
            BandWidth
          </Link>
        </div>
        <div className="flex-none gap-2">
          <Link to="/bands" className="btn btn-ghost">
            Bands
            {invites.length > 0 && (
              <span className="badge badge-primary badge-sm">
                {invites.length}
              </span>
            )}
          </Link>
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
