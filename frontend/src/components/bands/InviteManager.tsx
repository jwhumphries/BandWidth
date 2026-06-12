import {useState} from 'react';
import type {FormEvent} from 'react';
import {
  useBandInvites,
  useCreateInvite,
  useRevokeInvite,
} from '../../hooks/bands';
import type {BandRole} from '../../lib/types';

export default function InviteManager({bandId}: {bandId: number}) {
  const {data: invites = []} = useBandInvites(bandId, true);
  const createInvite = useCreateInvite(bandId);
  const revokeInvite = useRevokeInvite(bandId);
  const [username, setUsername] = useState('');
  const [role, setRole] = useState<BandRole>('editor');
  const [linkURL, setLinkURL] = useState<string | null>(null);

  const inviteUser = (e: FormEvent) => {
    e.preventDefault();
    if (!username.trim()) return;
    createInvite.mutate(
      {username: username.trim(), role},
      {onSuccess: () => setUsername('')},
    );
  };

  const createLink = () => {
    createInvite.mutate(
      {link: true, role},
      {
        onSuccess: created => {
          if (created.token) {
            setLinkURL(`${window.location.origin}/join/${created.token}`);
          }
        },
      },
    );
  };

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Invites</h2>
        <form className="flex flex-wrap gap-2" onSubmit={inviteUser}>
          <input
            className="input input-sm min-w-0 flex-1"
            placeholder="Username or email…"
            value={username}
            onChange={e => setUsername(e.target.value)}
          />
          <select
            className="select select-sm"
            aria-label="Invite role"
            value={role}
            onChange={e => setRole(e.target.value as BandRole)}
          >
            <option value="viewer">viewer</option>
            <option value="editor">editor</option>
            <option value="admin">admin</option>
          </select>
          <button className="btn btn-sm" disabled={createInvite.isPending}>
            Invite
          </button>
        </form>
        <button
          className="btn btn-outline btn-sm"
          onClick={createLink}
          disabled={createInvite.isPending}
        >
          Create invite link
        </button>
        {linkURL && (
          <div className="bg-base-200 rounded-box flex items-center gap-2 p-3">
            <code className="min-w-0 flex-1 truncate text-sm">{linkURL}</code>
            <button
              className="btn btn-ghost btn-xs"
              onClick={() => void navigator.clipboard.writeText(linkURL)}
            >
              Copy
            </button>
          </div>
        )}
        {createInvite.error && (
          <div role="alert" className="alert alert-error">
            {createInvite.error.message}
          </div>
        )}
        {invites.length > 0 && (
          <ul className="flex flex-col gap-1">
            {invites.map(invite => (
              <li key={invite.id} className="flex items-center gap-2 text-sm">
                <span className="min-w-0 flex-1 truncate">
                  {invite.isLink ? 'Invite link' : invite.invitedUsername}
                  {' · '}
                  {invite.role}
                </span>
                <button
                  className="btn btn-ghost btn-xs"
                  onClick={() => revokeInvite.mutate(invite.id)}
                >
                  Revoke
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
