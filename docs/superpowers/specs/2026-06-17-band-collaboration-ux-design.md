# Band collaboration UX improvements — Design

**Date:** 2026-06-17
**Status:** Approved design, pre-implementation

Three related improvements to the band/collaboration experience, delivered as
one spec and one implementation plan:

1. **One-tap band rehearsal logging** — make logging a band rehearsal a single
   click from the band view, instead of band → song → "Rehearsed today".
2. **Band name on the invite page** — the `/join/:token` link page should name
   the band you're being invited to.
3. **Public join page + return-after-auth** — an invite link should work for
   logged-out visitors, and survive the login/signup flow so the join
   completes afterward.

## Background

- Band songs already carry a band-keyed rehearsal log. `Repo.SongsForBand`
  joins the band's `practice_events` into each list item's `lastPracticedAt`
  (last rehearsal date) and `practiceCount` (rehearsal count). The
  `PUT /api/bands/:id/songs/:songId/rehearsal` and
  `DELETE /api/bands/:id/songs/:songId/rehearsal/:date` endpoints already
  exist; only the per-song log hook (`useLogBandRehearsal`) is wired today,
  used by `BandSongPage`.
- Direct invites are surfaced in-app via `GET /api/invites` (which already
  includes `bandName`) and accepted by invite id. **Share links**
  (`/join/:token`) carry only the opaque token; there is no endpoint to
  resolve a token to a band name without joining.
- `JoinByLink` (`Repo.JoinByLink`) looks up any pending, non-expired,
  non-revoked invite by `token_hash` and adds the caller as a member
  (idempotent if already a member).
- `/join/:token` currently renders inside `RequireAuth` + `Layout`. An
  unauthenticated visitor is redirected to `/login` with no memory of the
  destination, so the token is lost; signup always lands on `/`.

## Feature 1 — One-tap band rehearsal logging

Frontend only. The endpoints and rehearsal data already exist.

### Components

- **`BandSongRow`** (new, `frontend/src/components/bands/BandSongRow.tsx`):
  mirrors the redesigned Library `SongRow`. Props:
  `{bandId, song: SongListItem, canEdit, onRehearsed: (songId, date) => void}`.
  Renders:
  - title (display font) + artist as a link to `/bands/${bandId}/songs/${song.id}`,
  - a `StatusBadge` for the band status,
  - last-rehearsed in mono (`song.lastPracticedAt || 'never'`),
  - a **"Rehearsed"** button (with `CircleCheck` icon) shown only when
    `canEdit` is true. The button is a sibling of the link (not nested in the
    anchor) and calls `onRehearsed(song.id, localToday())`.

- **`BandSongList`** (restyle, `frontend/src/components/bands/BandSongList.tsx`):
  replace the single gray "settings" card holding plain rows with an
  action-oriented list of `BandSongRow` items styled like the Library
  (`bg-base-100`, bordered, hover). Keep the existing header ("Songs" +
  "Add song" for editors). Add an **undo toast** and an **error toast** using
  the same pattern as `HomePage` (a 6-second auto-dismiss undo with an Undo
  button; a separate error toast). Viewers see rows with no Rehearsed button.

### Hooks

Add to `frontend/src/hooks/bandsongs.ts`, scoped to a band so the list can act
on any row (the existing per-song `useLogBandRehearsal`/`BandSongPage` path is
left unchanged):

- `useLogBandRehearsalInList(bandId)` → `mutate({songId, date})`,
  `PUT /api/bands/${bandId}/songs/${songId}/rehearsal`.
- `useUndoBandRehearsalInList(bandId)` → `mutate({songId, date})`,
  `DELETE /api/bands/${bandId}/songs/${songId}/rehearsal/${date}`.

Both invalidate `['bands', bandId, 'songs']`, the affected
`['bands', bandId, 'songs', songId]`, and `['songs']` (band songs surface in
the personal library too), reusing the existing `invalidateBandSong` helper.

### Flow

1. Editor/Admin clicks **Rehearsed** on a row → log mutation fires for today.
2. On success, an undo toast appears: `Rehearsed "{title}"` with an Undo
   button. Undo calls the delete mutation for `{songId, today}`.
3. On error, an error toast appears: `Could not log rehearsal — try again.`

## Feature 2 — Band name on the invite page

### Backend

- **`Repo.BandNameByLinkToken(token string) (string, error)`** (in
  `internal/repository/bandinvites.go`): hash the token, look it up via the
  same `pendingInviteScope` used by `JoinByLink`, join to the band, return the
  band name. Return `gorm.ErrRecordNotFound` when no matching pending invite
  exists (so the handler can map it to 404). Mirrors `JoinByLink` semantics:
  any pending, non-expired, non-revoked invite token resolves (link or
  direct); revoked/expired/unknown tokens do not.

