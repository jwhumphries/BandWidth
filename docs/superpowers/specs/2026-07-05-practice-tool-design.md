# Practice Tool — Design

**Date:** 2026-07-05
**Status:** Approved design, pre-implementation

A **Practice Tool** that suggests which songs to practice next, ranked by how
long it's been since they were last practiced. The musician picks a source
(their whole library, a folder, a band, or a band folder), asks for a number
of songs, and gets a suggested list with a one-tap "Practiced" toggle on each
row. The current suggestion survives navigating away and coming back.

This is a **purely front-end feature**. Every list, every ranking input, and
both logging endpoints already exist; no backend, model, or API changes are
required.

## Goals

- Suggest N songs (1–10, default 3) least-recently practiced, from a chosen
  source.
- Break ties randomly, so repeated suggestions don't always surface the same
  songs among an equal-recency group.
- Let the user mark a suggested song practiced (or undo it) without leaving
  the tool.
- Persist the current suggestion loosely — it should still be there after
  navigating away and back, but it need not be durable long-term.

## Placement

- New route `<Route path="/practice">` under the existing `RequireAuth` →
  `Layout` nesting in `App.tsx`.
- New page `frontend/src/pages/PracticePage.tsx`.
- New `NavItem` in `Layout.tsx` (icon: `Dumbbell` from lucide, label
  "Practice"), placed in the primary nav row alongside Library / Bands /
  Profile.

## Controls

Rendered at the top of the page:

- **Source** — a single `<select>` with `<optgroup>`s:
  - `All Songs`
  - *Folders* group — one option per personal folder (`useFolders()`).
  - One group **per band** the user belongs to (`useBands()`), each
    containing `All <Band> songs` plus one option per band folder
    (`useBandFolders(bandId)`).
  - Option values encode the source:
    - `all`
    - `folder:<folderId>`
    - `band:<bandId>`
    - `bandfolder:<bandId>:<folderId>`
- **Mode toggle — Personal / Band** — shown **only** when the source is
  band-scoped (`band:` or `bandfolder:`). Hidden (and implicitly Personal)
  for `all` and personal folders. Default **Personal**. In **Band** mode the
  per-row action is disabled for Viewers (they cannot write the band
  rehearsal log; they can still see the band-ranked list).
- **Count** — a `<select>` of 1–10, default **3**.
- **Suggest Songs** — the button that (re)generates the list.

## Source → data resolution

The active source + mode selects which query supplies both the candidate set
and the ranking timestamp:

| Source | Mode | Candidates & ranking from |
| --- | --- | --- |
| `all` | Personal | `useSongs()` (whole personal library, incl. band songs) |
| `folder:<id>` | Personal | `useSongs()` filtered to that folder's `songIds` |
| `band:<id>` | Personal | `useSongs()` filtered to `bandId === <id>` — your personal dates |
| `band:<id>` | Band | `useBandSongs(<id>)` — band rehearsal dates |
| `bandfolder:<b>:<f>` | Personal | `useSongs()` filtered to the band folder's `songIds` — your personal dates |
| `bandfolder:<b>:<f>` | Band | `useBandSongs(<b>)` filtered to the band folder's `songIds` — band rehearsal dates |

Rationale: `SongsForUser` (`/api/songs`) ranks band songs by the member's own
practice events, while `SongsForBand` (`/api/bands/:id/songs`) ranks by the
band's rehearsal events (`practice_events WHERE band_id = ?`). Personal mode
reads the former; Band mode reads the latter. Personal folder song ids come
from `useFolders()`; band folder song ids from `useBandFolders(bandId)`.

## Suggestion algorithm

A pure function in `frontend/src/lib/practice.ts`, unit-tested with an
injectable RNG:

```
suggest(candidates: SongListItem[], count: number, rng): number[]
```

1. Rank by `lastPracticedAt` **ascending**, treating empty string (`''` =
   never practiced) as the earliest possible date, so never-practiced songs
   have the highest priority.
