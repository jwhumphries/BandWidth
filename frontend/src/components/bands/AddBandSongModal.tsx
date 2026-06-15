import {useState} from 'react';
import type {FormEvent} from 'react';
import {useCreateBandSong} from '../../hooks/bandsongs';

export default function AddBandSongModal({
  bandId,
  open,
  onClose,
}: {
  bandId: number;
  open: boolean;
  onClose: () => void;
}) {
  const createSong = useCreateBandSong(bandId);
  const [title, setTitle] = useState('');
  const [artist, setArtist] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    createSong.mutate(
      {title, artist},
      {
        onSuccess: () => {
          setTitle('');
          setArtist('');
          onClose();
        },
      },
    );
  };

  return (
    <dialog className={`modal ${open ? 'modal-open' : ''}`} open={open}>
      <div className="modal-box">
        <h3 className="text-lg font-bold">Add band song</h3>
        <form onSubmit={submit}>
          <label className="label" htmlFor="band-song-title">
            Title
          </label>
          <input
            id="band-song-title"
            className="input w-full"
            value={title}
            onChange={e => setTitle(e.target.value)}
            required
          />
          <label className="label" htmlFor="band-song-artist">
            Artist
          </label>
          <input
            id="band-song-artist"
            className="input w-full"
            value={artist}
            onChange={e => setArtist(e.target.value)}
          />
          {createSong.error && (
            <div role="alert" className="alert alert-error mt-2">
              {createSong.error.message}
            </div>
          )}
          <div className="modal-action">
            <button type="button" className="btn" onClick={onClose}>
              Cancel
            </button>
            <button className="btn btn-primary" disabled={createSong.isPending}>
              Add
            </button>
          </div>
        </form>
      </div>
    </dialog>
  );
}
