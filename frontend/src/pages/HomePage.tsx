import Fuse from 'fuse.js';
import {Music, Plus, Search} from 'lucide-react';
import {useEffect, useMemo, useState} from 'react';
import FolderSidebar from '../components/folders/FolderSidebar';
import SortableSongList from '../components/folders/SortableSongList';
import AddSongModal from '../components/songs/AddSongModal';
import LibraryProgress from '../components/songs/LibraryProgress';
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
  const [practiceError, setPracticeError] = useState<string | null>(null);

  useEffect(() => {
    if (!undo) return;
    const timer = setTimeout(() => setUndo(null), 6000);
    return () => clearTimeout(timer);
  }, [undo]);

  useEffect(() => {
    if (!practiceError) return;
    const timer = setTimeout(() => setPracticeError(null), 6000);
    return () => clearTimeout(timer);
  }, [practiceError]);

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
    setPracticeError(null);
    const song = songs.find(s => s.id === songId);
    logPractice.mutate(
      {id: songId, date},
      {
        onSuccess: () => setUndo({songId, date, title: song?.title ?? 'song'}),
        onError: () => setPracticeError('Could not log practice — try again.'),
      },
    );
  };

  return (
    <div className="flex flex-col gap-6 sm:flex-row">
      <FolderSidebar selectedId={folderId} onSelect={setFolderId} />

      <div className="flex min-w-0 flex-1 flex-col gap-4">
        <div className="flex flex-wrap items-end justify-between gap-2">
          <div>
            <h1 className="font-display text-2xl font-bold tracking-tight">
              {selectedFolder ? selectedFolder.name : 'Library'}
            </h1>
            <p className="text-base-content/55 text-sm">
              {folderSongs.length} {folderSongs.length === 1 ? 'song' : 'songs'}
            </p>
          </div>
        </div>

        <LibraryProgress songs={folderSongs} />

        <div className="flex items-center gap-3">
          <label className="input flex-1">
            <Search className="text-base-content/40 size-4" />
            <input
              placeholder="Search songs…"
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
          </label>
          <button
            className="btn btn-primary gap-1.5"
            onClick={() => setAdding(true)}
          >
            <Plus className="size-4" />
            Add song
          </button>
        </div>

        {visible.length === 0 ? (
          <div className="border-base-300/60 text-base-content/60 flex flex-col items-center gap-3 rounded-box border border-dashed py-16 text-center">
            <Music className="text-base-content/30 size-10" />
            {songs.length === 0 ? (
              <>
                <p className="text-base-content/80 font-medium">
                  Your library is empty
                </p>
                <button
                  className="btn btn-primary btn-sm gap-1.5"
                  onClick={() => setAdding(true)}
                >
                  <Plus className="size-4" />
                  Add your first song
                </button>
              </>
            ) : searching ? (
              <p>No songs match “{search.trim()}”.</p>
            ) : (
              <p>No songs in this folder yet.</p>
            )}
          </div>
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
            <div role="status" className="alert alert-success">
              <span>Practiced &quot;{undo.title}&quot;</span>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => {
                  undoPractice.mutate(
                    {id: undo.songId, date: undo.date},
                    {
                      onError: () =>
                        setPracticeError('Could not undo — try again.'),
                    },
                  );
                  setUndo(null);
                }}
              >
                Undo
              </button>
            </div>
          </div>
        )}
        {practiceError && (
          <div className="toast toast-center">
            <div role="alert" className="alert alert-error">
              <span>{practiceError}</span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
