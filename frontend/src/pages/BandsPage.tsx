import {ChevronRight, Plus, Users} from 'lucide-react';
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
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 className="font-display text-3xl font-bold tracking-tight">Bands</h1>

      {invites.length > 0 && (
        <section className="card bg-base-100 border-primary/30 border shadow-sm">
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
        <div className="border-base-300/60 text-base-content/60 flex flex-col items-center gap-3 rounded-box border border-dashed py-16 text-center">
          <Users className="text-base-content/30 size-10" />
          <p>No bands yet — create one or ask for an invite.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {bands.map(band => (
            <li key={band.id}>
              <Link
                to={`/bands/${band.id}`}
                className="group border-base-300/60 bg-base-100 hover:border-base-300 flex items-center gap-3 rounded-box border p-4 transition-all hover:shadow-md"
              >
                <span className="bg-base-300/60 text-base-content/70 grid size-9 shrink-0 place-items-center rounded-field">
                  <Users className="size-4" />
                </span>
                <span className="group-hover:text-primary min-w-0 flex-1 truncate font-display font-semibold transition-colors">
                  {band.name}
                </span>
                <span className="badge badge-ghost badge-sm capitalize">
                  {band.role}
                </span>
                <span className="text-base-content/55 font-mono text-xs">
                  {band.memberCount} members
                </span>
                <ChevronRight className="text-base-content/30 group-hover:text-base-content/60 size-4 transition-colors" />
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
        <button
          className="btn btn-primary gap-1.5"
          disabled={createBand.isPending}
        >
          <Plus className="size-4" />
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
