# BandWidth — Design

**Date:** 2026-06-10
**Status:** Approved design, pre-implementation

BandWidth is a practice tool for musicians and bands. It tracks song learning
with four statuses — Not Learned, Learning, Learned, Nailed! — and lets users
log that they practiced a song (day-granularity events, not time tracking).
It is a clean, simple, modern web app, installable as a PWA, deployed as a
single container on fly.io, and intended to be open source and very low cost
to run.

## Core Concepts

- **Song** — identity only: title, artist, and an owner (a user XOR a band).
- **Metadata layers** — all song metadata (status, notes, resources, practice
  events) lives in annotation layers keyed by song + subject, where the
  subject is a user or a band. Personal and band metadata have identical
  shape and share one code path.
- **Personal library** — a user's own songs plus every song owned by bands
  they belong to, in one merged list.
- **Band** — a group of users with roles. Band songs carry a full band-level
  metadata layer (band status, band notes/resources, rehearsal log).
- **Practice event** — "this song was practiced on this date." Deduplicated
  per song/subject/day. Band-level events represent rehearsals.

### The interleaving rule

- In the **personal view**, opening a band song shows *your* metadata
  (editable) plus a clearly labeled read-only section — "Band: \<name\>" —
  showing the band's status, notes, resources, and last rehearsal.
- In the **band view**, opening a song shows only the band layer.
- Band metadata is editable **only** from the band view (prevents accidental
  edits). Personal endpoints only ever write your layer; band endpoints only
  write the band layer.
- Individual members' metadata is never visible to other members or at the
  band level.

## Features

### Views

Three top-level views: **Personal** (default), **Band**, **Profile**.

### Personal view (`/`)

- Song list: title, artist, color-coded status badge, "last practiced"
  indicator, and a one-tap **Practiced** button (logs today; undo toast
  instead of a confirmation; deduped per day). Song detail offers a date
  picker to backfill a missed day.
- Practice display is **minimal stats**: "last practiced X days ago" and
  total days practiced. No heatmap, no scrollable log.
- **Search**: instant client-side fuzzy filter (Fuse.js) on title/artist.
- **Folders**: playlist-style. A song may be in many folders or none.
  "All Songs" is the default view. Folders and songs-within-folders are
  drag-reorderable. Personal folders may contain any song visible to the
  user, including band songs.
- Band songs appear in the list tagged with the band name. They cannot be
  deleted from the personal view.
- Add song: quick form (title + artist required, all else optional).
  Delete song: confirmation dialog required.

### Band view (`/bands/:id`)

- Same list/folder/search UX, scoped to the band's songs and folders,
  showing the band metadata layer.
- **Rehearsed** button logs a band-level practice event (same day model).
- Roles:
  - **Admin** — manage band settings, members, invites, songs, folders.
    The creator is a permanent Admin and cannot be demoted or removed.
  - **Editor** — add/edit/delete band songs and folders, log rehearsals.
    Default role for new members.
  - **Viewer** — read-only.
- Member management (Admin): invite existing users by username/email lookup
  (invitee sees a pending invite in-app and accepts/declines), or generate a
  revocable shareable invite link. Admins set roles and remove members.
- No invitation emails are sent in v1.

### Profile view (`/profile`)

- Edit username, email, password.
- TOTP 2FA enroll/disable: QR code at enrollment plus one-time backup codes
  (shown once, stored hashed).
- List of the user's bands with a "leave band" action.

### Auth

- Signup: username, email, password. Login: password, then TOTP step when
  2FA is enrolled (backup codes accepted).
- Password hashing with argon2id. DB-backed sessions in an HTTP-only Secure
  cookie. CSRF middleware. Rate-limited login.
- Password reset by email **only when SMTP is configured**; otherwise the
  feature is hidden and its endpoints return 404. Email is provider-agnostic
  SMTP (`wneessen/go-mail`) configured by env vars — no vendor SDK.

### Leave/removal conversion rule

When a band song stops being available to a user (the user leaves or is
removed, the band deletes the song, or the band is deleted):

- If the user has **any personal data** on it (an annotation row, resources,
  or practice events), the song converts to a personal song: a personal copy
  of the identity (title/artist) is created and the user's annotation,
  resource, and practice rows are re-pointed to it. The band layer is not
  copied.
- Untouched band songs simply disappear from the user's library.

The conversion runs in a transaction and applies the same rule in all
removal paths.

## Data Model

SQLite via GORM (AutoMigrate), WAL mode.

```
users              id, username (uq), email (uq), password_hash,
                   totp_secret?, created_at, updated_at
backup_codes       user_id, code_hash, used_at?
sessions           token (uq), user_id, expires_at
password_resets    token_hash, user_id, expires_at, used_at?

bands              id, name, creator_id
band_members       band_id, user_id, role (admin|editor|viewer)   uq(band, user)
band_invites       id, band_id, role, invited_user_id?,           -- direct invite
                   token?, expires_at, revoked_at?, accepted_at?  -- or share link

songs              id, title, artist,
                   owner_user_id?, owner_band_id?      CHECK exactly one owner
song_annotations   song_id, user_id?, band_id?,        CHECK exactly one subject
                   status, notes                       uq(song, subject)
resources          song_id, user_id?, band_id?, url, label, position
practice_events    song_id, user_id?, band_id?, date   uq(song, subject, date)

folders            id, name, position, owner_user_id?, owner_band_id?
folder_entries     folder_id, song_id, position        uq(folder, song)
```

