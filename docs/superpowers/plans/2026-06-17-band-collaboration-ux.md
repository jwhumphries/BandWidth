# Band Collaboration UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make band rehearsals one-tap from the band view, name the band on the `/join` invite page, and let invite links survive the login/signup flow.

**Architecture:** Three slices over the existing Go + Echo + SQLite backend and React 19 + TanStack Query SPA. Feature 1 is frontend-only (endpoints and rehearsal data already exist). Feature 2 adds one public read endpoint plus a repo method. Feature 3 moves `/join/:token` to a public, auth-aware page and threads a validated `?redirect=` param through login/signup.

**Tech Stack:** Go (Echo v5, GORM, `ncruces/go-sqlite3`), React 19, react-router v7, TanStack Query, Tailwind v4 + DaisyUI 5, lucide-react, Vitest, Testing Library. All checks/tests run in Dagger via `just`.

**Spec:** `docs/superpowers/specs/2026-06-17-band-collaboration-ux-design.md`

**Conventions:**
- Tests run only through Dagger: `just test` (Go), `just test-frontend` (JS), `just check` (everything). There is no single-test filter; each command runs its full suite.
- Format before committing JS/TS: `just format`. Go is formatted by `just fmt`.
- Frontend tests assert on accessible names, placeholders, and visible text — keep those stable.
- Commit after each task.

---

## Task 1: Repo method `BandNameByLinkToken`

Resolves a pending invite token to its band name without joining (powers the `/join` preview).

**Files:**
- Modify: `internal/repository/bandinvites.go`
- Test: `internal/repository/bandinvites_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/repository/bandinvites_test.go`:

```go
func TestBandNameByLinkToken(t *testing.T) {
	repo, alice, _, band := inviteFixture(t)

	_, token, err := repo.CreateLinkInvite(band.ID, model.RoleViewer, alice.ID)
	if err != nil {
		t.Fatalf("CreateLinkInvite: %v", err)
	}

	// Valid token resolves to the band name.
	name, err := repo.BandNameByLinkToken(token)
	if err != nil || name != "Band" {
		t.Fatalf("BandNameByLinkToken = %q, %v; want \"Band\"", name, err)
	}

	// Bogus token is not found.
	if _, err := repo.BandNameByLinkToken("bogus"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("bogus token err = %v; want ErrRecordNotFound", err)
	}

	// Revoked link stops resolving.
	invites, _ := repo.InvitesForBand(band.ID)
	if err := repo.RevokeInvite(invites[0].ID, band.ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}
	if _, err := repo.BandNameByLinkToken(token); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("revoked token err = %v; want ErrRecordNotFound", err)
	}
}
```

Add `"gorm.io/gorm"` to the test file's imports if not already present (it is not — add it).

- [ ] **Step 2: Run test to verify it fails**

Run: `just test`
Expected: FAIL — `repo.BandNameByLinkToken undefined`.

- [ ] **Step 3: Implement the repo method**

Add to `internal/repository/bandinvites.go` (after `JoinByLink`):

```go
// BandNameByLinkToken resolves a pending invite token to its band name
// without joining. Mirrors JoinByLink's lookup: any pending, non-expired,
// non-revoked invite resolves; anything else is not found.
func (r *Repo) BandNameByLinkToken(token string) (string, error) {
	hash := auth.HashToken(token)
	var names []string
	err := pendingInviteScope(r.db.Table("band_invites")).
		Joins("JOIN bands ON bands.id = band_invites.band_id").
		Where("band_invites.token_hash = ?", hash).
		Pluck("bands.name", &names).Error
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", gorm.ErrRecordNotFound
	}
	return names[0], nil
}
```

(`auth` and `gorm` are already imported in this file.)

- [ ] **Step 4: Run test to verify it passes**

Run: `just test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/bandinvites.go internal/repository/bandinvites_test.go
git commit -m "feat(invites): resolve a link token to its band name"
```

---

## Task 2: Public `PreviewInviteLink` handler + route

Exposes the band name for a token over an unauthenticated GET.

**Files:**
- Modify: `internal/handlers/bandinvites.go`
- Modify: `cmd/bandwidth/server.go:181-185` (register the public route near the invites group)
- Test: `internal/handlers/bandinvites_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/handlers/bandinvites_test.go`. First, register the public route in the test harness `newInvitesAPI` (add this line before `return e, api`):