2. **Tie-break randomly**: assign each candidate a random secondary key
   within its equal-`lastPracticedAt` group (via the injected RNG), so both
   *which* tied songs are chosen and their *order* are randomized.
3. Take the first `count` ids. If fewer than `count` candidates exist, return
   all of them.

The result is an **ordered array of song ids** — the frozen suggestion.

## Persisted state

One `localStorage` key (e.g. `bandwidth:practice`) holding:

```ts
{ source: string; mode: 'personal' | 'band'; count: number; list: number[] }
```

- **On mount**, restore `source`, `mode`, `count`, and `list`.
- Changing the source / mode / count updates the controls (and is persisted)
  but does **not** rebuild `list`.
- **Suggest Songs** is the only action that rebuilds `list`: it runs
  `suggest()` over the current source+mode's candidates and saves.
- Rows render by looking each id up in the live query data, so titles, status,
  and dates stay fresh. An id that no longer resolves (song deleted / left the
  source) is skipped.

Because the controls and the last-generated `list` are both persisted and
restored together, the displayed list matches the controls after a reload.
The only intentional mismatch is when the user changes the dropdown/toggle
*without* clicking Suggest — the previous list stays on screen until they
regenerate (chosen "one shared list, manual regen" behavior).

## The "Practiced" toggle

The done-state is **derived, not stored**: a suggested row is **done** ⟺ its
`lastPracticedAt` (personal or band, per the active mode) equals
`localToday()`. This means a song already practiced today — whether tapped
here or from the Library — shows as done automatically, and the marks clear on
their own when the calendar day rolls over.

Each row (`frontend/src/components/practice/PracticeRow.tsx`, reusing
`StatusBadge`) shows title / artist, status, the relevant last-practiced (or
last-rehearsed) date, and a toggle button:

- **Not done → click "Practiced"/"Rehearsed":** logs today via the matching
  hook — `useLogPractice` (Personal) or `useLogBandRehearsalInList` (Band) —
  with `localToday()`. The PUT is idempotent, so re-logging an
  already-practiced day is a no-op. The row flips to the done treatment
  (check + dimmed) once the query reflects today's date.
- **Done → click to undo:** opens the reused `ConfirmModal`
  (`components/songs/ConfirmModal.tsx`). On confirm, delete today's entry via
  `useUndoPractice` (Personal) or `useUndoBandRehearsalInList` (Band) with
  `localToday()`; the row returns to not-done. Since practice events are one
  atomic, idempotent entry per (song, subject, day), undo simply toggles that
  day's entry off.

The suggestion order is frozen after generation — marking or undoing a song
never reorders or removes it from the list.

Known, accepted behavior: a song shown as done because it was practiced
earlier today from the Library can be undone here, which removes that day's
entry. This is consistent with the one-entry-per-day model and is an explicit
action on a visibly-done row (further guarded by the confirm modal).

## Edge cases

- **Empty source** — show a "No songs in this source" prompt instead of a
  list.
- **Fewer songs than the requested count** — return all available (algorithm
  handles this).
- **Viewer + Band mode** — the band-ranked list is visible; the per-row
  action is disabled with a tooltip explaining Viewers can't log rehearsals.
- **Deleted / removed song** — an id in the saved `list` that no longer
  resolves in the live data is skipped when rendering.
- **Mode toggle on a personal source** — hidden; mode is implicitly Personal.

## Testing

- **Unit** (`frontend/src/lib/practice.test.ts`) — the `suggest()` function:
  ascending order, never-practiced first, tie randomness (deterministic via
  injected RNG), and the `count` cap / fewer-than-count case.
- **Component** (`PracticePage`) — Vitest + Testing Library with hooks mocked
  (per `frontend/src/test/setup.ts`): generating a list, the derived
  done-state for a song whose `lastPracticedAt` is today, the undo confirm
  flow, and restoration of the persisted list on remount.

## Out of scope

- No backend, model, or API changes.
- No new persistence table; the suggestion lives only in `localStorage`.
- No changes to how practice/rehearsal events are recorded elsewhere.
- No cross-device sync of the current suggestion.
