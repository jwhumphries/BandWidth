import {FolderTree, Link2, Repeat} from 'lucide-react';
import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useNavigate, useParams} from 'react-router';
import BandFolderPicker from '../components/bands/BandFolderPicker';
import ConfirmModal from '../components/songs/ConfirmModal';
import {useBand} from '../hooks/bands';
import {
  useBandSong,
  useCreateBandResource,
  useDeleteBandResource,
  useDeleteBandSong,
  useLogBandRehearsal,
  useUpdateBandSong,
} from '../hooks/bandsongs';
import {localToday} from '../lib/dates';
import type {SongStatus} from '../lib/types';

const statusOptions: Array<{value: SongStatus; label: string}> = [
  {value: 'not_learned', label: 'Not learned'},
  {value: 'learning', label: 'Learning'},
  {value: 'learned', label: 'Learned'},
  {value: 'nailed', label: 'Nailed!'},
];

export default function BandSongPage() {
  const {id: idParam, songId: songParam} = useParams();
  const bandId = Number(idParam);
  const songId = Number(songParam);
  const navigate = useNavigate();
  const {data: band} = useBand(bandId);
  const {data: song, isPending, isError, error} = useBandSong(bandId, songId);
  const updateSong = useUpdateBandSong(bandId, songId);
  const deleteSong = useDeleteBandSong(bandId);
  const logRehearsal = useLogBandRehearsal(bandId, songId);
  const createResource = useCreateBandResource(bandId, songId);
  const deleteResource = useDeleteBandResource(bandId, songId);

  const [title, setTitle] = useState('');
  const [artist, setArtist] = useState('');
  const [notes, setNotes] = useState('');
  const [dirty, setDirty] = useState(false);
  const [resUrl, setResUrl] = useState('');
  const [resLabel, setResLabel] = useState('');
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    if (song && !dirty) {
      setTitle(song.title);
      setArtist(song.artist);
      setNotes(song.notes);
    }
  }, [song, dirty]);

  if (isPending) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner" aria-label="Loading" />
      </div>
    );
  }
  if (isError || !song) {
    return (
      <div className="flex flex-col items-center gap-4 py-12">
        <p>{error?.message ?? 'Could not load this song.'}</p>
        <Link className="btn btn-ghost" to={`/bands/${bandId}`}>
          Back to band
        </Link>
      </div>
    );
  }

  const canEdit = band ? band.myRole !== 'viewer' : false;

  const save = (e: FormEvent) => {
    e.preventDefault();
    updateSong.mutate(
      {title, artist, notes},
      {onSuccess: () => setDirty(false)},
    );
  };
  const addResource = (e: FormEvent) => {
    e.preventDefault();
    createResource.mutate(
      {url: resUrl, label: resLabel},
      {
        onSuccess: () => {
          setResUrl('');
          setResLabel('');
        },
      },
    );
  };

  return (
    <div className="flex flex-col gap-6">
      <Link className="link text-sm" to={`/bands/${bandId}`}>
        ← {band?.name ?? 'Band'}
      </Link>

      {canEdit ? (
        <form className="card bg-base-100 shadow" onSubmit={save}>
          <div className="card-body">
            <label className="label" htmlFor="title">
              Title
            </label>
            <input
              id="title"
              className="input w-full"
              value={title}
              onChange={e => {
                setDirty(true);
                setTitle(e.target.value);
              }}
              required
            />
            <label className="label" htmlFor="artist">
              Artist
            </label>
            <input
              id="artist"
              className="input w-full"
              value={artist}
              onChange={e => {
                setDirty(true);
                setArtist(e.target.value);
              }}
            />
            <label className="label" htmlFor="status">
              Status
            </label>
            <select
              id="status"
              className="select w-full"
              value={song.status}
              onChange={e =>
                updateSong.mutate({status: e.target.value as SongStatus})
              }
            >
              {statusOptions.map(o => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <label className="label" htmlFor="notes">
              Band notes
            </label>
            <textarea
              id="notes"
              className="textarea min-h-32 w-full"
              value={notes}
              onChange={e => {
                setDirty(true);
                setNotes(e.target.value);
              }}
            />
            {updateSong.error && (
              <div role="alert" className="alert alert-error">
                {updateSong.error.message}
              </div>
            )}
            <div className="card-actions justify-end">
              <button
                className="btn btn-primary"
                disabled={updateSong.isPending}
              >
                Save
              </button>
            </div>
          </div>
        </form>
      ) : (
        <div className="card bg-base-100 shadow">
          <div className="card-body">
            <h1 className="text-2xl font-bold">{song.title}</h1>
            <p className="text-base-content/60">{song.artist || '—'}</p>
            <p>
              Status: <span className="badge">{song.status}</span>
            </p>
            {song.notes && <p className="whitespace-pre-wrap">{song.notes}</p>}
          </div>
        </div>
      )}

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">
            <Repeat className="text-primary size-5" />
            Rehearsals
          </h2>
          <p>
            {song.rehearsalCount} rehearsals
            {song.lastRehearsedAt && <> · last on {song.lastRehearsedAt}</>}
          </p>
          {canEdit && (
            <div className="card-actions">
              <button
                className="btn btn-outline"
                onClick={() => logRehearsal.mutate({date: localToday()})}
              >
                Rehearsed today
              </button>
            </div>
          )}
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">
            <Link2 className="text-primary size-5" />
            Band links
          </h2>
          <ul className="flex flex-col gap-1">
            {song.resources.map(r => (
              <li key={r.id} className="flex items-center gap-2">
                <a
                  className="link min-w-0 flex-1 truncate"
                  href={r.url}
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {r.label || r.url}
                </a>
                {canEdit && (
                  <button
                    className="btn btn-ghost btn-xs"
                    aria-label={`Remove ${r.label || r.url}`}
                    onClick={() => deleteResource.mutate(r.id)}
                  >
                    ✕
                  </button>
                )}
              </li>
            ))}
          </ul>
          {canEdit && (
            <form className="flex flex-wrap gap-2" onSubmit={addResource}>
              <input
                className="input input-sm min-w-0 flex-1"
                placeholder="https://…"
                aria-label="Band resource URL"
                value={resUrl}
                onChange={e => setResUrl(e.target.value)}
                required
              />
              <input
                className="input input-sm w-32"
                placeholder="Label"
                aria-label="Band resource label"
                value={resLabel}
                onChange={e => setResLabel(e.target.value)}
              />
              <button
                className="btn btn-sm"
                disabled={createResource.isPending}
              >
                Add link
              </button>
            </form>
          )}
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">
            <FolderTree className="text-primary size-5" />
            Folders
          </h2>
          <BandFolderPicker bandId={bandId} songId={songId} canEdit={canEdit} />
        </div>
      </section>

      {canEdit && (
        <section className="card bg-base-100 shadow">
          <div className="card-body">
            <h2 className="card-title">Danger zone</h2>
            <div className="card-actions">
              <button
                className="btn btn-error btn-outline"
                onClick={() => setConfirming(true)}
              >
                Delete song
              </button>
            </div>
          </div>
        </section>
      )}

      <ConfirmModal
        open={confirming}
        title="Delete band song"
        message={`Delete "${song.title}" from the band? Each member who personally tracked it keeps a personal copy.`}
        confirmLabel="Delete"
        onConfirm={() =>
          deleteSong.mutate(song.id, {
            onSuccess: () => void navigate(`/bands/${bandId}`),
          })
        }
        onCancel={() => setConfirming(false)}
      />
    </div>
  );
}