```go
	e.GET("/api/invites/link/:token", api.PreviewInviteLink)
```

Then add the test:

```go
func TestPreviewInviteLink(t *testing.T) {
	e, api := newInvitesAPI(t)
	alice := signupAndCookie(t, e, "alice")
	band := createBandFor(t, e, alice, "The Quietones")

	// Create a link invite as admin.
	rec := jsonReq(e, http.MethodPost, fmt.Sprintf("/api/bands/%d/invites", band),
		`{"link":true,"role":"editor"}`, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create link: %d %s", rec.Code, rec.Body.String())
	}
	var link struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil || link.Token == "" {
		t.Fatalf("link body: %s (%v)", rec.Body.String(), err)
	}

	// Preview works WITHOUT a session (nil cookie) and returns the band name.
	rec = jsonReq(e, http.MethodGet, "/api/invites/link/"+link.Token, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		BandName string `json:"bandName"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.BandName != "The Quietones" {
		t.Fatalf("preview body: %s (%v)", rec.Body.String(), err)
	}

	// Bogus token 404s.
	if rec := jsonReq(e, http.MethodGet, "/api/invites/link/bogus", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("bogus preview: %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just test`
Expected: FAIL — `api.PreviewInviteLink undefined`.

- [ ] **Step 3: Implement the handler**

Add to `internal/handlers/bandinvites.go` (after `JoinByLink`):

```go
// PreviewInviteLink returns the band name for a pending invite token,
// without joining and without authentication, so the /join page can name
// the band before the visitor logs in.
func (a *API) PreviewInviteLink(c *echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return echo.NewHTTPError(http.StatusNotFound, "invite not found")
	}
	name, err := a.Repo.BandNameByLinkToken(token)
	if err != nil {
		return notFoundOr(err, "invite")
	}
	return c.JSON(http.StatusOK, map[string]any{"bandName": name})
}
```

- [ ] **Step 4: Register the public route**

In `cmd/bandwidth/server.go`, immediately after the `invites := apiGroup.Group(...)` block (after line 185), add:

```go
	// Public: name the band behind an invite token so /join can show it
	// before the visitor authenticates. Rate-limited like other token paths.
	apiGroup.GET("/invites/link/:token", api.PreviewInviteLink, authLimiter)
```

Note: this is on `apiGroup` (CSRF-wrapped, no `RequireAuth`), not the authed `invites` group. A GET passes the fetch-metadata CSRF check.

- [ ] **Step 5: Run tests to verify they pass**

Run: `just test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/bandinvites.go internal/handlers/bandinvites_test.go cmd/bandwidth/server.go
git commit -m "feat(invites): public endpoint to preview a link's band name"
```

---

## Task 3: `safeRedirect` helper

Validates a post-auth redirect target to prevent open redirects.

**Files:**
- Create: `frontend/src/lib/redirect.ts`
- Test: `frontend/src/lib/redirect.test.ts`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/redirect.test.ts`:

```ts
import {describe, expect, it} from 'vitest';
import {safeRedirect} from './redirect';

describe('safeRedirect', () => {
  it('allows same-origin relative paths', () => {
    expect(safeRedirect('/join/abc')).toBe('/join/abc');
    expect(safeRedirect('/bands/3')).toBe('/bands/3');
  });

  it('rejects protocol-relative and absolute URLs', () => {
    expect(safeRedirect('//evil.com')).toBe('/');
    expect(safeRedirect('https://evil.com')).toBe('/');
    expect(safeRedirect('/\\evil.com')).toBe('/');
  });

  it('defaults to / for null, empty, or non-paths', () => {
    expect(safeRedirect(null)).toBe('/');
    expect(safeRedirect('')).toBe('/');
    expect(safeRedirect('join/abc')).toBe('/');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — cannot resolve `./redirect`.

- [ ] **Step 3: Implement the helper**

Create `frontend/src/lib/redirect.ts`:

```ts
// safeRedirect returns the given value only when it is a same-origin
// relative path (begins with a single "/", not "//" or "/\"), guarding
// against open redirects. Otherwise it returns "/".
export function safeRedirect(value: string | null): string {
  if (!value) return '/';
  if (value[0] !== '/') return '/';
  if (value[1] === '/' || value[1] === '\\') return '/';
  return value;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `just test-frontend`
Expected: PASS.

- [ ] **Step 5: Format and commit**

```bash
just format
git add frontend/src/lib/redirect.ts frontend/src/lib/redirect.test.ts
git commit -m "feat(auth): add safeRedirect open-redirect guard"
```

---

## Task 4: Login & Signup honor `?redirect=`

After auth, return to the page the visitor came from; keep the cross-links carrying the param.

**Files:**
- Modify: `frontend/src/pages/LoginPage.tsx`
- Modify: `frontend/src/pages/SignupPage.tsx`
- Test: `frontend/src/pages/LoginPage.test.tsx`

- [ ] **Step 1: Write the failing test**

Add this test to `frontend/src/pages/LoginPage.test.tsx` inside the existing top-level `describe` (it mounts the page at a route carrying the param). Use the existing fetch-mock style in that file — the login POST should resolve with a user object. Add:

```ts
  it('returns to the redirect target after login', async () => {
    renderWithProviders(
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/join/abc" element={<p>join page</p>} />
      </Routes>,
      {route: '/login?redirect=%2Fjoin%2Fabc'},
    );
    await userEvent.type(screen.getByLabelText(/username or email/i), 'alice');
    await userEvent.type(screen.getByLabelText(/password/i), 'pw');
    await userEvent.click(screen.getByRole('button', {name: /log in/i}));
    await waitFor(() =>
      expect(screen.getByText('join page')).toBeInTheDocument(),
    );
  });
```

Ensure the file imports `Routes`, `Route` from `react-router` and `userEvent`, `waitFor`, `screen` (mirror the existing imports/mock in the file; the existing login test already mocks `fetch` to return a user on POST `/api/auth/login` — reuse that `beforeEach`).

- [ ] **Step 2: Run test to verify it fails**

Run: `just test-frontend`
Expected: FAIL — after login it navigates to `/`, so "join page" never renders.

- [ ] **Step 3: Update LoginPage**

In `frontend/src/pages/LoginPage.tsx`:

Add imports at the top:

```tsx
import {Link, useNavigate, useSearchParams} from 'react-router';
import {safeRedirect} from '../lib/redirect';
```

Inside the component, after `const navigate = useNavigate();`:

```tsx
  const [params] = useSearchParams();
  const redirect = safeRedirect(params.get('redirect'));
  const redirectQuery = redirect === '/' ? '' : `?redirect=${encodeURIComponent(redirect)}`;
```

Change the login success navigation from `() => void navigate('/')` to:

```tsx
        onSuccess: () => void navigate(redirect),
```

Update the "Sign up" link to carry the param:

```tsx
          <Link className="link" to={`/signup${redirectQuery}`}>
            Sign up
          </Link>
```

(Leave the "Forgot password?" link unchanged.)

- [ ] **Step 4: Update SignupPage**

In `frontend/src/pages/SignupPage.tsx`:

Add imports:

```tsx
import {Link, useNavigate, useSearchParams} from 'react-router';
import {safeRedirect} from '../lib/redirect';
```

After `const navigate = useNavigate();`:

```tsx
  const [params] = useSearchParams();
  const redirect = safeRedirect(params.get('redirect'));
  const redirectQuery = redirect === '/' ? '' : `?redirect=${encodeURIComponent(redirect)}`;
```

Change signup success navigation `{onSuccess: () => void navigate('/')}` to:

```tsx
      {onSuccess: () => void navigate(redirect)},
```

Update the "Log in" link:

```tsx
          <Link className="link" to={`/login${redirectQuery}`}>
            Log in
          </Link>
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `just test-frontend`
Expected: PASS (the new test plus the existing login/signup tests, which use no `redirect` param and still navigate to `/`).

- [ ] **Step 6: Format and commit**

```bash
just format
git add frontend/src/pages/LoginPage.tsx frontend/src/pages/SignupPage.tsx frontend/src/pages/LoginPage.test.tsx
git commit -m "feat(auth): return to ?redirect= target after login/signup"
```

---

## Task 5: `RequireAuth` forwards the intended path

So every protected deep link (not just `/join`) returns after login.

**Files:**
- Modify: `frontend/src/components/RequireAuth.tsx`
- Test: `frontend/src/components/RequireAuth.test.tsx` (existing test stays green; add one assertion)

- [ ] **Step 1: Update the redirect to include the current path**

In `frontend/src/components/RequireAuth.tsx`:

Change the imports line:

```tsx
import {Navigate, Outlet, useLocation} from 'react-router';
```

Inside the component, before the `if (isPending)` block:

```tsx
  const location = useLocation();
```

Replace `return <Navigate to="/login" replace />;` with:

```tsx
      return (
        <Navigate
          to={`/login?redirect=${encodeURIComponent(location.pathname + location.search)}`}
          replace
        />
      );
```

- [ ] **Step 2: Strengthen the existing test**

In `frontend/src/components/RequireAuth.test.tsx`, the first test renders the app at `/` and expects the `/login` route to render. That still matches with a query string. Add an assertion that the redirect carried the origin path. Replace the body of the `it('redirects to /login when unauthenticated', ...)` test with a version that captures the URL via a small location probe:

```tsx
  it('redirects to /login with a redirect param when unauthenticated', async () => {
    renderWithProviders(
      <Routes>
        <Route
          path="/login"
          element={<LocationProbe />}
        />
        <Route element={<RequireAuth />}>
          <Route path="/bands/3" element={<p>secret</p>} />
        </Route>
      </Routes>,
      {route: '/bands/3'},
    );
    await waitFor(() =>
      expect(screen.getByTestId('loc')).toHaveTextContent(
        '/login?redirect=%2Fbands%2F3',
      ),
    );
    expect(screen.queryByText('secret')).not.toBeInTheDocument();
  });
```

Add this helper component and the `useLocation` import at the top of the test file:

```tsx
import {Route, Routes, useLocation} from 'react-router';

function LocationProbe() {
  const loc = useLocation();
  return <span data-testid="loc">{loc.pathname + loc.search}</span>;
}
```

`renderWithProviders` accepts a `{route}` option (see `frontend/src/test/utils.tsx`). Keep the second test (server-error → retry) unchanged.

- [ ] **Step 3: Run tests to verify they pass**

Run: `just test-frontend`
Expected: PASS.

- [ ] **Step 4: Format and commit**

```bash
just format
git add frontend/src/components/RequireAuth.tsx frontend/src/components/RequireAuth.test.tsx
git commit -m "feat(auth): RequireAuth forwards the intended path to login"
```

---

## Task 6: Public, auth-aware `JoinPage`

Move `/join/:token` out of the auth shell; show the band name; route logged-out visitors through login/signup and back.

**Files:**
- Modify: `frontend/src/hooks/invites.ts` (add `useLinkPreview`)
- Modify: `frontend/src/App.tsx` (move the route out of `RequireAuth`/`Layout`)
- Rewrite: `frontend/src/pages/JoinPage.tsx`
- Test: `frontend/src/pages/JoinPage.test.tsx` (new)

- [ ] **Step 1: Add the `useLinkPreview` hook**

In `frontend/src/hooks/invites.ts`, add (after `useMyInvites`):

```ts
export function useLinkPreview(token: string) {
  return useQuery<{bandName: string}, ApiError>({
    queryKey: ['invite-preview', token],
    queryFn: () => api.get<{bandName: string}>(`/api/invites/link/${token}`),
    retry: false,
    staleTime: 60 * 1000,
  });
}
```

- [ ] **Step 2: Write the failing tests**

Create `frontend/src/pages/JoinPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import JoinPage from './JoinPage';

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

// loggedIn controls how /api/me resolves.
function mockFetch(loggedIn: boolean) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/api/invites/link/') && (!init || init.method === undefined)) {
        return Promise.resolve(json(200, {bandName: 'The Quietones'}));
      }
      if (url.endsWith('/api/me')) {
        return loggedIn
          ? Promise.resolve(json(200, {id: 1, username: 'a', email: 'a@b.c', totpEnabled: false}))
          : Promise.resolve(json(401, {message: 'authentication required'}));
      }
      if (url.includes('/api/invites/link') && init?.method === 'POST') {
        return Promise.resolve(json(200, {bandId: 7}));
      }
      return Promise.resolve(json(404, {message: 'not found'}));
    }),
  );
}

describe('JoinPage', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('shows the band name and log in / sign up when logged out', async () => {
    mockFetch(false);
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/abc'},
    );
    expect(await screen.findByText(/The Quietones/)).toBeInTheDocument();
    const login = screen.getByRole('link', {name: /log in/i});
    expect(login).toHaveAttribute('href', '/login?redirect=%2Fjoin%2Fabc');
    expect(screen.getByRole('link', {name: /sign up/i})).toHaveAttribute(
      'href',
      '/signup?redirect=%2Fjoin%2Fabc',
    );
  });

  it('joins and navigates to the band when logged in', async () => {
    mockFetch(true);
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
        <Route path="/bands/7" element={<p>band page</p>} />
      </Routes>,
      {route: '/join/abc'},
    );
    await userEvent.click(await screen.findByRole('button', {name: /join/i}));
    await waitFor(() =>
      expect(screen.getByText('band page')).toBeInTheDocument(),
    );
  });

  it('shows an error for an invalid token', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(json(404, {message: 'invite not found'})),
    );
    renderWithProviders(
      <Routes>
        <Route path="/join/:token" element={<JoinPage />} />
      </Routes>,
      {route: '/join/bad'},
    );
    expect(
      await screen.findByText(/invalid or has expired/i),
    ).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `just test-frontend`
Expected: FAIL — current `JoinPage` shows generic text, no band name, no login/signup links.

- [ ] **Step 4: Rewrite `JoinPage`**

Replace `frontend/src/pages/JoinPage.tsx` with:

```tsx
import {AudioLines} from 'lucide-react';
import {Link, useNavigate, useParams} from 'react-router';
import {useMe} from '../hooks/auth';
import {useJoinByLink, useLinkPreview} from '../hooks/invites';

export default function JoinPage() {
  const {token} = useParams();
  const navigate = useNavigate();
  const me = useMe();
  const preview = useLinkPreview(token ?? '');
  const join = useJoinByLink();

  const card = (children: React.ReactNode) => (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col gap-6">
        <span className="bg-primary text-primary-content grid size-14 place-items-center rounded-box shadow-lg">
          <AudioLines className="size-8" strokeWidth={2.25} />
        </span>
        <div className="card bg-base-100 border-base-300/60 w-full border p-6 text-center shadow-xl">
          {children}
        </div>
      </div>
    </main>
  );

  if (!token) {
    return card(<p>Invalid invite link.</p>);
  }
  if (preview.isPending || me.isPending) {
    return card(
      <span className="loading loading-spinner mx-auto" aria-label="Loading" />,
    );
  }
  if (preview.isError) {
    return card(
      <p className="text-base-content/70">
        This invite link is invalid or has expired.
      </p>,
    );
  }

  const bandName = preview.data.bandName;
  const isLoggedIn = !me.isError && me.data !== undefined;
  const redirect = `?redirect=${encodeURIComponent(`/join/${token}`)}`;

  return card(
    <div className="flex flex-col gap-4">
      <p className="text-lg">
        You&apos;ve been invited to join{' '}
        <span className="font-display font-bold">{bandName}</span>.
      </p>
      {join.error && (
        <div role="alert" className="alert alert-error">
          {join.error.message}
        </div>
      )}
      {isLoggedIn ? (
        <button
          className="btn btn-primary"
          disabled={join.isPending}
          onClick={() =>
            join.mutate(token, {
              onSuccess: ({bandId}) => void navigate(`/bands/${bandId}`),
            })
          }
        >
          Join
        </button>
      ) : (
        <div className="flex flex-col gap-2">
          <Link className="btn btn-primary" to={`/login${redirect}`}>
            Log in
          </Link>
          <Link className="btn btn-ghost" to={`/signup${redirect}`}>
            Sign up
          </Link>
        </div>
      )}
    </div>,
  );
}
```

- [ ] **Step 5: Move the route out of the auth shell**

In `frontend/src/App.tsx`, remove the `/join/:token` route from inside the `<Route element={<Layout />}>` block, and add it to the public routes alongside `/login` and `/signup`:

Remove this line from the Layout block:

```tsx
            <Route path="/join/:token" element={<JoinPage />} />
```

Add it after the `/reset-password` route (still inside `<Routes>`, outside `RequireAuth`):

```tsx
        <Route path="/join/:token" element={<JoinPage />} />
```

(The `import JoinPage` line stays.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `just test-frontend`
Expected: PASS.

- [ ] **Step 7: Format and commit**

```bash
just format
git add frontend/src/hooks/invites.ts frontend/src/App.tsx frontend/src/pages/JoinPage.tsx frontend/src/pages/JoinPage.test.tsx
git commit -m "feat(invites): public join page that names the band and survives auth"
```

---

## Task 7: Band rehearsal list hooks + `BandSongRow`

A reusable row mirroring the Library's `SongRow`, with a Rehearsed action for editors.

**Files:**
- Modify: `frontend/src/hooks/bandsongs.ts` (add list-scoped log/undo hooks)
- Create: `frontend/src/components/bands/BandSongRow.tsx`

- [ ] **Step 1: Add list-scoped rehearsal hooks**

In `frontend/src/hooks/bandsongs.ts`, add after `useLogBandRehearsal`:

```ts
// List-scoped variants: the band view logs/undoes a rehearsal for any row
// without opening the song. Payload carries the songId.
export function useLogBandRehearsalInList(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<RehearsalStats, ApiError, {songId: number; date: string}>({
    mutationFn: ({songId, date}) =>
      api.put<RehearsalStats>(
        `/api/bands/${bandId}/songs/${songId}/rehearsal`,
        {date},
      ),
    onSuccess: (_data, {songId}) =>
      invalidateBandSong(queryClient, bandId, songId),
  });
}

export function useUndoBandRehearsalInList(bandId: number) {
  const queryClient = useQueryClient();
  return useMutation<RehearsalStats, ApiError, {songId: number; date: string}>({
    mutationFn: ({songId, date}) =>
      api.delete<RehearsalStats>(
        `/api/bands/${bandId}/songs/${songId}/rehearsal/${date}`,
      ),
    onSuccess: (_data, {songId}) =>
      invalidateBandSong(queryClient, bandId, songId),
  });
}
```

- [ ] **Step 2: Create `BandSongRow`**

Create `frontend/src/components/bands/BandSongRow.tsx`. This mirrors `frontend/src/components/songs/SongRow.tsx` (read it for the exact classes), but links to the band-song detail, labels the metric "rehearsed", and gates the action on `canEdit`:

```tsx
import {CircleCheck, Clock} from 'lucide-react';
import {Link} from 'react-router';
import {localToday} from '../../lib/dates';
import type {SongListItem} from '../../lib/types';
import StatusBadge from '../songs/StatusBadge';

export default function BandSongRow({
  bandId,
  song,
  canEdit,
  onRehearsed,
}: {
  bandId: number;
  song: SongListItem;
  canEdit: boolean;
  onRehearsed: (songId: number, date: string) => void;
}) {
  return (
    <li className="group border-base-300/60 bg-base-100 hover:border-base-300 flex items-center gap-3 rounded-box border p-3 transition-all hover:shadow-md sm:gap-4 sm:p-4">
      <Link
        to={`/bands/${bandId}/songs/${song.id}`}
        className="min-w-0 flex-1"
      >
        <span className="font-display group-hover:text-primary block truncate text-base font-semibold transition-colors">
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
        {canEdit && (
          <button
            className="btn btn-primary btn-sm gap-1.5"
            onClick={() => onRehearsed(song.id, localToday())}
          >
            <CircleCheck className="size-4" />
            Rehearsed
          </button>
        )}
      </div>
    </li>
  );
}
```

- [ ] **Step 3: Verify it compiles (typecheck)**

Run: `just typecheck`
Expected: PASS (no consumer yet; this confirms the hooks and component type-check).

- [ ] **Step 4: Commit**

```bash
just format
git add frontend/src/hooks/bandsongs.ts frontend/src/components/bands/BandSongRow.tsx
git commit -m "feat(bands): list-scoped rehearsal hooks and BandSongRow"
```

---

## Task 8: `BandSongList` — Rehearsed button, undo toast, action-oriented restyle

Make the band song list feel like the Library: actionable rows with one-tap Rehearsed + undo.

**Files:**
- Modify: `frontend/src/components/bands/BandSongList.tsx`
- Test: `frontend/src/components/bands/BandSongList.test.tsx`

- [ ] **Step 1: Write the failing tests**

Add two tests to `frontend/src/components/bands/BandSongList.test.tsx`. The existing `beforeEach` mock returns the song list on GET and `{id: 9}` on POST; extend it to also handle the rehearsal PUT. Replace the `beforeEach` mock's `if (init?.method === 'POST')` block by adding a PUT branch above the POST one:

```ts
          if (init?.method === 'PUT') {
            return Promise.resolve(
              jsonResponse(200, {lastRehearsedAt: '2026-06-17', rehearsalCount: 1}),
            );
          }
```

Then add:

```ts
  it('lets editors log a rehearsal with undo', async () => {
    renderWithProviders(<BandSongList bandId={3} canEdit={true} />);
    await screen.findByRole('link', {name: /wonderwall/i});
    await userEvent.click(screen.getByRole('button', {name: /rehearsed/i}));
    await waitFor(() => {
      const calls = vi.mocked(fetch).mock.calls;
      expect(
        calls.some(
          ([input, init]) =>
            String(input).endsWith('/api/bands/3/songs/1/rehearsal') &&
            init?.method === 'PUT',
        ),
      ).toBe(true);
    });
    expect(
      await screen.findByRole('button', {name: /undo/i}),
    ).toBeInTheDocument();
  });

  it('hides the rehearsed button from viewers', async () => {
    renderWithProviders(<BandSongList bandId={3} canEdit={false} />);
    await screen.findByRole('link', {name: /wonderwall/i});
    expect(
      screen.queryByRole('button', {name: /rehearsed/i}),
    ).not.toBeInTheDocument();
  });
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test-frontend`
Expected: FAIL — no Rehearsed button exists yet.

- [ ] **Step 3: Rewrite `BandSongList`**

Replace `frontend/src/components/bands/BandSongList.tsx` with (mirrors `HomePage`'s undo/error toast pattern):

```tsx
import {Plus} from 'lucide-react';
import {useEffect, useState} from 'react';
import BandSongRow from './BandSongRow';
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
            <BandSongRow
              key={song.id}
              bandId={bandId}
              song={song}
              canEdit={canEdit}
              onRehearsed={rehearsed}
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
          <div className="alert alert-success">
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
```

Note: this drops the outer `card bg-base-100 shadow` wrapper (the "settings page" feel) in favor of action-oriented rows; `StatusBadge` is now used via `BandSongRow`, so the direct import is removed.

- [ ] **Step 4: Run tests to verify they pass**

Run: `just test-frontend`
Expected: PASS — including the existing "lists band songs" and "lets editors add a band song" tests (link role/name, Add song button, and add flow are unchanged).

- [ ] **Step 5: Format and commit**

```bash
just format
git add frontend/src/components/bands/BandSongList.tsx frontend/src/components/bands/BandSongList.test.tsx
git commit -m "feat(bands): one-tap Rehearsed button with undo in the band song list"
```

---

## Task 9: Full verification

- [ ] **Step 1: Run the whole check suite**

Run: `just check`
Expected: all of lint-go, lint-js, typecheck, format-check, test-go, test-frontend PASS.

- [ ] **Step 2: Build the production bundle**

Run: `just build`
Expected: succeeds (compiles the frontend, validating the new components/CSS classes).

- [ ] **Step 3: Manual smoke (optional, `just dev`)**

- Create an invite link in a band (Invites → Create invite link), open it in a logged-out session: the page should name the band and offer Log in / Sign up.
- Through Sign up, confirm you land back on `/join/:token` and the Join button joins the band.
- In the band view, click Rehearsed on a song row; confirm the undo toast and that the rehearsal count updates on the song's detail page.

- [ ] **Step 4: Push (updates PR #7)**

```bash
git push
```

---

## Self-review notes

- **Spec coverage:** Feature 1 → Tasks 7–8. Feature 2 → Tasks 1–2 (backend) + Task 6 Step 1 (`useLinkPreview`). Feature 3 → Tasks 3–6. Testing section → tests embedded per task + Task 9. Out-of-scope items (emails, auto-join, direct-invite flow, password-reset redirect) are not implemented.
- **Type consistency:** `useLogBandRehearsalInList`/`useUndoBandRehearsalInList` both take `{songId, date}` and return `RehearsalStats`; `BandSongRow.onRehearsed(songId, date)` matches `rehearsed(songId, date)` in `BandSongList`; `useLinkPreview` returns `{bandName}` consumed as `preview.data.bandName`; `PreviewInviteLink` returns `{"bandName": ...}`; `BandNameByLinkToken` returns `(string, error)`.
- **No placeholders:** every code step is complete and copy-paste-ready.
