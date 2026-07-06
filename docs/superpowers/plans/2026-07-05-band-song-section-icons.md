# Band Song Page: Section Icons + Folders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the band song page (`BandSongPage.tsx`) the same section icons as the user library song page, and add a Folders section so band members can assign a band song to band folders.

**Architecture:** Frontend-only, additive changes. Reuse the existing `lucide-react` icon set and the existing band-folder backend/hooks (`useBandFolders`, `useSetBandFolderEntries`) that already power `BandFolderSidebar`. Add one new small component, `BandFolderPicker`, that mirrors the personal-library `FolderPicker` but is parameterized by `bandId` and a `canEdit` flag (band folders are Editor+ writable, Viewer readable).

**Tech Stack:** React, TypeScript, `lucide-react`, TanStack Query hooks (already implemented), Vitest + Testing Library for tests, Dagger via `just` for all verification.

## Global Constraints

- No backend changes — `useBandFolders(bandId)` / `useSetBandFolderEntries(bandId)` (`frontend/src/hooks/bandfolders.ts`) already exist and are already used by `BandFolderSidebar`.
- All verification commands run through `just` (Dagger), not the host toolchain directly: `just test-frontend`, `just typecheck`, `just lint-js`, `just format-check`, `just check`.
- Match existing code style exactly: single quotes, semicolons, trailing commas (Prettier-formatted), import order external-packages-alphabetical then relative-paths-alphabetical (see `SongPage.tsx:1-16` for the reference pattern).
- Viewers (`canEdit === false`) must see folder membership but not be able to toggle it — checkboxes render `disabled` for them, matching the read-only treatment already used elsewhere on `BandSongPage.tsx` (e.g. no status `<select>`, no delete button for viewers).

---

### Task 1: Section icons on existing `BandSongPage.tsx` headers

**Files:**
- Modify: `frontend/src/pages/BandSongPage.tsx:1` (imports), `:186` (Rehearsals header), `:206` (Band links header)

**Interfaces:**
- Consumes: `lucide-react` exports `Repeat`, `Link2` (also `FolderTree`, needed by Task 3 — import all three now to avoid touching the import line twice).
- Produces: no new exports; purely presentational change to this file's JSX.

- [ ] **Step 1: Add the lucide-react import**

At the top of `frontend/src/pages/BandSongPage.tsx`, before the existing `import {useEffect, useState} from 'react';` line, add:

```tsx
import {FolderTree, Link2, Repeat} from 'lucide-react';
```

- [ ] **Step 2: Add the icon to the "Rehearsals" header**

Find (currently line 186):

```tsx
          <h2 className="card-title">Rehearsals</h2>
```

Replace with:

```tsx
          <h2 className="card-title">
            <Repeat className="text-primary size-5" />
            Rehearsals
          </h2>
```

- [ ] **Step 3: Add the icon to the "Band links" header**

Find (currently line 206):

```tsx
          <h2 className="card-title">Band links</h2>
```

Replace with:

```tsx
          <h2 className="card-title">
            <Link2 className="text-primary size-5" />
            Band links
          </h2>
```

- [ ] **Step 4: Run the existing frontend test suite to confirm no regressions**