- **`(a *API) PreviewInviteLink(c *echo.Context) error`** (in
  `internal/handlers/bandinvites.go`): read the `:token` path param, call the
  repo method, return `200 {"bandName": "..."}` or `404` (via the existing
  `notFoundOr` helper). **Unauthenticated** — no `CurrentUser`/session needed.
  Empty token → 404.

- **Route:** register a public route in `cmd/bandwidth/server.go`:
  `e.GET("/api/invites/link/:token", api.PreviewInviteLink, authLimiter)`
  — outside the `RequireAuth` `invites` group, sharing the auth rate limiter
  (per-IP, to blunt token guessing). No path collision: the authed join
  endpoint is `POST /api/invites/link`; this is `GET …/link/:token`.

### Frontend

- **`useLinkPreview(token)`** (in `frontend/src/hooks/invites.ts`):
  `useQuery` → `GET /api/invites/link/${token}`, returns `{bandName: string}`.
  A 404 surfaces as an `ApiError` the page treats as "invalid/expired".

## Feature 3 — Public join page + return-after-auth

### Routing (`frontend/src/App.tsx`)

Move `/join/:token` out of the `RequireAuth` + `Layout` group into the public
route group alongside `/login` and `/signup`. It renders for logged-out
visitors without the app nav shell.

### `JoinPage` (rewrite, `frontend/src/pages/JoinPage.tsx`)

Auth-aware and styled standalone (centered, with the BandWidth logo mark, like
the auth pages):

- No token in the URL → "Invalid invite link."
- `useLinkPreview(token)`: loading spinner; on error (404) →
  "This invite link is invalid or has expired."; on success it has the band
  name.
- `useMe()` to determine auth. A 401 is treated as **logged-out** (the page is
  public; it must not redirect).
  - **Logged in:** "You've been invited to join *{bandName}*." + a **Join**
    button (`useJoinByLink` → on success navigate to `/bands/${bandId}`).
  - **Logged out:** same heading + **Log in** and **Sign up** buttons linking
    to `/login?redirect=${encodeURIComponent('/join/' + token)}` and the same
    for `/signup`.

The Join button is explicit (no auto-join), so the member confirms which band
before joining.

### Return-after-auth

- **`safeRedirect(value: string | null): string`** (new
  `frontend/src/lib/redirect.ts`): returns `value` only when it is a
  same-origin relative path — begins with a single `/` and not `//` or `/\`
  (open-redirect guard); otherwise returns `/`.

- **`LoginPage` / `SignupPage`**: read `redirect` via `useSearchParams`,
  compute `safeRedirect(redirect)`, and `navigate(target)` on success instead
  of the hardcoded `/`. Their cross-links carry the param forward:
  "No account? Sign up" → `/signup?redirect=…`, "Already have an account? Log
  in" → `/login?redirect=…`. The "Forgot password?" link is unchanged
  (password-reset redirect is out of scope).

- **`RequireAuth`**: when redirecting to login, include the current location:
  `<Navigate to={'/login?redirect=' + encodeURIComponent(location.pathname + location.search)} replace />`
  (via `useLocation`). This generalizes return-after-auth to every protected
  deep link, not just `/join`.

## Testing

### Backend (Go, via Dagger)

- Repo test (`internal/repository/bandinvites_test.go`):
  `BandNameByLinkToken` returns the band name for a valid pending link token;
  returns not-found for a bogus token, a revoked link, and an expired invite.
- Handler test (`internal/handlers/bandinvites_test.go`):
  `GET /api/invites/link/:token` returns `200` with `bandName` for a valid
  token and `404` for a bogus one, **without** an authenticated session.

### Frontend (Vitest)

- `frontend/src/lib/redirect.test.ts`: `safeRedirect` allows `/join/abc`,
  rejects `//evil.com` and `https://evil.com` and protocol-relative/backslash
  forms, and defaults to `/` for null/empty.
- `JoinPage` tests: logged-out renders band name + Log in/Sign up links
  carrying the redirect param; logged-in renders Join and joins → navigates to
  the band; invalid token renders the expired message.
- `LoginPage` / `SignupPage` tests: with `?redirect=/join/abc`, a successful
  submit navigates to `/join/abc`; the cross-link preserves the param.
- `BandSongList` test (extend existing
  `frontend/src/components/bands/BandSongList.test.tsx`): an editor sees the
  Rehearsed button, clicking it issues the `PUT …/rehearsal` and shows the undo
  toast; a viewer sees no Rehearsed button. Existing assertions stay green.

All checks run through Dagger (`just check`); the production bundle must build
(`just build`) since it compiles the frontend.

## Out of scope

- Invitation emails.
- Auto-join on arrival (an explicit Join button is kept).
- Changes to the in-app direct-invite (accept/decline) flow.
- Carrying a redirect through the password-reset flow.
