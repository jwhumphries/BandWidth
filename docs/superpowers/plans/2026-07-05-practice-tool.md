# Practice Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `/practice` page that suggests the least-recently-practiced songs from a chosen source (all songs, a folder, a band, or a band folder), with a per-row Practiced/Rehearsed toggle and a suggestion that survives navigation.

**Architecture:** Purely front-end. Ranking is a pure function over lists already fetched by existing hooks (`useSongs`, `useFolders`, `useBands`, `useBandSongs`, `useBandFolders`). The current suggestion and controls live in `localStorage`. The Practiced/Rehearsed button calls the existing personal-practice and band-rehearsal endpoints. Done-state is derived live (`lastPracticedAt === localToday()`), so it needs no separate stored set. The only shared-code change is adding an optional `enabled` flag to two band hooks (mirroring the existing `useBandInvites(bandId, enabled)`), so the page can subscribe to band data only when a band source is selected.

**Tech Stack:** React, TypeScript, React Router, TanStack Query, Tailwind v4 + DaisyUI 5, `lucide-react`; Vitest + Testing Library (jsdom) for tests; Dagger via `just` for all verification.

## Global Constraints

- **No backend/model/API changes.** Every endpoint and hook already exists. The only source change outside the new files is adding an optional `enabled = true` parameter to `useBandSongs` and `useBandFolders`.
- **All verification runs through `just` (Dagger), never the host toolchain:** `just test-frontend`, `just typecheck`, `just lint-js`, `just format-check`, `just check`.
- **Match existing code style exactly:** single quotes, semicolons, trailing commas (Prettier-formatted); import order = external packages alphabetical, then relative paths alphabetical (see `pages/HomePage.tsx:1-9` for the reference pattern); `import type {…}` for type-only imports.
- **Tests mock `fetch` via `vi.stubGlobal` and render through `renderWithProviders` from `frontend/src/test/utils.tsx`** (never mock the hooks themselves) — follow `pages/BandsPage.test.tsx` as the reference.
- **Song ranking rule:** ascending by `lastPracticedAt` (a `YYYY-MM-DD` string, or `''` = never), so `''` sorts first (highest priority); ties broken randomly.
- **Practice dates use `localToday()` from `frontend/src/lib/dates.ts`** (the musician's local calendar day).

---

### Task 1: Add optional `enabled` flag to the two band hooks

**Files:**
- Modify: `frontend/src/hooks/bandsongs.ts` (the `useBandSongs` function)
- Modify: `frontend/src/hooks/bandfolders.ts` (the `useBandFolders` function)

**Interfaces:**
- Consumes: nothing new.
- Produces: `useBandSongs(bandId: number, enabled?: boolean)` and `useBandFolders(bandId: number, enabled?: boolean)`, both defaulting `enabled` to `true`. Later tasks pass `enabled` to gate fetches. All existing call sites (which pass only `bandId`) keep working unchanged.

- [ ] **Step 1: Add `enabled` to `useBandSongs`**

In `frontend/src/hooks/bandsongs.ts`, replace the `useBandSongs` function:

```ts
export function useBandSongs(bandId: number, enabled = true) {
  return useQuery<SongListItem[], ApiError>({
    queryKey: ['bands', bandId, 'songs'],
    queryFn: () => api.get<SongListItem[]>(`/api/bands/${bandId}/songs`),
    enabled,
  });
}
```

- [ ] **Step 2: Add `enabled` to `useBandFolders`**

In `frontend/src/hooks/bandfolders.ts`, replace the `useBandFolders` function:

```ts
export function useBandFolders(bandId: number, enabled = true) {
  return useQuery<Folder[], ApiError>({
    queryKey: ['bands', bandId, 'folders'],
    queryFn: () => api.get<Folder[]>(`/api/bands/${bandId}/folders`),
    enabled,
  });
}
```

- [ ] **Step 3: Run the existing suites + typecheck to confirm no regressions**

Run: `just test-frontend && just typecheck`
Expected: PASS — the added parameter is optional, so `BandPage`, `BandSongPage`, `BandFolderSidebar`, etc. compile and their tests stay green.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/hooks/bandsongs.ts frontend/src/hooks/bandfolders.ts
git commit -m "Add optional enabled flag to band song/folder hooks"
```

---

### Task 2: Practice logic + persistence (`lib/practice.ts`)

**Files:**
- Create: `frontend/src/lib/practice.ts`
- Test: `frontend/src/lib/practice.test.ts`

**Interfaces:**
- Consumes: `Folder`, `SongListItem`, `SongStatus` from `frontend/src/lib/types.ts`.
- Produces (all imported by Tasks 4 & 5):
  - Types: `PracticeMode = 'personal' | 'band'`; `ParsedSource`; `SourceData`; `GeneratedList`; `PracticeState`.
  - `parseSource(value: string): ParsedSource`
  - `isBandScoped(value: string): boolean`
  - `effectiveMode(value: string, mode: PracticeMode): PracticeMode`
  - `resolveCandidates(parsed: ParsedSource, mode: PracticeMode, data: SourceData): SongListItem[]`
  - `suggest(candidates: SongListItem[], count: number, rng?: () => number): number[]`
  - `loadPracticeState(): PracticeState` and `savePracticeState(state: PracticeState): void`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/practice.test.ts`:

```ts
import {beforeEach, describe, expect, it} from 'vitest';
import type {Folder, SongListItem} from './types';
import {
  effectiveMode,
  isBandScoped,
  loadPracticeState,
  parseSource,
  resolveCandidates,
  savePracticeState,
  suggest,
} from './practice';

function song(id: number, lastPracticedAt: string, bandId?: number): SongListItem {
  return {
    id,
    title: `Song ${id}`,
    artist: 'A',
    status: 'learning',
    lastPracticedAt,
    practiceCount: 0,
    ...(bandId ? {bandId, bandName: 'Band'} : {}),
  };
}

const folder = (id: number, songIds: number[]): Folder => ({
  id,
  name: `F${id}`,
  position: id,
  songIds,
});

describe('parseSource', () => {
  it('decodes every source shape and falls back to all', () => {
    expect(parseSource('all')).toEqual({kind: 'all'});
    expect(parseSource('folder:7')).toEqual({kind: 'folder', folderId: 7});
    expect(parseSource('band:3')).toEqual({kind: 'band', bandId: 3});
    expect(parseSource('bandfolder:3:9')).toEqual({
      kind: 'bandfolder',
      bandId: 3,
      folderId: 9,
    });
    expect(parseSource('garbage')).toEqual({kind: 'all'});
  });
});

describe('isBandScoped / effectiveMode', () => {
  it('treats band and bandfolder as band-scoped only', () => {
    expect(isBandScoped('all')).toBe(false);
    expect(isBandScoped('folder:1')).toBe(false);
    expect(isBandScoped('band:2')).toBe(true);
    expect(isBandScoped('bandfolder:2:3')).toBe(true);
  });

  it('forces personal mode for non-band sources', () => {
    expect(effectiveMode('all', 'band')).toBe('personal');
    expect(effectiveMode('band:2', 'band')).toBe('band');
    expect(effectiveMode('band:2', 'personal')).toBe('personal');
  });
});

describe('suggest', () => {
  const rng = () => 0.5; // deterministic tie-break

  it('ranks never-practiced first, then oldest date, capped at count', () => {
    const candidates = [
      song(1, '2026-05-01'),
      song(2, ''),
      song(3, '2026-01-01'),
    ];
    expect(suggest(candidates, 2, rng)).toEqual([2, 3]);
  });

  it('returns all when fewer than count', () => {
    expect(suggest([song(1, ''), song(2, '2026-01-01')], 5, rng)).toEqual([
      1, 2,
    ]);
  });

  it('randomly picks among a tie group via the injected rng', () => {
    const tied = [song(1, ''), song(2, ''), song(3, '')];
    // rng returns ascending keys -> stable order 1,2,3; take first 2.
    let n = 0;
    const asc = () => (n++ % 3) / 3;
    expect(suggest(tied, 2, asc)).toEqual([1, 2]);
    // rng returns descending keys -> order reverses; take first 2.
    let m = 0;
    const desc = () => (2 - (m++ % 3)) / 3;
    expect(suggest(tied, 2, desc)).toEqual([3, 2]);
  });
});

describe('resolveCandidates', () => {
  const data = {
    songs: [song(1, ''), song(2, '2026-01-01', 3), song(3, '2026-02-01', 3)],
    folders: [folder(10, [1, 3])],
    bandSongs: [song(2, '2026-06-01'), song(3, '2026-06-02')],
    bandFolders: [folder(20, [3])],
  };

  it('all -> whole personal library', () => {
    expect(resolveCandidates({kind: 'all'}, 'personal', data).map(s => s.id)).toEqual([
      1, 2, 3,
    ]);
  });

  it('folder -> personal songs in the folder', () => {
    expect(
      resolveCandidates({kind: 'folder', folderId: 10}, 'personal', data).map(
        s => s.id,
      ),
    ).toEqual([1, 3]);
  });

  it('band + personal -> personal rows for that band', () => {
    const out = resolveCandidates({kind: 'band', bandId: 3}, 'personal', data);
    expect(out.map(s => s.id)).toEqual([2, 3]);
    expect(out[0].lastPracticedAt).toBe('2026-01-01'); // personal date
  });

  it('band + band -> band song list', () => {
    const out = resolveCandidates({kind: 'band', bandId: 3}, 'band', data);
    expect(out[0].lastPracticedAt).toBe('2026-06-01'); // band rehearsal date
  });

  it('bandfolder honors mode for the ranking source', () => {
    expect(
      resolveCandidates({kind: 'bandfolder', bandId: 3, folderId: 20}, 'personal', data)
        .map(s => s.id),
    ).toEqual([3]);
    expect(
      resolveCandidates({kind: 'bandfolder', bandId: 3, folderId: 20}, 'band', data)[0]
        .lastPracticedAt,
    ).toBe('2026-06-02');
  });
});

describe('load/save practice state', () => {
  beforeEach(() => localStorage.clear());

  it('returns defaults when nothing is stored', () => {
    expect(loadPracticeState()).toEqual({
      source: 'all',
      mode: 'personal',
      count: 3,
      generated: null,
    });
  });

  it('round-trips a saved state', () => {
    const state = {
      source: 'band:3',
      mode: 'band' as const,
      count: 5,
      generated: {source: 'band:3', mode: 'band' as const, list: [2, 3]},
    };
    savePracticeState(state);
    expect(loadPracticeState()).toEqual(state);
  });

  it('falls back to defaults on malformed storage', () => {
    localStorage.setItem('bandwidth:practice', '{not json');
    expect(loadPracticeState()).toEqual({
      source: 'all',
      mode: 'personal',
      count: 3,
      generated: null,
    });
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — `frontend/src/lib/practice.ts` does not exist (module not found).

- [ ] **Step 3: Write the implementation**

Create `frontend/src/lib/practice.ts`:

```ts
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

export function savePracticeState(state: PracticeState): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // ignore storage failures (private mode, quota exceeded)
  }
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `just test-frontend`
Expected: PASS — all `practice.test.ts` cases green, no other suite affected.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/practice.ts frontend/src/lib/practice.test.ts
git commit -m "Add practice suggestion logic and persistence"
```

---

### Task 3: `PracticeRow` component

**Files:**
- Create: `frontend/src/components/practice/PracticeRow.tsx`
- Test: `frontend/src/components/practice/PracticeRow.test.tsx`

**Interfaces:**
- Consumes: `SongListItem` from `lib/types.ts`; `StatusBadge` from `components/songs/StatusBadge.tsx` (props `{status}`).
- Produces: default export
  `PracticeRow({song, linkTo, done, actionLabel, canAct, onToggle}: {song: SongListItem; linkTo: string; done: boolean; actionLabel: string; canAct: boolean; onToggle: () => void})`.
  Task 5 renders one per suggested song. `onToggle` fires on the button; the parent decides log vs. undo based on `done`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/practice/PracticeRow.test.tsx`:

