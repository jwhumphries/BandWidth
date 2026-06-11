import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import {useNavigate, useParams} from 'react-router';
import ConfirmModal from '../components/songs/ConfirmModal';
import ResourceList from '../components/songs/ResourceList';
import FolderPicker from '../components/folders/FolderPicker';
import {
  useDeleteSong,
  useLogPractice,
  useSong,
  useUpdateSong,
} from '../hooks/songs';
import {localToday} from '../lib/dates';
import type {SongStatus} from '../lib/types';

const statusOptions: Array<{value: SongStatus; label: string}> = [
  {value: 'not_learned', label: 'Not learned'},
  {value: 'learning', label: 'Learning'},
  {value: 'learned', label: 'Learned'},
  {value: 'nailed', label: 'Nailed!'},
];

export default function SongPage() {
  const {id: idParam} = useParams();
  const id = Number(idParam);
  const navigate = useNavigate();
  const {data: song} = useSong(id);
  const updateSong = useUpdateSong(id);
  const deleteSong = useDeleteSong();
  const logPractice = useLogPractice();

  const [title, setTitle] = useState('');
  const [artist, setArtist] = useState('');
  const [notes, setNotes] = useState('');
  const [dirty, setDirty] = useState(false);
  const [backfill, setBackfill] = useState('');
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    if (song && !dirty) {
      setTitle(song.title);
      setArtist(song.artist);
      setNotes(song.notes);
    }
  }, [song, dirty]);

  if (!song) {
    return (
      <div className="flex justify-center py-12">
        <span className="loading loading-spinner" aria-label="Loading" />
      </div>
    );
  }

  const save = (e: FormEvent) => {
    e.preventDefault();
    updateSong.mutate(
      {title, artist, notes},
      {onSuccess: () => setDirty(false)},
    );
  };

  return (
    <div className="flex flex-col gap-6">
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
            onChange={e => updateSong.mutate({status: e.target.value})}
          >
            {statusOptions.map(o => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
          <label className="label" htmlFor="notes">
            Notes
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
            <button className="btn btn-primary" disabled={updateSong.isPending}>
              Save
            </button>
          </div>
        </div>
      </form>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Practice</h2>
          <p>
            {song.practiceCount} days practiced
            {song.lastPracticedAt && <> · last on {song.lastPracticedAt}</>}
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <button
              className="btn btn-outline"
              onClick={() => logPractice.mutate({id, date: localToday()})}
            >
              Practiced today
            </button>
            <input
              type="date"
              className="input w-44"
              aria-label="Backfill date"
              value={backfill}
              onChange={e => setBackfill(e.target.value)}
            />
            <button
              className="btn btn-ghost"
              disabled={!backfill}
              onClick={() =>
                logPractice.mutate(
                  {id, date: backfill},
                  {onSuccess: () => setBackfill('')},
                )
              }
            >
              Log past day
            </button>
          </div>
          {logPractice.error && (
            <div role="alert" className="alert alert-error">
              {logPractice.error.message}
            </div>
          )}
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Links</h2>
          <ResourceList songId={id} resources={song.resources} />
        </div>
      </section>

      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Folders</h2>
          <FolderPicker songId={id} />
        </div>
      </section>

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

      <ConfirmModal
        open={confirming}
        title="Delete song"
        message={`Delete "${song.title}" and all of its notes, links, and practice history?`}
        confirmLabel="Delete"
        onConfirm={() =>
          deleteSong.mutate(id, {onSuccess: () => void navigate('/')})
        }
        onCancel={() => setConfirming(false)}
      />
    </div>
  );
}
