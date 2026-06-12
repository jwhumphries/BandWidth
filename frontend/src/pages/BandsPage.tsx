import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link} from 'react-router';
import {useBands, useCreateBand} from '../hooks/bands';
import {
  useAcceptInvite,
  useDeclineInvite,
  useMyInvites,
} from '../hooks/invites';

export default function BandsPage() {
  const {data: bands = []} = useBands();
  const {data: invites = []} = useMyInvites();
  const createBand = useCreateBand();
  const acceptInvite = useAcceptInvite();
  const declineInvite = useDeclineInvite();
  const [name, setName] = useState('');

  const create = (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    createBand.mutate({name: name.trim()}, {onSuccess: () => setName('')});
  };

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-bold">Bands</h1>

      {invites.length > 0 && (
        <section className="card bg-base-100 shadow">
          <div className="card-body">
            <h2 className="card-title">Invitations</h2>
            <ul className="flex flex-col gap-2">
              {invites.map(invite => (
                <li key={invite.id} className="flex items-center gap-3">
                  <span className="min-w-0 flex-1 truncate">
                    {invite.bandName}{' '}
                    <span className="badge badge-ghost">{invite.role}</span>
                  </span>
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={() => acceptInvite.mutate(invite.id)}
                  >
                    Accept
                  </button>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => declineInvite.mutate(invite.id)}
                  >
                    Decline
                  </button>
                </li>
              ))}
            </ul>
            {(acceptInvite.error ?? declineInvite.error) && (
              <div role="alert" className="alert alert-error">
                {(acceptInvite.error ?? declineInvite.error)?.message}
              </div>
            )}
          </div>
        </section>
      )}

      {bands.length === 0 ? (
        <p className="text-base-content/60 py-6 text-center">
          No bands yet — create one or ask for an invite.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {bands.map(band => (
            <li key={band.id}>
              <Link
                to={`/bands/${band.id}`}
                className="bg-base-100 flex items-center gap-3 rounded-box p-4 shadow-sm"
              >
                <span className="min-w-0 flex-1 truncate font-semibold">
                  {band.name}
                </span>
                <span className="badge badge-ghost">{band.role}</span>
                <span className="text-base-content/60 text-sm">
                  {band.memberCount} members
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <form className="flex gap-2" onSubmit={create}>
        <input
          className="input min-w-0 flex-1"
          placeholder="New band name…"
          value={name}
          onChange={e => setName(e.target.value)}
        />
        <button className="btn btn-primary" disabled={createBand.isPending}>
          Create
        </button>
      </form>
      {createBand.error && (
        <div role="alert" className="alert alert-error">
          {createBand.error.message}
        </div>
      )}
    </div>
  );
}