```tsx
import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import type {SongListItem} from '../../lib/types';
import PracticeRow from './PracticeRow';

const song: SongListItem = {
  id: 1,
  title: 'Wonderwall',
  artist: 'Oasis',
  status: 'learning',
  lastPracticedAt: '2026-01-01',
  practiceCount: 2,
};

describe('PracticeRow', () => {
  it('shows the action label and fires onToggle when not done', async () => {
    const onToggle = vi.fn();
    renderWithProviders(
      <ul>
        <PracticeRow
          song={song}
          linkTo="/songs/1"
          done={false}
          actionLabel="Practiced"
          canAct
          onToggle={onToggle}
        />
      </ul>,
    );
    const button = screen.getByRole('button', {name: /practiced/i});
    await userEvent.click(button);
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('shows an Undo button when done', () => {
    renderWithProviders(
      <ul>
        <PracticeRow
          song={song}
          linkTo="/songs/1"
          done
          actionLabel="Practiced"
          canAct
          onToggle={vi.fn()}
        />
      </ul>,
    );
    expect(screen.getByRole('button', {name: /undo/i})).toBeInTheDocument();
  });

  it('disables the button when the user cannot act', () => {
    renderWithProviders(
      <ul>
        <PracticeRow
          song={song}
          linkTo="/songs/1"
          done={false}
          actionLabel="Rehearsed"
          canAct={false}
          onToggle={vi.fn()}
        />
      </ul>,
    );
    expect(screen.getByRole('button', {name: /rehearsed/i})).toBeDisabled();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — `./PracticeRow` module not found.

- [ ] **Step 3: Write the implementation**

Create `frontend/src/components/practice/PracticeRow.tsx`:

```tsx
import {CircleCheck, Clock, RotateCcw} from 'lucide-react';
import {Link} from 'react-router';
import type {SongListItem} from '../../lib/types';
import StatusBadge from '../songs/StatusBadge';

