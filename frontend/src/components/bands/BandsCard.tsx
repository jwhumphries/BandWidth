import {useState} from 'react';
import {Link} from 'react-router';
import ConfirmModal from '../songs/ConfirmModal';
import {useMe} from '../../hooks/auth';
import {useBands, useRemoveMember} from '../../hooks/bands';
import type {BandSummary} from '../../lib/types';

function LeaveButton({band}: {band: BandSummary}) {
  const {data: me} = useMe();
  const removeMember = useRemoveMember(band.id);
  const [confirming, setConfirming] = useState(false);

  return (
    <>
      <button
        className="btn btn-ghost btn-xs"
        onClick={() => setConfirming(true)}
      >
        Leave
      </button>
      <ConfirmModal
        open={confirming}
        title="Leave band"
        message={`Leave "${band.name}"?`}
        confirmLabel="Leave"
        onConfirm={() => {
          if (me) {
            removeMember.mutate(me.id);
          }
          setConfirming(false);
        }}
        onCancel={() => setConfirming(false)}
      />
      {removeMember.error && (
        <div role="alert" className="alert alert-error">
          {removeMember.error.message}
        </div>
      )}
    </>
  );
}

export default function BandsCard() {
  const {data: me} = useMe();
  const {data: bands = []} = useBands();

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Bands</h2>
        {bands.length === 0 ? (
          <p className="text-base-content/60 text-sm">
            You are not in any bands yet.{' '}
            <Link className="link" to="/bands">
              Create or join one.
            </Link>
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {bands.map(band => (
              <li key={band.id} className="flex items-center gap-3">
                <Link
                  to={`/bands/${band.id}`}
                  className="link min-w-0 flex-1 truncate"
                >
                  {band.name}
                </Link>
                <span className="badge badge-ghost">{band.role}</span>
                {/* The creator cannot leave — they delete the band instead. */}
                {me && band.creatorId !== me.id && <LeaveButton band={band} />}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
