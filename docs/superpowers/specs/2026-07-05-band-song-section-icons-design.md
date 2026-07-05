# Band song page: section icons + Folders section

## Problem

The user library song view (`SongPage.tsx`) shows a `lucide-react` icon on
each section header: `Repeat` for Practice, `Link2` for Links, `FolderTree`
for Folders. The band song view (`BandSongPage.tsx`) has no icons on its
equivalent sections, and has no Folders section at all — even though the
backend already supports band-owned folders and band-song-to-band-folder
associations.

## Goals

- Match the library view's icon treatment on the band song view's existing
  section headers.
- Add a Folders section to the band song view so band members can assign a
  band song to one or more band folders, mirroring the library view's
  folder picker.
- No backend changes — all required endpoints and hooks already exist.

## Design

### 1. Icons on existing headers

In `frontend/src/pages/BandSongPage.tsx`, add:

```tsx
import {FolderTree, Link2, Repeat} from 'lucide-react';
```

- "Rehearsals" header → prepend `<Repeat className="text-primary size-5" />`
- "Band links" header → prepend `<Link2 className="text-primary size-5" />`

Same pattern as `SongPage.tsx`'s `<h2 className="card-title">` sections.

### 2. New `BandFolderPicker` component

New file: `frontend/src/components/bands/BandFolderPicker.tsx`, modeled on
`frontend/src/components/folders/FolderPicker.tsx` but parameterized by
`bandId` instead of assuming the personal-library context:

```tsx
export default function BandFolderPicker({
  bandId,
  songId,
  canEdit,
}: {
  bandId: number;
  songId: number;
  canEdit: boolean;
})
```

- Uses `useBandFolders(bandId)` / `useSetBandFolderEntries(bandId)` from
  `frontend/src/hooks/bandfolders.ts` (already implemented) instead of the
  personal `useFolders()` / `useSetFolderEntries()`.
- Same checkbox-list rendering as `FolderPicker`, but the checkbox is
  `disabled={!canEdit}` so viewers see membership read-only while
  Editor+ can toggle it (consistent with the read/write gating already
  used elsewhere on `BandSongPage`).
- Empty state: "No folders yet — create one from the band page." (band
  folders are created via `BandFolderSidebar`, not this picker).

### 3. Mounting in `BandSongPage.tsx`

Add a new `<section className="card bg-base-100 shadow">` between the
"Band links" section and "Danger zone", with a `<FolderTree>`-iconed
"Folders" header, containing:

```tsx
<BandFolderPicker bandId={bandId} songId={songId} canEdit={canEdit} />
```

## Out of scope

- No changes to `BandFolderSidebar.tsx` (band folder create/rename/reorder
  UI is unaffected).
- No icon added to "Danger zone" — not requested, and not present in the
  library view's danger zone as a header icon either (it's icon-only on
  the button, `Trash2`, so left untouched here for consistency with what
  was actually asked).
- No backend changes; `useBandFolders` / `useSetBandFolderEntries` already
  exist and are used as-is.

## Testing

- Manual check in browser: band song page shows icons on Rehearsals and
  Band links; Folders section appears, checkboxes toggle band folder
  membership for Editor+ members, and are disabled (but reflect state) for
  Viewers.
- Existing frontend test suite (if any covers `BandSongPage`/`FolderPicker`)
  should still pass; add/update tests for `BandFolderPicker` alongside the
  existing `FolderPicker` tests if such tests exist.