- `status` is a string enum: `not_learned | learning | learned | nailed`.
- A missing annotation row reads as Not Learned with empty notes. Joining a
  band therefore requires no backfill; member overlay rows are created
  lazily on first edit.
- Personal library query: songs where `owner_user_id = me` OR
  `owner_band_id IN (my bands)`, joined with my annotations and, for band
  songs, the band's annotations.
- Reordering uses integer `position` columns, reindexed on reorder.
- Deleting a song cascades to its annotations, resources, practice events,
  and folder entries (after the conversion rule, for band songs).

## API

JSON REST under `/api`, session-cookie auth.

```
POST   /api/auth/signup | login | logout
POST   /api/auth/2fa/setup | verify | disable
POST   /api/auth/password-reset | password-reset/confirm   (404 when SMTP off)
GET    /api/me          PATCH /api/me

GET    /api/songs                        # merged personal library, both layers
POST   /api/songs                        # create personal song
GET|PATCH|DELETE /api/songs/:id          # PATCH edits identity (if owned) + my annotation
PUT    /api/songs/:id/practice           # body {date?} default today; idempotent
DELETE /api/songs/:id/practice/:date
POST|PATCH|DELETE /api/songs/:id/resources[/:resourceId]

GET|POST /api/folders
PATCH|DELETE /api/folders/:id
PUT    /api/folders/:id/entries          # full membership + order

GET|POST /api/bands     GET|PATCH|DELETE /api/bands/:id
GET|POST /api/bands/:id/songs            # band layer; role-checked
GET|PATCH|DELETE /api/bands/:id/songs/:songId
PUT    /api/bands/:id/songs/:songId/rehearsed
POST|PATCH|DELETE /api/bands/:id/songs/:songId/resources[/:resourceId]
...    /api/bands/:id/folders[...]       # same shape as personal folders
GET|POST|PATCH|DELETE /api/bands/:id/members[/:userId]
GET|POST|DELETE /api/bands/:id/invites[/:inviteId]
GET    /api/invites                      # my pending invites
POST   /api/invites/:id/accept | decline
POST   /api/invites/link/:token          # join via share link
GET    /healthz
```

## Architecture & Stack

### Backend

- Go 1.26, Echo (v5 if stable at implementation start, else v4),
  Cobra + Viper CLI/config.
- Env vars: `BANDWIDTH_PORT`, `BANDWIDTH_DB_PATH`, `BANDWIDTH_COOKIE_SECRET`
  (required, 32+ chars), `BANDWIDTH_BASE_URL`, `BANDWIDTH_LOG_LEVEL`,
  `BANDWIDTH_SMTP_{HOST,PORT,USER,PASS,FROM}` (optional).
- GORM + `ncruces/go-sqlite3` (CGO-free).
- Layout: `cmd/bandwidth/`, `internal/model/`, `internal/repository/`,
  `internal/handlers/`, `internal/middleware/`, `internal/mail/`,
  `internal/static/` (embedded frontend dist), `version/`.
- Frontend dist embedded via `go:embed`; SPA fallback handler. No SSG/SEO
  machinery — this is an authenticated app.

### Frontend (`frontend/`)

- React 19 + TypeScript, Vite 7, bun for dependencies.
- React Router 7 routes: `/`, `/songs/:id`, `/bands/:id`,
  `/bands/:id/songs/:songId`, `/profile`, `/login`, `/signup`.
- TanStack Query 5 for server state. Tailwind CSS v4 + DaisyUI 5 for UI.
  Fuse.js for search. dnd-kit for drag-and-drop (touch-friendly).
- Mobile-first layout; the Practiced button is thumb-reachable on list rows.

### PWA

- `vite-plugin-pwa` (Workbox): `registerType: 'autoUpdate'`, precached app
  shell (JS/CSS/fonts/icons), **NetworkOnly for `/api`** — no stale data in
  v1. Offline data can layer on later without rework.
- Manifest: name, theme colors, maskable icons (192/512),
  `display: standalone`. An "update available → reload" toast wired to the
  service worker lifecycle.

### Testing

- Go: table-driven tests; repository and handler tests against in-memory
  SQLite. Dedicated coverage for the leave-band conversion and role
  enforcement.
- Frontend: Vitest + Testing Library for components and hooks (status badge,
  practice logging, folder ordering logic).

### CI / Deploy

- just + Dagger per the ci style guide: `dev` (air + vite hot reload),
  `check` (golangci-lint, ESLint, Prettier-check, tsc, go test, vitest in
  one parallel Dagger session), `fmt`, `build`, `release`.
- GitHub Actions invoking the same Dagger functions; Renovate for updates.
- Canonical configs copied from the style-guides repo: `.golangci.yml`,
  `eslint.config.js`, `.prettierrc.json`, `tsconfig.base.json`.
- fly.io: single Alpine container, Fly volume for the SQLite file. Exactly
  one always-on machine (SQLite cannot multi-attach; no auto stop/start —
  the app fits under Fly's monthly billing minimum anyway). Optional later:
  Litestream backups.

## Out of Scope for v1

Offline data/sync, push notifications, song keys/tempo/tuning metadata,
setlist printing/export, audio attachments, band chat, email invites,
email verification on signup, admin UI.
