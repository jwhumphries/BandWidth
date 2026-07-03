import {Plus} from 'lucide-react';
import {useEffect, useState} from 'react';
import SongRow from '../songs/SongRow';
import AddBandSongModal from './AddBandSongModal';
import {
  useBandSongs,
  useLogBandRehearsalInList,
  useUndoBandRehearsalInList,
} from '../../hooks/bandsongs';
import {useBandFolders} from '../../hooks/bandfolders';

interface UndoState {
  songId: number;
  date: string;
  title: string;
}

export default function BandSongList({
  bandId,
  canEdit,
  folderId = null,
}: {
  bandId: number;
  canEdit: boolean;
  folderId?: number | null;
}) {
  const {data: songs = []} = useBandSongs(bandId);
  const {data: folders = []} = useBandFolders(bandId);
  const logRehearsal = useLogBandRehearsalInList(bandId);
  const undoRehearsal = useUndoBandRehearsalInList(bandId);
  const [adding, setAdding] = useState(false);
  const [undo, setUndo] = useState<UndoState | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!undo) return;
    const timer = setTimeout(() => setUndo(null), 6000);
    return () => clearTimeout(timer);
  }, [undo]);

  useEffect(() => {
    if (!error) return;
    const timer = setTimeout(() => setError(null), 6000);
    return () => clearTimeout(timer);
  }, [error]);

  const folder =
    folderId === null ? null : folders.find(f => f.id === folderId);
  const visible =
    folder === null || folder === undefined
      ? songs
      : (() => {
          const byID = new Map(songs.map(s => [s.id, s]));
          return folder.songIds
            .map(id => byID.get(id))
            .filter((s): s is (typeof songs)[number] => s !== undefined);
        })();

  const rehearsed = (songId: number, date: string) => {
    setError(null);
    const song = songs.find(s => s.id === songId);
    logRehearsal.mutate(
      {songId, date},
      {
        onSuccess: () => setUndo({songId, date, title: song?.title ?? 'song'}),
        onError: () => setError('Could not log rehearsal — try again.'),
      },
    );
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-xl font-bold tracking-tight">Songs</h2>
        {canEdit && (
          <button
            className="btn btn-primary btn-sm gap-1.5"
            onClick={() => setAdding(true)}
          >
            <Plus className="size-4" />
            Add song
          </button>
        )}
      </div>

      {visible.length === 0 ? (
        <p className="border-base-300/60 text-base-content/60 rounded-box border border-dashed py-12 text-center text-sm">
          No band songs yet.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {visible.map(song => (
            <SongRow
              key={song.id}
              song={song}
              linkTo={`/bands/${bandId}/songs/${song.id}`}
              canEdit={canEdit}
              actionLabel="Rehearsed"
              onPracticed={rehearsed}
            />
          ))}
        </ul>
      )}

      <AddBandSongModal
        bandId={bandId}
        open={adding}
        onClose={() => setAdding(false)}
      />

      {undo && (
        <div className="toast toast-center">
          <div role="status" className="alert alert-success">
            <span>Rehearsed &quot;{undo.title}&quot;</span>
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => {
                undoRehearsal.mutate(
                  {songId: undo.songId, date: undo.date},
                  {onError: () => setError('Could not undo — try again.')},
                );
                setUndo(null);
              }}
            >
              Undo
            </button>
          </div>
        </div>
      )}
      {error && (
        <div className="toast toast-center">
          <div role="alert" className="alert alert-error">
            <span>{error}</span>
          </div>
        </div>
      )}
    </div>
  );
}
