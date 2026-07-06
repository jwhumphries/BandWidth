import type {Folder, SongListItem} from './types';

export type PracticeMode = 'personal' | 'band';

export type ParsedSource =
  | {kind: 'all'}
  | {kind: 'folder'; folderId: number}
  | {kind: 'band'; bandId: number}
  | {kind: 'bandfolder'; bandId: number; folderId: number};

export interface SourceData {
  songs: SongListItem[];
  folders: Folder[];
  bandSongs: SongListItem[];
  bandFolders: Folder[];
}

export interface GeneratedList {
  source: string;
  mode: PracticeMode;
  list: number[];
}

export interface PracticeState {
  source: string;
  mode: PracticeMode;
  count: number;
  generated: GeneratedList | null;
}

const STORAGE_KEY = 'bandwidth:practice';

const DEFAULT_STATE: PracticeState = {
  source: 'all',
  mode: 'personal',
  count: 3,
  generated: null,
};

// parseSource decodes a source <select> value; unknown values fall back to all.
export function parseSource(value: string): ParsedSource {
  const parts = value.split(':');
  if (parts[0] === 'folder' && parts.length === 2) {
    return {kind: 'folder', folderId: Number(parts[1])};
  }
  if (parts[0] === 'band' && parts.length === 2) {
    return {kind: 'band', bandId: Number(parts[1])};
  }
  if (parts[0] === 'bandfolder' && parts.length === 3) {
    return {kind: 'bandfolder', bandId: Number(parts[1]), folderId: Number(parts[2])};
  }
  return {kind: 'all'};
}

// isBandScoped reports whether the Personal/Band mode toggle applies.
export function isBandScoped(value: string): boolean {
  const parsed = parseSource(value);
  return parsed.kind === 'band' || parsed.kind === 'bandfolder';
}

// effectiveMode forces personal mode for non-band sources.
export function effectiveMode(value: string, mode: PracticeMode): PracticeMode {
  return isBandScoped(value) ? mode : 'personal';
}

function byIds(songs: SongListItem[], ids: number[] | undefined): SongListItem[] {
  if (!ids) return [];
  const wanted = new Set(ids);
  return songs.filter(s => wanted.has(s.id));
}

// resolveCandidates picks the songs a source+mode ranks over: the personal
// library (personal mode) or the band song list (band mode).
export function resolveCandidates(
  parsed: ParsedSource,
  mode: PracticeMode,
  data: SourceData,
): SongListItem[] {
  switch (parsed.kind) {
    case 'all':
      return data.songs;
    case 'folder':
      return byIds(
        data.songs,
        data.folders.find(f => f.id === parsed.folderId)?.songIds,
      );
    case 'band':
      return mode === 'band'
        ? data.bandSongs
        : data.songs.filter(s => s.bandId === parsed.bandId);
    case 'bandfolder': {
      const ids = data.bandFolders.find(f => f.id === parsed.folderId)?.songIds;
      return mode === 'band' ? byIds(data.bandSongs, ids) : byIds(data.songs, ids);
    }
  }
}

// suggest ranks least-recently-practiced first (never practiced first),
// breaking ties randomly via rng, and returns up to count song ids.
export function suggest(
  candidates: SongListItem[],
  count: number,
  rng: () => number = Math.random,
): number[] {
  return candidates
    .map(song => ({song, key: rng()}))
    .sort((a, b) => {
      if (a.song.lastPracticedAt !== b.song.lastPracticedAt) {
        // '' (never) is lexicographically less than any YYYY-MM-DD date.
        return a.song.lastPracticedAt < b.song.lastPracticedAt ? -1 : 1;
      }
      return a.key - b.key;
    })
    .slice(0, count)
    .map(({song}) => song.id);
}

// loadPracticeState reads persisted practice-tool settings, falling back to
// defaults when nothing is stored or the stored value is malformed.
export function loadPracticeState(): PracticeState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_STATE;
    const p = JSON.parse(raw) as Partial<PracticeState>;
    const g = p.generated;
    return {
      source: typeof p.source === 'string' ? p.source : DEFAULT_STATE.source,
      mode: p.mode === 'band' ? 'band' : 'personal',
      count:
        typeof p.count === 'number' && p.count >= 1 && p.count <= 10
          ? p.count
          : DEFAULT_STATE.count,
      generated:
        g && typeof g.source === 'string' && Array.isArray(g.list)
          ? {
              source: g.source,
              mode: g.mode === 'band' ? 'band' : 'personal',
              list: g.list.filter((n): n is number => typeof n === 'number'),
            }
          : null,
    };
  } catch {
    return DEFAULT_STATE;
  }
}

// savePracticeState persists practice-tool settings for the next visit.
export function savePracticeState(state: PracticeState): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // ignore storage failures (private mode, quota exceeded)
  }
}