Run: `just test-frontend`
Expected: all tests pass, including the existing `BandSongPage.test.tsx` suite (icons are presentational and untested by name elsewhere in this codebase — `SongPage.test.tsx` doesn't assert on its icons either, so no new assertions are expected here).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/BandSongPage.tsx
git commit -m "Add section icons to band song page headers"
```

---

### Task 2: `BandFolderPicker` component (TDD)

**Files:**
- Create: `frontend/src/components/bands/BandFolderPicker.tsx`
- Test: `frontend/src/components/bands/BandFolderPicker.test.tsx`

**Interfaces:**
- Consumes: `useBandFolders(bandId: number)` and `useSetBandFolderEntries(bandId: number)` from `frontend/src/hooks/bandfolders.ts` (already implemented — `useBandFolders` returns `{data: Folder[] | undefined, ...}`; `useSetBandFolderEntries` returns a mutation whose `.mutate` takes `{id: number; songIds: number[]}`). `Folder` type from `frontend/src/lib/types.ts:73-78` (`{id: number; name: string; position: number; songIds: number[]}`).
- Produces: default export `BandFolderPicker({bandId, songId, canEdit}: {bandId: number; songId: number; canEdit: boolean})` — a React component. Task 3 mounts this directly.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/bands/BandFolderPicker.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import BandFolderPicker from './BandFolderPicker';

const folders = [
  {id: 1, name: 'Setlist', position: 1, songIds: [7]},
  {id: 2, name: 'Queue', position: 2, songIds: []},
];

describe('BandFolderPicker', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
          if (init?.method === 'PUT') {
            return Promise.resolve(new Response(null, {status: 204}));
          }
          return Promise.resolve(
            new Response(JSON.stringify(folders), {status: 200}),
          );
        }),
    );
  });

  it('checks folders containing the song and toggles membership when editable', async () => {
    renderWithProviders(<BandFolderPicker bandId={3} songId={7} canEdit />);
    const setlist = await screen.findByLabelText('Setlist');
    const queue = screen.getByLabelText('Queue');
    expect(setlist).toBeChecked();
    expect(queue).not.toBeChecked();
    expect(setlist).not.toBeDisabled();

    await userEvent.click(queue);
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/api/bands/3/folders/2/entries') &&
            init?.method === 'PUT' &&
            String(init.body).includes('7'),
        ),
      ).toBe(true);
    });
  });

  it('disables checkboxes for viewers', async () => {
    renderWithProviders(
      <BandFolderPicker bandId={3} songId={7} canEdit={false} />,
    );
    const setlist = await screen.findByLabelText('Setlist');
    expect(setlist).toBeChecked();
    expect(setlist).toBeDisabled();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — `frontend/src/components/bands/BandFolderPicker.tsx` does not exist yet (module not found).

- [ ] **Step 3: Write the implementation**

Create `frontend/src/components/bands/BandFolderPicker.tsx`:

```tsx
import {useBandFolders, useSetBandFolderEntries} from '../../hooks/bandfolders';

export default function BandFolderPicker({
  bandId,
  songId,
  canEdit,
}: {
  bandId: number;
  songId: number;
  canEdit: boolean;
}) {
  const {data: folders = []} = useBandFolders(bandId);
  const setEntries = useSetBandFolderEntries(bandId);

  if (folders.length === 0) {
    return (
      <p className="text-base-content/60 text-sm">
        No folders yet — create one from the band page.
      </p>
    );
  }

  const toggle = (folderId: number, member: boolean) => {
    const folder = folders.find(f => f.id === folderId);
    if (!folder) return;
    const songIds = member
      ? [...folder.songIds, songId]
      : folder.songIds.filter(id => id !== songId);
    setEntries.mutate({id: folderId, songIds});
  };

  return (
    <ul className="flex flex-col gap-1">
      {folders.map(f => {
        const member = f.songIds.includes(songId);
        return (
          <li key={f.id}>
            <label className="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                className="checkbox checkbox-sm"
                checked={member}
                disabled={!canEdit}
                onChange={() => toggle(f.id, !member)}
                aria-label={f.name}
              />
              <span>{f.name}</span>
            </label>
          </li>
        );
      })}
    </ul>
  );
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `just test-frontend`
Expected: PASS — both `BandFolderPicker` tests green, and no other suite regresses.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/bands/BandFolderPicker.tsx frontend/src/components/bands/BandFolderPicker.test.tsx
git commit -m "Add BandFolderPicker component"
```

---

### Task 3: Mount the Folders section in `BandSongPage.tsx`

**Files:**
- Modify: `frontend/src/pages/BandSongPage.tsx` (imports, new section between "Band links" and "Danger zone")
- Modify: `frontend/src/pages/BandSongPage.test.tsx` (`stubFetch` helper, new test case)

**Interfaces:**
- Consumes: `BandFolderPicker` from Task 2 (`{bandId, songId, canEdit}` props, all already in scope in `BandSongPage` as `bandId`, `songId`, `canEdit` locals), `FolderTree` icon (already imported in Task 1).
- Produces: nothing new for later tasks — this is the final integration point.

- [ ] **Step 1: Update `stubFetch` in `BandSongPage.test.tsx` and add the failing test**

In `frontend/src/pages/BandSongPage.test.tsx`, replace the `stubFetch` function with a version that accepts an optional `folders` list and serves it for any `/folders` GET, and serves `204` for a folder-entries `PUT`:

```tsx
function stubFetch(
  myRole: string,
  folders: Array<{id: number; name: string; position: number; songIds: number[]}> = [],
) {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes('/folders/') && init?.method === 'PUT') {
          return Promise.resolve(new Response(null, {status: 204}));
        }
        if (url.includes('/folders')) {
          return Promise.resolve(jsonResponse(200, folders));
        }
        if (init?.method === 'PATCH') {
          return Promise.resolve(
            jsonResponse(200, {...detail, status: 'nailed'}),
          );
        }
        if (init?.method === 'PUT' || init?.method === 'POST') {
          return Promise.resolve(
            jsonResponse(200, {
              lastRehearsedAt: '2026-06-11',
              rehearsalCount: 5,
            }),
          );
        }
        if (url.includes('/songs/1')) {
          return Promise.resolve(jsonResponse(200, detail));
        }
        // band detail
        return Promise.resolve(
          jsonResponse(200, {
            id: 3,
            name: 'The Quietones',
            creatorId: 1,
            myRole,
            members: [],
          }),
        );
      }),
  );
}
```

Then add this test inside `describe('BandSongPage', ...)`, after the existing two tests:

```tsx
  it('shows the folders section and toggles band folder membership for editors', async () => {
    stubFetch('admin', [{id: 10, name: 'Setlist', position: 1, songIds: []}]);
    renderPage();
    await screen.findByDisplayValue('Wonderwall');
    const setlist = await screen.findByLabelText('Setlist');
    expect(setlist).not.toBeChecked();

    await userEvent.click(setlist);
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).includes('/bands/3/folders/10/entries') &&
            init?.method === 'PUT' &&
            String(init.body).includes('1'),
        ),
      ).toBe(true);
    });
  });
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — `BandSongPage.tsx` renders no "Folders" section yet, so `screen.findByLabelText('Setlist')` times out.

