import {Metronome, Sparkles} from 'lucide-react';
import {useEffect, useMemo, useState} from 'react';
import PracticeRow from '../components/practice/PracticeRow';
import PracticeSourcePicker from '../components/practice/PracticeSourcePicker';
import ConfirmModal from '../components/songs/ConfirmModal';
import {useBandFolders} from '../hooks/bandfolders';
import {useBands} from '../hooks/bands';
import {
  useBandSongs,
  useLogBandRehearsalInList,
  useUndoBandRehearsalInList,
} from '../hooks/bandsongs';
import {useFolders} from '../hooks/folders';
import {useLogPractice, useSongs, useUndoPractice} from '../hooks/songs';
import {localToday} from '../lib/dates';
import {
  effectiveMode,
  isBandScoped,
  loadPracticeState,
  parseSource,
  resolveCandidates,
  savePracticeState,
  suggest,
} from '../lib/practice';
import type {ParsedSource, PracticeMode, PracticeState} from '../lib/practice';
import type {SongListItem} from '../lib/types';

// useBandData subscribes to a source's band song list / folders, enabled only
// when that source actually needs them.
function useBandData(parsed: ParsedSource, mode: PracticeMode) {
  const bandId =
    parsed.kind === 'band' || parsed.kind === 'bandfolder' ? parsed.bandId : 0;
  const {data: bandSongs = []} = useBandSongs(
    bandId,
    bandId > 0 && mode === 'band',
  );
  const {data: bandFolders = []} = useBandFolders(
    bandId,
    parsed.kind === 'bandfolder' && bandId > 0,
  );
  return {bandSongs, bandFolders};
}

