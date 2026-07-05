# Admin panel & signup access policy — Design

**Date:** 2026-07-03
**Status:** Approved design, pre-implementation

Add a site-admin panel for user/band management, and a signup access policy
(deny-all-except-list) so registration can be locked down without relying on
an external gateway (Cloudflare Access hit onboarding friction and was
abandoned for this).

## Background

- `User` currently has no global role — only `BandMember.Role`
  (viewer/editor/admin), scoped to a single band. There is no concept of a
  site-wide administrator.
- Registration (`POST /api/auth/signup`, `internal/handlers/auth.go`) is
  fully open today: any valid, non-duplicate username/email/password creates
  an account.
- There is no user-deletion path anywhere in the app yet. `DeleteBand`
  (`internal/repository/bands.go`) is the closest existing cascade-delete
  example: it runs in a transaction, converts each member's band-owned songs
  to personal copies, then deletes band members, invites, and the band row.
- Config is wired through `viper` with a `BANDWIDTH_` env prefix
  (`internal/config` / `cmd/bandwidth/server.go`); Fly secrets are the
  existing mechanism for anything sensitive (`BANDWIDTH_DB_PATH`,
  `BANDWIDTH_SECURE_COOKIES`, etc.) — nothing sensitive is committed to the
  (public) repo.
- Frontend is React + react-router + react-query. `RequireAuth.tsx`
  (`frontend/src/components/`) is the existing pattern for a route guard
  driven by the `useMe()` hook; `Layout.tsx` holds the nav.

## Decisions

- **Site-admin identity is not stored in the database.** It's a
  `BANDWIDTH_ADMIN_EMAILS` Fly secret (comma-separated), parsed once at
  startup into an in-memory set. This keeps admin identity out of the public
  git repo entirely — no encryption scheme needed, consistent with how every
  other sensitive value in this app is handled. Changing who is an admin
  requires `flyctl secrets set` + redeploy, which is acceptable since it
  changes rarely.
- **The signup access policy is DB-backed and admin-editable**, since it's
  expected to change more often than admin identity and the whole point is
  to avoid redeploy-per-change friction.
- **The policy only gates registration**, not login. Existing accounts
  always work; removing an email from the allow-list does not revoke
  already-created accounts.
- **Admin capabilities are list + delete only** for both users and bands —
  no edit, no force-logout, no membership drill-down. Kept intentionally
  narrow.
- **The admin panel reuses the normal session/login** (no separate admin
  auth system). A route is gated by checking the logged-in user's email
  against the admin set, both server-side (middleware) and client-side
  (nav visibility).
- **Deleting a user cascades to any band they created** (bands have a
  permanent creator/admin with no ownership-transfer feature), in addition
  to their personal songs/folders/sessions/etc. Other members of a
  cascade-deleted band lose access to it entirely.
- **An admin cannot delete their own account** through the panel (avoids
  self-lockout).

## Data model

Two new tables, added to the existing `AutoMigrate` list in
`internal/repository/repository.go`:

```go
// AccessPolicy is a single-row settings record controlling signup gating.
type AccessPolicy struct {
    ID      uint `gorm:"primarykey"`
    Enabled bool `gorm:"not null"` // false = open registration (today's behavior)
}

// AllowedEmail is one entry on the signup allow-list.
type AllowedEmail struct {
    ID        uint   `gorm:"primarykey"`
    Email     string `gorm:"uniqueIndex;not null"` // stored lowercase/trimmed
    CreatedBy uint   `gorm:"not null"`
    CreatedAt time.Time
}
```

`AccessPolicy` is a singleton: if no row exists, one is created with
`Enabled=false` on first access, so deploying this feature does not lock out
registration until an admin explicitly enables it.

No new column/table represents site-admin status — see Decisions.

## Config wiring

- New viper key `admin_emails` → env `BANDWIDTH_ADMIN_EMAILS`
  (comma-separated), parsed in `cmd/bandwidth/server.go` into
  `map[string]bool` and passed into `handlers.API` as a new
  `AdminEmails map[string]bool` field, alongside the existing
  `SecureCookies`/`BaseURL` fields.