- [ ] **Step 3: Mount `BandFolderPicker` in `BandSongPage.tsx`**

Add the import, alongside the other relative-component imports (alphabetically before `ConfirmModal`, since `components/bands` sorts before `components/songs`):

```tsx
import BandFolderPicker from '../components/bands/BandFolderPicker';
import ConfirmModal from '../components/songs/ConfirmModal';
```

Insert a new section between the closing `</section>` of "Band links" and the `{canEdit && (` that opens "Danger zone":

```tsx
      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">
            <FolderTree className="text-primary size-5" />
            Folders
          </h2>
          <BandFolderPicker bandId={bandId} songId={songId} canEdit={canEdit} />
        </div>
      </section>
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `just test-frontend`
Expected: PASS — all `BandSongPage.test.tsx` tests (including the two pre-existing ones and the new one) and all `BandFolderPicker.test.tsx` tests green.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/BandSongPage.tsx frontend/src/pages/BandSongPage.test.tsx
git commit -m "Add Folders section to band song page"
```

---

### Task 4: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full check suite**

Run: `just check`
Expected: `lint-go`, `lint-js`, `typecheck`, `test-go`, `test-frontend`, and `format-check` all pass.

- [ ] **Step 2: If `format-check` fails, format and commit**

Run: `just format`
Then:

```bash
git add -A
git commit -m "Format"
```

(Skip this step entirely if `format-check` already passed in Step 1.)
