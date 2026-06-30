import {
  AudioLines,
  LibraryBig,
  LogOut,
  Moon,
  Sun,
  User,
  Users,
} from 'lucide-react';
import type {ReactNode} from 'react';
import {Link, NavLink, Outlet} from 'react-router';
import {useLogout} from '../hooks/auth';
import {useMyInvites} from '../hooks/invites';
import {useTheme} from '../lib/theme';

function NavItem({
  to,
  icon,
  label,
  badge,
}: {
  to: string;
  icon: ReactNode;
  label: string;
  badge?: number;
}) {
  return (
    <NavLink
      to={to}
      end={to === '/'}
      aria-label={label}
      className={({isActive}) =>
        `group relative flex items-center gap-2 rounded-field px-3 py-2 text-sm font-medium transition-colors ${
          isActive
            ? 'bg-base-300 text-base-content'
            : 'text-base-content/65 hover:bg-base-300/60 hover:text-base-content'
        }`
      }
    >
      {icon}
      <span className="hidden sm:inline">{label}</span>
      {badge !== undefined && badge > 0 && (
        <span className="badge badge-primary badge-sm font-semibold">
          {badge}
        </span>
      )}
    </NavLink>
  );
}

export default function Layout() {
  const logout = useLogout();
  const {data: invites = []} = useMyInvites();
  const {theme, toggle} = useTheme();

  return (
    <div className="bg-base-200 text-base-content min-h-screen">
      <header className="border-base-300/70 bg-base-100/80 sticky top-0 z-30 border-b backdrop-blur-lg">
        <nav className="mx-auto flex max-w-6xl items-center gap-2 px-4 py-2.5">
          <Link
            to="/"
            className="group mr-2 flex items-center gap-2"
            aria-label="BandWidth home"
          >
            <span className="bg-primary text-primary-content grid size-8 place-items-center rounded-field shadow-sm transition-transform group-hover:scale-105">
              <AudioLines className="size-5" strokeWidth={2.25} />
            </span>
            <span className="font-display text-xl font-bold tracking-tight">
              Band<span className="text-primary">Width</span>
            </span>
          </Link>

          <div className="flex flex-1 items-center justify-end gap-1">
            <NavItem
              to="/"
              icon={<LibraryBig className="size-4" />}
              label="Library"
            />
            <NavItem
              to="/bands"
              icon={<Users className="size-4" />}
              label="Bands"
              badge={invites.length}
            />
            <NavItem
              to="/profile"
              icon={<User className="size-4" />}
              label="Profile"
            />

            <span className="bg-base-300/70 mx-1 hidden h-6 w-px sm:block" />

            <button
              className="btn btn-ghost btn-sm btn-square"
              onClick={toggle}
              aria-label={
                theme === 'bandwidth'
                  ? 'Switch to light theme'
                  : 'Switch to dark theme'
              }
              title="Toggle theme"
            >
              {theme === 'bandwidth' ? (
                <Sun className="size-4" />
              ) : (
                <Moon className="size-4" />
              )}
            </button>
            <button
              className="btn btn-ghost btn-sm gap-2"
              onClick={() => logout.mutate()}
              disabled={logout.isPending}
              aria-label="Log out"
            >
              <LogOut className="size-4" />
              <span className="hidden sm:inline">Log out</span>
            </button>
          </div>
        </nav>
      </header>

      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
