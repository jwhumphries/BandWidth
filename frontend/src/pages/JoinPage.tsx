import {AudioLines} from 'lucide-react';
import type {ReactNode} from 'react';
import {Link, useNavigate, useParams} from 'react-router';
import {useMe} from '../hooks/auth';
import {useJoinByLink, useLinkPreview} from '../hooks/invites';

export default function JoinPage() {
  const {token} = useParams();
  const navigate = useNavigate();
  const me = useMe();
  const preview = useLinkPreview(token ?? '');
  const join = useJoinByLink();

  const card = (children: ReactNode) => (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col gap-6">
        <span className="bg-primary text-primary-content grid size-14 place-items-center rounded-box shadow-lg">
          <AudioLines className="size-8" strokeWidth={2.25} />
        </span>
        <div className="card bg-base-100 border-base-300/60 w-full border p-6 text-center shadow-xl">
          {children}
        </div>
      </div>
    </main>
  );

  if (!token) {
    return card(<p>Invalid invite link.</p>);
  }
  if (preview.isPending || me.isPending) {
    return card(
      <span className="loading loading-spinner mx-auto" aria-label="Loading" />,
    );
  }
  if (preview.isError) {
    return card(
      <p className="text-base-content/70">
        This invite link is invalid or has expired.
      </p>,
    );
  }

  const bandName = preview.data.bandName;
  const isLoggedIn = !me.isError && me.data !== undefined;
  const redirect = `?redirect=${encodeURIComponent(`/join/${token}`)}`;

  return card(
    <div className="flex flex-col gap-4">
      <p className="text-lg">
        You&apos;ve been invited to join{' '}
        <span className="font-display font-bold">{bandName}</span>.
      </p>
      {join.error && (
        <div role="alert" className="alert alert-error">
          {join.error.message}
        </div>
      )}
      {isLoggedIn ? (
        <button
          className="btn btn-primary"
          disabled={join.isPending}
          onClick={() =>
            join.mutate(token, {
              onSuccess: ({bandId}) => void navigate(`/bands/${bandId}`),
            })
          }
        >
          Join
        </button>
      ) : (
        <div className="flex flex-col gap-2">
          <Link className="btn btn-primary" to={`/login${redirect}`}>
            Log in
          </Link>
          <Link className="btn btn-ghost" to={`/signup${redirect}`}>
            Sign up
          </Link>
        </div>
      )}
    </div>,
  );
}