export default function PracticePage() {
  const [state, setState] = useState<PracticeState>(loadPracticeState);
  const [pendingUndo, setPendingUndo] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    savePracticeState(state);
  }, [state]);

  useEffect(() => {
    if (!error) return;
    const timer = setTimeout(() => setError(null), 6000);
    return () => clearTimeout(timer);
  }, [error]);

  const {data: songs = []} = useSongs();
  const {data: folders = []} = useFolders();
  const {data: bands = []} = useBands();

  const controlParsed = parseSource(state.source);
  const controlMode = effectiveMode(state.source, state.mode);
  const controlBand = useBandData(controlParsed, controlMode);

  const gen = state.generated;
  const genParsed: ParsedSource = gen ? parseSource(gen.source) : {kind: 'all'};
  const genMode: PracticeMode = gen?.mode ?? 'personal';
  const genBand = useBandData(genParsed, genMode);
  const genBandId =
    genParsed.kind === 'band' || genParsed.kind === 'bandfolder'
      ? genParsed.bandId
      : 0;

  const logPractice = useLogPractice();
  const undoPractice = useUndoPractice();
  const logRehearsal = useLogBandRehearsalInList(genBandId);
  const undoRehearsal = useUndoBandRehearsalInList(genBandId);

  const today = localToday();

  const generate = () => {
    const candidates = resolveCandidates(controlParsed, controlMode, {
      songs,
      folders,
      bandSongs: controlBand.bandSongs,
      bandFolders: controlBand.bandFolders,
    });
    const list = suggest(candidates, state.count);
    setState(s => ({
      ...s,
      generated: {source: s.source, mode: controlMode, list},
    }));
  };

  // Rebuild rows from live data so titles/dates/done-state stay fresh; drop
  // ids that no longer resolve in the generated source.
  const rows = useMemo(() => {
    if (!gen) return [] as SongListItem[];
    const resolved = resolveCandidates(genParsed, genMode, {
      songs,
      folders,
      bandSongs: genBand.bandSongs,
      bandFolders: genBand.bandFolders,
    });
    const byId = new Map(resolved.map(s => [s.id, s]));
    return gen.list
      .map(id => byId.get(id))
      .filter((s): s is SongListItem => s !== undefined);
  }, [
    gen,
    genParsed,
    genMode,
    songs,
    folders,
    genBand.bandSongs,
    genBand.bandFolders,
  ]);

  const genBandRole =
    genBandId > 0 ? bands.find(b => b.id === genBandId)?.role : undefined;
  const canAct = !(genMode === 'band' && genBandRole === 'viewer');
  const isBandMode = genMode === 'band';
  const actionLabel = isBandMode ? 'Rehearsed' : 'Practiced';

  const logFor = (id: number) => {
    setError(null);
    const onError = () => setError('Could not log — try again.');
    if (isBandMode) {
      logRehearsal.mutate({songId: id, date: today}, {onError});
    } else {
      logPractice.mutate({id, date: today}, {onError});
    }
  };

  const undoFor = (id: number) => {
    setError(null);
    const onError = () => setError('Could not undo — try again.');
    if (isBandMode) {
      undoRehearsal.mutate({songId: id, date: today}, {onError});
    } else {
      undoPractice.mutate({id, date: today}, {onError});
    }
    setPendingUndo(null);
  };

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-display flex items-center gap-2 text-2xl font-bold tracking-tight">
          <Metronome className="text-primary size-6" />
          Practice
        </h1>
        <p className="text-base-content/55 text-sm">
          Suggests the songs you haven’t practiced in the longest.
        </p>
      </div>

      <div className="border-base-300/60 bg-base-100 flex flex-wrap items-end gap-3 rounded-box border p-4">
        <label className="flex flex-col gap-1">
          <span className="text-base-content/60 text-xs font-medium">
            Source
          </span>
          <PracticeSourcePicker
            value={state.source}
            onChange={source => setState(s => ({...s, source}))}
          />
        </label>

        {isBandScoped(state.source) && (
          <div className="join" role="group" aria-label="Mode">
            <button
              type="button"
              className={`btn join-item btn-sm ${state.mode === 'personal' ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => setState(s => ({...s, mode: 'personal'}))}
            >
              Personal
            </button>
            <button
              type="button"
              className={`btn join-item btn-sm ${state.mode === 'band' ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => setState(s => ({...s, mode: 'band'}))}
            >
              Band
            </button>
          </div>
        )}

        <label className="flex flex-col gap-1">
          <span className="text-base-content/60 text-xs font-medium">
            Songs
          </span>
          <select
            className="select"
            aria-label="Number of songs"
            value={state.count}
            onChange={e =>
              setState(s => ({...s, count: Number(e.target.value)}))
            }
          >
            {Array.from({length: 10}, (_, i) => i + 1).map(n => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>

        <button className="btn btn-primary gap-1.5" onClick={generate}>
          <Sparkles className="size-4" />
          Suggest Songs
        </button>
      </div>

      {!gen ? (
        <div className="border-base-300/60 text-base-content/60 rounded-box border border-dashed py-16 text-center">
          <p>Choose a source and hit “Suggest Songs”.</p>
        </div>
      ) : rows.length === 0 ? (
        <div className="border-base-300/60 text-base-content/60 rounded-box border border-dashed py-16 text-center">
          <p>No songs in this source.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-2">
          {rows.map(song => {
            const done = song.lastPracticedAt === today;
            return (
              <PracticeRow
                key={song.id}
                song={song}
                linkTo={
                  isBandMode && genBandId > 0
                    ? `/bands/${genBandId}/songs/${song.id}`
                    : `/songs/${song.id}`
                }
                done={done}
                actionLabel={actionLabel}
                canAct={canAct}
                onToggle={() =>
                  done ? setPendingUndo(song.id) : logFor(song.id)
                }
              />
            );
          })}
        </ul>
      )}

      <ConfirmModal
        open={pendingUndo !== null}
        title={isBandMode ? 'Undo rehearsal?' : 'Undo practice?'}
        message={
          isBandMode
            ? 'This removes today’s rehearsal entry for this song.'
            : 'This removes today’s practice entry for this song.'
        }
        confirmLabel="Undo"
        onConfirm={() => pendingUndo !== null && undoFor(pendingUndo)}
        onCancel={() => setPendingUndo(null)}
      />

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