export default function PracticeRow({
  song,
  linkTo,
  done,
  actionLabel,
  canAct,
  onToggle,
}: {
  song: SongListItem;
  linkTo: string;
  done: boolean;
  actionLabel: string;
  canAct: boolean;
  onToggle: () => void;
}) {
  return (
    <li
      className={`group border-base-300/60 bg-base-100 hover:border-base-300 flex items-center gap-3 rounded-box border p-3 transition-all hover:shadow-md sm:gap-4 sm:p-4 ${
        done ? 'opacity-60' : ''
      }`}
    >
      <Link to={linkTo} className="min-w-0 flex-1">
        <span
          className={`font-display group-hover:text-primary block truncate text-base font-semibold transition-colors ${
            done ? 'line-through' : ''
          }`}
        >
          {song.title}
        </span>
        <span className="text-base-content/55 block truncate text-sm">
          {song.artist || '—'}
        </span>
      </Link>

      <div className="flex shrink-0 items-center gap-2 sm:gap-3">
        <StatusBadge status={song.status} />
        <span className="text-base-content/45 hidden w-28 items-center justify-end gap-1 font-mono text-xs sm:flex">
          <Clock className="size-3 shrink-0" />
          {song.lastPracticedAt || 'never'}
        </span>
        <button
          type="button"
          className={`btn btn-sm gap-1.5 ${done ? 'btn-ghost' : 'btn-primary'}`}
          onClick={onToggle}
          disabled={!canAct}
          title={canAct ? undefined : 'Viewers cannot log band rehearsals'}
        >
          {done ? (
            <RotateCcw className="size-4" />
          ) : (
            <CircleCheck className="size-4" />
          )}
          {done ? 'Undo' : actionLabel}
        </button>
      </div>
    </li>
  );
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `just test-frontend`
Expected: PASS — all three `PracticeRow` cases green.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/practice/PracticeRow.tsx frontend/src/components/practice/PracticeRow.test.tsx
git commit -m "Add PracticeRow component"
```

---

### Task 4: `PracticeSourcePicker` component

**Files:**
- Create: `frontend/src/components/practice/PracticeSourcePicker.tsx`
- Test: `frontend/src/components/practice/PracticeSourcePicker.test.tsx`

**Interfaces:**
- Consumes: `useFolders()` (`hooks/folders.ts`), `useBands()` (`hooks/bands.ts`), `useBandFolders(bandId)` (`hooks/bandfolders.ts`); `BandSummary` from `lib/types.ts`.
- Produces: default export `PracticeSourcePicker({value, onChange}: {value: string; onChange: (value: string) => void})`. Emits source values `all` / `folder:<id>` / `band:<id>` / `bandfolder:<bandId>:<folderId>`. Task 5 mounts it and feeds `value`/`onChange` from state.

Note: each band's folders load via a small `BandOptgroup` child so `useBandFolders` is called once per band (hooks can't be called in a `.map` over bands otherwise).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/practice/PracticeSourcePicker.test.tsx`:

```tsx
import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import PracticeSourcePicker from './PracticeSourcePicker';

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

const folders = [{id: 1, name: 'Warmups', position: 1, songIds: []}];
const bands = [
  {id: 5, name: 'The Cure', creatorId: 1, role: 'editor', memberCount: 2},
];
const bandFolders = [{id: 9, name: 'Setlist', position: 1, songIds: []}];

describe('PracticeSourcePicker', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/api/bands/5/folders')) return Promise.resolve(json(200, bandFolders));
        if (url.endsWith('/api/bands')) return Promise.resolve(json(200, bands));
        if (url.endsWith('/api/folders')) return Promise.resolve(json(200, folders));
        return Promise.resolve(json(200, []));
      }),
    );
  });

  it('renders every source group and emits encoded values', async () => {
    const onChange = vi.fn();
    renderWithProviders(<PracticeSourcePicker value="all" onChange={onChange} />);

    expect(screen.getByRole('option', {name: 'All Songs'})).toBeInTheDocument();
    expect(screen.getByRole('option', {name: 'Warmups'})).toBeInTheDocument();
    expect(
      await screen.findByRole('option', {name: 'All The Cure songs'}),
    ).toBeInTheDocument();
    expect(await screen.findByRole('option', {name: 'Setlist'})).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByRole('combobox'), 'bandfolder:5:9');
    expect(onChange).toHaveBeenCalledWith('bandfolder:5:9');
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — `./PracticeSourcePicker` module not found.

- [ ] **Step 3: Write the implementation**

Create `frontend/src/components/practice/PracticeSourcePicker.tsx`:

```tsx
import {useBandFolders} from '../../hooks/bandfolders';
import {useBands} from '../../hooks/bands';
import {useFolders} from '../../hooks/folders';
import type {BandSummary} from '../../lib/types';

function BandOptgroup({band}: {band: BandSummary}) {
  const {data: folders = []} = useBandFolders(band.id);
  return (
    <optgroup label={band.name}>
      <option value={`band:${band.id}`}>All {band.name} songs</option>
      {folders.map(f => (
        <option key={f.id} value={`bandfolder:${band.id}:${f.id}`}>
          {f.name}
        </option>
      ))}
    </optgroup>
  );
}

export default function PracticeSourcePicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const {data: folders = []} = useFolders();
  const {data: bands = []} = useBands();
  return (
    <select
      className="select"
      aria-label="Source"
      value={value}
      onChange={e => onChange(e.target.value)}
    >
      <option value="all">All Songs</option>
      {folders.length > 0 && (
        <optgroup label="Folders">
          {folders.map(f => (
            <option key={f.id} value={`folder:${f.id}`}>
              {f.name}
            </option>
          ))}
        </optgroup>
      )}
      {bands.map(band => (
        <BandOptgroup key={band.id} band={band} />
      ))}
    </select>
  );
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `just test-frontend`
Expected: PASS — the picker renders all groups and emits `bandfolder:5:9`.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/practice/PracticeSourcePicker.tsx frontend/src/components/practice/PracticeSourcePicker.test.tsx
git commit -m "Add PracticeSourcePicker component"
```

---

### Task 5: `PracticePage`

**Files:**
- Create: `frontend/src/pages/PracticePage.tsx`
- Test: `frontend/src/pages/PracticePage.test.tsx`

**Interfaces:**
- Consumes: everything from Tasks 2–4 plus existing hooks — `useSongs`, `useLogPractice`, `useUndoPractice` (`hooks/songs.ts`); `useBands` (`hooks/bands.ts`); `useFolders` (`hooks/folders.ts`); `useBandSongs`, `useLogBandRehearsalInList`, `useUndoBandRehearsalInList` (`hooks/bandsongs.ts`); `useBandFolders` (`hooks/bandfolders.ts`); `localToday` (`lib/dates.ts`); `ConfirmModal` (`components/songs/ConfirmModal.tsx`).
- Produces: default export `PracticePage` — a route component (no props). Task 6 wires it to `/practice`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/pages/PracticePage.test.tsx`:

```tsx
import {cleanup, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {localToday} from '../lib/dates';
import {renderWithProviders} from '../test/utils';
import PracticePage from './PracticePage';

const today = localToday();

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

const songs = [
  {id: 1, title: 'Alpha', artist: 'A', status: 'learning', lastPracticedAt: '', practiceCount: 0},
  {id: 2, title: 'Bravo', artist: 'B', status: 'learned', lastPracticedAt: '2026-01-01', practiceCount: 3},
  {id: 3, title: 'Charlie', artist: 'C', status: 'nailed', lastPracticedAt: today, practiceCount: 5},
];

function stubFetch() {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/api/songs/') && init?.method === 'PUT') {
        return Promise.resolve(json(200, {lastPracticedAt: today, practiceCount: 1}));
      }
      if (url.includes('/api/songs/') && init?.method === 'DELETE') {
        return Promise.resolve(json(200, {lastPracticedAt: '', practiceCount: 0}));
      }
      if (url.endsWith('/api/songs')) return Promise.resolve(json(200, songs));
      return Promise.resolve(json(200, [])); // /api/folders, /api/bands
    }),
  );
}

function rowFor(title: string) {
  return screen.getByText(title).closest('li') as HTMLElement;
}

describe('PracticePage', () => {
  beforeEach(() => {
    localStorage.clear();
    stubFetch();
  });

  it('suggests least-recently-practiced songs and marks today as done', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));

    // Never-practiced Alpha first, then Bravo; Charlie (today) shows as done.
    expect(await screen.findByText('Alpha')).toBeInTheDocument();
    expect(within(rowFor('Alpha')).getByRole('button', {name: /practiced/i})).toBeInTheDocument();
    expect(within(rowFor('Charlie')).getByRole('button', {name: /undo/i})).toBeInTheDocument();
  });

  it('logs practice with today when a suggested song is tapped', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));
    await screen.findByText('Alpha');

    await userEvent.click(within(rowFor('Alpha')).getByRole('button', {name: /practiced/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/api/songs/1/practice') &&
            init?.method === 'PUT' &&
            String(init.body).includes(today),
        ),
      ).toBe(true);
    });
  });

  it('confirms before undoing an already-practiced song', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));
    await screen.findByText('Charlie');

    await userEvent.click(within(rowFor('Charlie')).getByRole('button', {name: /undo/i}));
    // Confirm modal opens; confirm it.
    await userEvent.click(
      within(screen.getByRole('dialog')).getByRole('button', {name: /undo/i}),
    );
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes(`/api/songs/3/practice/${today}`) &&
            init?.method === 'DELETE',
        ),
      ).toBe(true);
    });
  });

  it('restores the previous suggestion on remount without regenerating', async () => {
    renderWithProviders(<PracticePage />);
    await userEvent.click(screen.getByRole('button', {name: /suggest songs/i}));
    await screen.findByText('Alpha');

    cleanup();
    renderWithProviders(<PracticePage />);
    // No Suggest click this time — list comes back from localStorage.
    expect(await screen.findByText('Alpha')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — `./PracticePage` module not found.

- [ ] **Step 3: Write the implementation**

Create `frontend/src/pages/PracticePage.tsx`:

```tsx
import {Dumbbell, Sparkles} from 'lucide-react';
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
  const {data: bandSongs = []} = useBandSongs(bandId, bandId > 0 && mode === 'band');
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
    setState(s => ({...s, generated: {source: s.source, mode: controlMode, list}}));
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
  }, [gen, genParsed, genMode, songs, folders, genBand.bandSongs, genBand.bandFolders]);

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
          <Dumbbell className="text-primary size-6" />
          Practice
        </h1>
        <p className="text-base-content/55 text-sm">
          Suggests the songs you haven’t practiced in the longest.
        </p>
      </div>

      <div className="border-base-300/60 bg-base-100 flex flex-wrap items-end gap-3 rounded-box border p-4">
        <label className="flex flex-col gap-1">
          <span className="text-base-content/60 text-xs font-medium">Source</span>
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
          <span className="text-base-content/60 text-xs font-medium">Songs</span>
          <select
            className="select"
            aria-label="Number of songs"
            value={state.count}
            onChange={e => setState(s => ({...s, count: Number(e.target.value)}))}
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
                onToggle={() => (done ? setPendingUndo(song.id) : logFor(song.id))}
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `just test-frontend`
Expected: PASS — all four `PracticePage` cases green.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/PracticePage.tsx frontend/src/pages/PracticePage.test.tsx
git commit -m "Add PracticePage"
```

---

### Task 6: Wire the route and the nav item

**Files:**
- Modify: `frontend/src/App.tsx` (import + route)
- Modify: `frontend/src/components/Layout.tsx` (import icon + nav item)
- Test: `frontend/src/components/Layout.test.tsx`

**Interfaces:**
- Consumes: `PracticePage` from Task 5; `Dumbbell` from `lucide-react`; the existing `NavItem` component inside `Layout.tsx`.
- Produces: a reachable `/practice` route and a "Practice" nav link. Terminal integration point.

- [ ] **Step 1: Write the failing nav test**

Create `frontend/src/components/Layout.test.tsx`:

```tsx
import {screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import Layout from './Layout';

describe('Layout', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith('/api/me')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                id: 1,
                username: 'jo',
                email: 'jo@x.com',
                totpEnabled: false,
                isAdmin: false,
              }),
              {status: 200},
            ),
          );
        }
        // /api/invites
        return Promise.resolve(new Response(JSON.stringify([]), {status: 200}));
      }),
    );
  });

  it('shows a Practice nav link pointing at /practice', () => {
    renderWithProviders(<Layout />);
    const link = screen.getByRole('link', {name: /practice/i});
    expect(link).toHaveAttribute('href', '/practice');
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — no "Practice" link exists in `Layout` yet.

- [ ] **Step 3: Add the route in `App.tsx`**

Add the import alongside the other page imports (alphabetical — after `LoginPage`, before `ProfilePage`):

```tsx
import PracticePage from './pages/PracticePage';
```

Add the route inside the `<Layout />` route block, after the `/` route and before `/profile`:

```tsx
              <Route path="/practice" element={<PracticePage />} />
```

- [ ] **Step 4: Add the nav item in `Layout.tsx`**

Add `Dumbbell` to the existing `lucide-react` import (keep the list alphabetical):

```tsx
import {
  AudioLines,
  Dumbbell,
  LibraryBig,
  LogOut,
  Moon,
  Shield,
  Sun,
  User,
  Users,
} from 'lucide-react';
```

Insert a new `NavItem` immediately after the Library `NavItem` and before the Bands `NavItem`:

```tsx
            <NavItem
              to="/practice"
              icon={<Dumbbell className="size-4" />}
              label="Practice"
            />
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `just test-frontend`
Expected: PASS — `Layout.test.tsx` finds the `/practice` link, and no existing suite regresses.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/Layout.tsx frontend/src/components/Layout.test.tsx
git commit -m "Add Practice route and nav item"
```

---

### Task 7: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full check suite**

Run: `just check`
Expected: `lint-go`, `lint-js`, `typecheck`, `test-go`, `test-frontend`, and `format-check` all pass.

- [ ] **Step 2: If `format-check` fails, format and commit**

Run: `just format`
Then:

```bash
git add -A
git commit -m "Format practice tool"
```

(Skip entirely if `format-check` already passed in Step 1.)
