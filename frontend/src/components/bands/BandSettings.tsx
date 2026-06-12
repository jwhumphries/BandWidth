import {useState} from 'react';
import type {FormEvent} from 'react';
import {useNavigate} from 'react-router';
import ConfirmModal from '../songs/ConfirmModal';
import {useDeleteBand, useRenameBand} from '../../hooks/bands';
import {useMe} from '../../hooks/auth';
import type {BandDetail} from '../../lib/types';

export default function BandSettings({band}: {band: BandDetail}) {
  const {data: me} = useMe();
  const renameBand = useRenameBand(band.id);
  const deleteBand = useDeleteBand();
  const navigate = useNavigate();
  const [name, setName] = useState(band.name);
  const [confirming, setConfirming] = useState(false);
  const isCreator = me?.id === band.creatorId;

  const rename = (e: FormEvent) => {
    e.preventDefault();
    if (name.trim()) {
      renameBand.mutate({name: name.trim()});
    }
  };

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Settings</h2>
        <form className="flex gap-2" onSubmit={rename}>
          <input
            className="input min-w-0 flex-1"
            aria-label="Band name"
            value={name}
            onChange={e => setName(e.target.value)}
          />
          <button className="btn" disabled={renameBand.isPending}>
            Rename
          </button>
        </form>
        {renameBand.error && (
          <div role="alert" className="alert alert-error">
            {renameBand.error.message}
          </div>
        )}
        {isCreator && (
          <div className="card-actions">
            <button
              className="btn btn-error btn-outline"
              onClick={() => setConfirming(true)}
            >
              Delete band
            </button>
          </div>
        )}
        <ConfirmModal
          open={confirming}
          title="Delete band"
          message={`Delete "${band.name}" for every member?`}
          confirmLabel="Delete"
          onConfirm={() =>
            deleteBand.mutate(band.id, {
              onSuccess: () => void navigate('/bands'),
            })
          }
          onCancel={() => setConfirming(false)}
        />
      </div>
    </section>
  );
}