- Setting `BANDWIDTH_ADMIN_EMAILS` on Fly (`flyctl secrets set ...`) is a
  deploy step to call out in the implementation plan, not something done in
  code.
- Empty/unset `BANDWIDTH_ADMIN_EMAILS` simply means no one matches — the
  panel is unreachable, no special-cased error message.

## Signup enforcement

`Signup` (`internal/handlers/auth.go`) gains one check, after existing field
validation and before `CreateUser`:

```go
if a.Repo.AccessPolicyEnabled() {
    if !a.IsAdminEmail(req.Email) && !a.Repo.EmailAllowed(req.Email) {
        return echo.NewHTTPError(http.StatusForbidden, "registration is not open")
    }
}
```

- `IsAdminEmail` checks the in-memory admin set — admins never need a
  redundant allow-list row.
- Same generic 403 either way (policy disabled entirely vs. policy enabled
  but email not listed are indistinguishable to the caller) — avoids leaking
  whether the app is gated at all.
- No caching of `AccessPolicyEnabled`/`EmailAllowed` — each is one indexed
  row read, and `/auth/signup` is already behind the existing per-IP rate
  limiter (`authLimiter` in `server.go`), so the extra query cost is
  negligible against the complexity of cache invalidation.
- `Login` is untouched.

## Admin middleware & routes

New middleware in `internal/middleware/auth.go`, layered after `RequireAuth`
(needs `CurrentUser` already loaded):

```go
func RequireAdmin(isAdminEmail func(string) bool) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) error {
            user := CurrentUser(c)
            if user == nil || !isAdminEmail(user.Email) {
                return echo.NewHTTPError(http.StatusForbidden, "admin access required")
            }
            return next(c)
        }
    }
}
```

New route group in `cmd/bandwidth/server.go`, alongside the existing groups:

```go
admin := apiGroup.Group("/admin", appmw.RequireAuth(api.Repo), appmw.RequireAdmin(api.IsAdminEmail))
admin.GET("/users", api.AdminUsers)
admin.DELETE("/users/:id", api.AdminDeleteUser)
admin.GET("/bands", api.AdminBands)
admin.DELETE("/bands/:id", api.AdminDeleteBand)
admin.GET("/access-policy", api.AdminGetAccessPolicy)
admin.PUT("/access-policy", api.AdminSetAccessPolicy)        // {enabled: bool}
admin.POST("/access-policy/emails", api.AdminAddAllowedEmail)
admin.DELETE("/access-policy/emails/:id", api.AdminRemoveAllowedEmail)
```

- `AdminDeleteBand` reuses `Repo.DeleteBand`.
- `AdminDeleteUser` calls the new `Repo.DeleteUser` (below) and 400s if
  `:id` matches the current admin's own user ID (self-delete guard).
- `Me` (`GET /api/me`) response gains an `isAdmin bool` field, computed the
  same way (checked against `AdminEmails`, not stored), so the frontend
  knows whether to render the admin nav link.

## `Repo.DeleteUser` cascade

New function in `internal/repository/users.go`, transactional like
`DeleteBand`:

```go
// DeleteUser removes a user and everything they solely own: sessions, 2FA
// backup codes, pending password resets, personal (non-band) songs/folders,
// band memberships, and any band they created (cascaded, same as DeleteBand).
func (r *Repo) DeleteUser(userID uint) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        var createdBands []model.Band
        if err := tx.Where("creator_id = ?", userID).Find(&createdBands).Error; err != nil {
            return err
        }
        for _, b := range createdBands {
            if err := deleteBandTx(tx, b.ID); err != nil {
                return err
            }
        }
        // personal songs (owner_band_id IS NULL, owner_user_id = userID),
        // via the DeleteSong body, factored the same way
        // folders, sessions, backup codes, password resets,
        // remaining band_member rows
        ...
        return tx.Delete(&model.User{}, userID).Error
    })
}
```

