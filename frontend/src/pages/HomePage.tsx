import Fuse from 'fuse.js';
import {useEffect, useMemo, useState} from 'react';
import FolderSidebar from '../components/folders/FolderSidebar';
import SortableSongList from '../components/folders/SortableSongList';
import AddSongModal from '../components/songs/AddSongModal';
import SongRow from '../components/songs/SongRow';
import {useFolders, useSetFolderEntries} from '../hooks/folders';
import {useLogPractice, useSongs, useUndoPractice} from '../hooks/songs';

interface UndoState {
  songId: number;
  date: string;
  title: string;
}

export default function HomePage() {
  const {data: songs = []} = useSongs();
  const {data: folders = []} = useFolders();
  const logPractice = useLogPractice();
  const undoPractice = useUndoPractice();
  const setEntries = useSetFolderEntries();
  const [search, setSearch] = useState('');
  const [adding, setAdding] = useState(false);
  const [undo, setUndo] = useState<UndoState | null>(null);
  const [folderId, setFolderId] = useState<number | null>(null);

  useEffect(() => {
    if (!undo) return;
    const timer = setTimeout(() => setUndo(null), 6000);
    return () => clearTimeout(timer);
  }, [undo]);

  const selectedFolder = folders.find(f => f.id === folderId) ?? null;

  // Folder view shows the folder's songs in folder order.
  const folderSongs = useMemo(() => {
    if (!selectedFolder) return songs;
    const byID = new Map(songs.map(s => [s.id, s]));
    return selectedFolder.songIds
      .map(id => byID.get(id))
      .filter((s): s is NonNullable<typeof s> => s !== undefined);
  }, [songs, selectedFolder]);

  const fuse = useMemo(
    () => new Fuse(folderSongs, {keys: ['title', 'artist'], threshold: 0.35}),
    [folderSongs],
  );
  const searching = search.trim() !== '';
  const visible = searching
    ? fuse.search(search.trim()).map(r => r.item)
    : folderSongs;

  const practiced = (songId: number, date: string) => {
    const song = songs.find(s => s.id === songId);
    logPractice.mutate(
      {id: songId, date},
      {onSuccess: () => setUndo({songId, date, title: song?.title ?? 'song'})},
    );
  };

  return (
    <div className="flex flex-col gap-4 sm:flex-row">
      <FolderSidebar selectedId={folderId} onSelect={setFolderId} />

      <div className="flex min-w-0 flex-1 flex-col gap-4">
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
              : 'No songs here.'}
          </p>
        ) : selectedFolder && !searching ? (
          <SortableSongList
            songs={visible}
            onPracticed={practiced}
            onReorder={songIds =>
              setEntries.mutate({id: selectedFolder.id, songIds})
            }
          />
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
    </div>
  );
}
