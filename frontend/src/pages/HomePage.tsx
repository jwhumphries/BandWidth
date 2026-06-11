import Fuse from 'fuse.js';
import {useEffect, useMemo, useState} from 'react';
import {useLogPractice, useSongs, useUndoPractice} from '../hooks/songs';
import AddSongModal from '../components/songs/AddSongModal';
import SongRow from '../components/songs/SongRow';

interface UndoState {
  songId: number;
  date: string;
  title: string;
}

export default function HomePage() {
  const {data: songs = []} = useSongs();
  const logPractice = useLogPractice();
  const undoPractice = useUndoPractice();
  const [search, setSearch] = useState('');
  const [adding, setAdding] = useState(false);
  const [undo, setUndo] = useState<UndoState | null>(null);

  useEffect(() => {
    if (!undo) return;
    const timer = setTimeout(() => setUndo(null), 6000);
    return () => clearTimeout(timer);
  }, [undo]);

  const fuse = useMemo(
    () => new Fuse(songs, {keys: ['title', 'artist'], threshold: 0.35}),
    [songs],
  );
  const visible = search.trim()
    ? fuse.search(search.trim()).map(r => r.item)
    : songs;

  const practiced = (songId: number, date: string) => {
    const song = songs.find(s => s.id === songId);
    logPractice.mutate(
      {id: songId, date},
      {onSuccess: () => setUndo({songId, date, title: song?.title ?? 'song'})},
    );
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <input
          className="input flex-1"
          placeholder="Search songs…"
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <button className="btn btn-primary" onClick={() => setAdding(true)}>
          Add song
        </button>
      </div>

      {visible.length === 0 ? (
        <p className="text-base-content/60 py-12 text-center">
          {songs.length === 0
            ? 'No songs yet — add your first one.'
            : 'No songs match your search.'}
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {visible.map(song => (
            <SongRow key={song.id} song={song} onPracticed={practiced} />
          ))}
        </ul>
      )}

      <AddSongModal open={adding} onClose={() => setAdding(false)} />

      {undo && (
        <div className="toast toast-center">
          <div className="alert alert-success">
            <span>Practiced &quot;{undo.title}&quot;</span>
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => {
                undoPractice.mutate({id: undo.songId, date: undo.date});
                setUndo(null);
              }}
            >
              Undo
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