This requires factoring `DeleteBand`'s transaction body into a
`deleteBandTx(tx *gorm.DB, bandID uint) error` helper (GORM/SQLite don't
support nested transactions), with `DeleteBand(bandID)` becoming a thin
wrapper that opens a transaction and calls it. `DeleteSong`
(`internal/repository/songs.go`) needs the same treatment for personal-song
cleanup. This is the trickiest part of the implementation.

## Access-policy & listing repo functions

Straightforward CRUD, no cascade complexity:

```go
func (r *Repo) AccessPolicyEnabled() (bool, error)
func (r *Repo) SetAccessPolicyEnabled(enabled bool) error
func (r *Repo) EmailAllowed(email string) (bool, error)
func (r *Repo) AllowedEmails() ([]model.AllowedEmail, error)
func (r *Repo) AddAllowedEmail(email string, addedBy uint) (*model.AllowedEmail, error) // dup -> IsDuplicate(err)
func (r *Repo) RemoveAllowedEmail(id uint) error

func (r *Repo) AllUsers() ([]model.User, error)          // id, username, email, created_at
func (r *Repo) AllBands() ([]model.BandWithStats, error) // name, creator, member count (joined query)
```

Emails are lowercased/trimmed before insert/lookup, matching `Signup`'s
existing normalization.

## Frontend

Follows existing patterns exactly (react-router, `useMe()`/react-query
hooks, DaisyUI classes as in `Layout.tsx`).

- `Me` response type gains `isAdmin: boolean`.
- New `RequireAdmin.tsx` component, same shape as `RequireAuth.tsx`: reads
  `useMe()`, `<Navigate to="/" />` if `!data.isAdmin`, else `<Outlet />`.
  Nested inside the existing `RequireAuth` route.
- `Layout.tsx` gets a conditional `NavLink` to `/admin`, rendered only when
  `useMe().data?.isAdmin`.
- One new page, `AdminPage.tsx`, with three sections/tabs (one route, not
  three):
  - **Users** — table (username, email, created), delete button per row
    with a confirm dialog (cascading, destructive).
  - **Bands** — table (name, creator, member count), delete button per row
    with a confirm dialog.
  - **Access policy** — enabled/disabled toggle + allow-list table with an
    add-email form and per-row remove.
- New `frontend/src/hooks/admin.ts` with react-query hooks (`useAdminUsers`,
  `useDeleteAdminUser`, `useAdminBands`, `useDeleteAdminBand`,
  `useAccessPolicy`, `useSetAccessPolicy`, `useAllowedEmails`,
  `useAddAllowedEmail`, `useRemoveAllowedEmail`) mirroring how existing
  hooks wrap `/api/bands` etc.
- Routing in `App.tsx`: `<Route path="/admin" element={<AdminPage />} />`
  nested under a new `<Route element={<RequireAdmin />}>`, itself inside the
  existing `RequireAuth`/`Layout` nesting.

## Testing

Follows the existing per-file `*_test.go` / `*.test.tsx` convention:

- `Repo.DeleteUser` cascade test (fixtures similar to
  `TestDeleteBandConvertsForAllMembers` in `conversion_test.go`).
- Access-policy enable/disable + allow/deny signup cases in
  `internal/handlers/auth_test.go`.
- `RequireAdmin` middleware test (non-admin 403, admin passes) in
  `internal/middleware/auth_test.go`.
- New `internal/handlers/admin_test.go`: list/delete users & bands,
  self-delete blocked, allow-list add/remove/duplicate.
- Frontend: `RequireAdmin.test.tsx` and `AdminPage.test.tsx`.

## Out of scope

- Editing user details, resetting passwords from the panel, force-logout.
- Viewing band membership detail before deleting.
- Login-time enforcement of the allow-list (revocation of existing
  accounts).
- Domain-based or invite-link-based allow-list entries — exact-email-match
  only.
- Any changes to the Cloudflare setup; this feature replaces the need for
  Cloudflare Access rather than integrating with it.
</content>
