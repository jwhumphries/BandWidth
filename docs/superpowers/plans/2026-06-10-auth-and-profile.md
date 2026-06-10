# BandWidth Auth + Profile Implementation Plan (Plan 2 of 5)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Full account system: signup/login/logout with DB-backed cookie sessions, TOTP 2FA with backup codes, profile editing, and optional-SMTP password reset — backend and frontend.

**Architecture:** This plan introduces the persistence layer (GORM + CGO-free SQLite via ncruces/gormlite, WAL mode, AutoMigrate). All auth primitives (argon2id hashing, random tokens, TOTP, backup codes) live in `internal/auth`; persistence in `internal/repository` (one `Repo` struct, methods split by entity); HTTP in `internal/handlers` (one `API` struct holding deps); `internal/middleware.RequireAuth` guards routes. CSRF uses Echo v5's fetch-metadata-aware middleware (browsers send `Sec-Fetch-Site` automatically — no frontend token plumbing). The frontend gains TanStack Query, an api client, auth hooks, route guards, and login/signup/profile/reset pages.

**Tech Stack:** Go 1.26, Echo v5.1.1, GORM + `github.com/ncruces/go-sqlite3/gormlite`, `alexedwards/argon2id`, `pquerna/otp`, `wneessen/go-mail`; React 19 + TanStack Query 5 + `qrcode`; existing just + Dagger CI.

---

## Conventions for the executor

- Repo root: `/Users/john/code/git/BandWidth`, branch off `main` (e.g. `auth-and-profile`).
- Echo v5 API facts (verified against v5.1.1 — do not "fix" these to v4 idioms): handlers are `func(c *echo.Context) error`; `c.Bind(&v)`, `c.Set/Get`, `c.SetCookie`, `c.NoContent` all exist; `middleware.CSRFConfig{...}.ToMiddleware()` returns `(echo.MiddlewareFunc, error)`; the CSRF middleware honors `Sec-Fetch-Site` headers; rate limiting via `middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{Rate, Burst, ExpiresIn}))`.
- Go checks during development may run on the host (`go test ./...`); each task's final verification before commit is host-level, and `just check` gates Tasks 8, 11, and 14.
- Frontend deps are installed with bun at latest (`bun add ...`).
- JSON error responses come from Echo's default error handler (`{"message": "..."}`); handlers return `echo.NewHTTPError(code, message)`.
- Spec deviation (approved during planning): no `BANDWIDTH_COOKIE_SECRET` — sessions are opaque 256-bit random tokens stored hashed in the DB, so no cookie signing is needed.

## File structure being built

```
internal/model/model.go              # User, Session, BackupCode, PasswordReset
internal/repository/repository.go    # Open (gormlite+WAL), AutoMigrate, Repo struct
internal/repository/users.go         # user CRUD + IsDuplicate
internal/repository/sessions.go      # session create/lookup/delete
internal/repository/backupcodes.go   # replace/consume backup codes
internal/repository/passwordresets.go
internal/auth/password.go            # argon2id wrappers
internal/auth/token.go               # NewToken, HashToken
internal/auth/totp.go                # NewTOTPKey, ValidateTOTP
internal/auth/backupcodes.go         # NewBackupCodes
internal/auth/session.go             # cookie name + duration constants
internal/middleware/auth.go          # RequireAuth, CurrentUser
internal/mail/mail.go                # Mailer iface, SMTP impl, disabled no-op
internal/handlers/api.go             # API struct, userResponse, cookie helpers
internal/handlers/auth.go            # Signup, Login, Logout, Features
internal/handlers/account.go         # Me, UpdateMe, ChangePassword
internal/handlers/twofa.go           # TwoFactorSetup/Verify/Disable
internal/handlers/passwordreset.go   # RequestPasswordReset, ConfirmPasswordReset
cmd/bandwidth/main.go                # new config keys
cmd/bandwidth/server.go              # DB open, route registration, CSRF, rate limit
frontend/src/lib/api.ts              # fetch wrapper + ApiError
frontend/src/lib/types.ts            # User type
frontend/src/hooks/auth.ts           # useMe/useLogin/... TanStack Query hooks
frontend/src/components/RequireAuth.tsx
frontend/src/components/Layout.tsx   # navbar + outlet
frontend/src/components/profile/AccountSettings.tsx
frontend/src/components/profile/PasswordSettings.tsx
frontend/src/components/profile/TwoFactorSettings.tsx
frontend/src/pages/{Login,Signup,Profile,ForgotPassword,ResetPassword}Page.tsx
frontend/src/pages/HomePage.tsx      # becomes authed welcome page
frontend/src/test/utils.tsx          # renderWithProviders helper
```

---

### Task 1: Database foundation (models + Open/AutoMigrate)

**Files:**
- Create: `internal/model/model.go`
- Create: `internal/repository/repository.go`
- Test: `internal/repository/repository_test.go`
- Modify: `cmd/bandwidth/main.go` (db_path default)

- [ ] **Step 1: Add dependencies**

```bash
go get gorm.io/gorm@latest github.com/ncruces/go-sqlite3/gormlite@latest github.com/ncruces/go-sqlite3@latest
```

- [ ] **Step 2: Write `internal/model/model.go`**

```go
// Package model holds the persisted domain types.
package model

import "time"

// User is an account holder.
type User struct {
	ID              uint   `gorm:"primarykey"`
	Username        string `gorm:"uniqueIndex;not null"`
	Email           string `gorm:"uniqueIndex;not null"`
	PasswordHash    string `gorm:"not null"`
	TOTPSecret      string
	TOTPConfirmedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TOTPEnabled reports whether 2FA is fully enrolled (secret set and verified).
func (u *User) TOTPEnabled() bool {
	return u.TOTPSecret != "" && u.TOTPConfirmedAt != nil
}

// Session is a logged-in browser session; the cookie holds the raw token,
// the row holds its SHA-256.
type Session struct {
	ID        uint      `gorm:"primarykey"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

// BackupCode is a one-time 2FA recovery code (stored hashed).
type BackupCode struct {
	ID       uint   `gorm:"primarykey"`
	UserID   uint   `gorm:"index;not null"`
	CodeHash string `gorm:"not null"`
	UsedAt   *time.Time
}

// PasswordReset is a single-use, expiring reset token (stored hashed).
type PasswordReset struct {
	ID        uint      `gorm:"primarykey"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time
}
```

- [ ] **Step 3: Write the failing repository test**

`internal/repository/repository_test.go`:

```go
package repository

import "testing"

// testRepo returns a Repo backed by a fresh in-memory database.
func testRepo(t *testing.T) *Repo {
	t.Helper()
	repo, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}

func TestOpenMigratesSchema(t *testing.T) {
	repo := testRepo(t)

	for _, table := range []string{"users", "sessions", "backup_codes", "password_resets"} {
		var n int64
		if err := repo.db.Table(table).Count(&n).Error; err != nil {
			t.Errorf("table %s not migrated: %v", table, err)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/repository/`
Expected: FAIL — `undefined: Repo` / `undefined: Open`.

- [ ] **Step 5: Write `internal/repository/repository.go`**

```go
// Package repository is the persistence layer over SQLite/GORM.
package repository

import (
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/embed" // embedded WASM SQLite
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// Repo provides all database operations.
type Repo struct {
	db *gorm.DB
}

// Open opens (creating if needed) the SQLite database at path, enables WAL,
// and migrates the schema. Use ":memory:" for tests.
func Open(path string) (*Repo, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		path,
	)
	db, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Session{},
		&model.BackupCode{},
		&model.PasswordReset{},
	); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return &Repo{db: db}, nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/repository/`
Expected: PASS. (If gormlite's import path or `Open` signature differs, check `go doc github.com/ncruces/go-sqlite3/gormlite` and adapt minimally; report the deviation.)

- [ ] **Step 7: Add the db_path config default**

In `cmd/bandwidth/main.go`, `initConfig()`, add after the existing defaults:

```go
	viper.SetDefault("db_path", "data/bandwidth.db")
```

- [ ] **Step 8: Verify everything builds, commit**

Run: `go build ./... && go test ./...`

```bash
git add go.mod go.sum internal/model/ internal/repository/ cmd/bandwidth/main.go
git commit -m "feat: database foundation with gorm and sqlite"
```

---

### Task 2: Auth primitives (passwords, tokens, backup codes, session constants)

**Files:**
- Create: `internal/auth/password.go`, `internal/auth/token.go`, `internal/auth/backupcodes.go`, `internal/auth/session.go`
- Test: `internal/auth/auth_test.go`

- [ ] **Step 1: Add dependency**

```bash
go get github.com/alexedwards/argon2id@latest
```

- [ ] **Step 2: Write the failing tests**

`internal/auth/auth_test.go`:

```go
package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q, want argon2id format", hash)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Error("correct password rejected")
	}
	if VerifyPassword("wrong password", hash) {
		t.Error("wrong password accepted")
	}
	if VerifyPassword("anything", "not-a-hash") {
		t.Error("garbage hash accepted")
	}
}

func TestNewTokenIsRandomAndHashable(t *testing.T) {
	a, b := NewToken(), NewToken()
	if a == b {
		t.Fatal("two tokens are identical")
	}
	if len(a) < 40 {
		t.Fatalf("token too short: %d chars", len(a))
	}
	if HashToken(a) == HashToken(b) {
		t.Error("different tokens hash identically")
	}
	if HashToken(a) != HashToken(a) {
		t.Error("hash is not deterministic")
	}
}

func TestNewBackupCodes(t *testing.T) {
	codes := NewBackupCodes()
	if len(codes) != 10 {
		t.Fatalf("got %d codes, want 10", len(codes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 9 || c[4] != '-' {
			t.Errorf("code %q not in XXXX-XXXX format", c)
		}
		if seen[c] {
			t.Errorf("duplicate code %q", c)
		}
		seen[c] = true
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/auth/`
Expected: FAIL — undefined: HashPassword (etc.).

- [ ] **Step 4: Write the implementations**

`internal/auth/password.go`:

```go
// Package auth holds authentication primitives: password hashing, random
// tokens, TOTP, and backup codes.
package auth

import "github.com/alexedwards/argon2id"

// HashPassword hashes a password with argon2id default parameters.
func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

// VerifyPassword reports whether password matches the stored hash.
func VerifyPassword(password, hash string) bool {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	return err == nil && match
}
```

`internal/auth/token.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken returns a 256-bit random token in URL-safe base64.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken returns the hex SHA-256 of a token for storage. Tokens are
// high-entropy, so a fast unsalted hash is sufficient.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

`internal/auth/backupcodes.go`:

```go
package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const backupCodeCount = 10

// NewBackupCodes returns one-time recovery codes like "4F7K-Q2ML".
// The alphabet omits 0/O and 1/I to avoid transcription mistakes.
func NewBackupCodes() []string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codes := make([]string, backupCodeCount)
	for i := range codes {
		buf := make([]byte, 8)
		for j := range buf {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				panic(err) // crypto/rand failure is unrecoverable
			}
			buf[j] = alphabet[n.Int64()]
		}
		codes[i] = fmt.Sprintf("%s-%s", buf[:4], buf[4:])
	}
	return codes
}
```

`internal/auth/session.go`:

```go
package auth

import "time"

// SessionCookieName is the HTTP cookie carrying the raw session token.
const SessionCookieName = "bandwidth_session"

// SessionDuration is how long a session stays valid.
const SessionDuration = 30 * 24 * time.Hour
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/auth/`
Expected: PASS (argon2id tests take ~1s; that's the KDF working).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/auth/
git commit -m "feat: auth primitives - argon2id, tokens, backup codes"
```

---

### Task 3: User + session repositories

**Files:**
- Create: `internal/repository/users.go`, `internal/repository/sessions.go`
- Test: `internal/repository/users_test.go`, `internal/repository/sessions_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/repository/users_test.go`:

```go
package repository

import "testing"

func TestCreateAndFindUser(t *testing.T) {
	repo := testRepo(t)

	user, err := repo.CreateUser("alice", "alice@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("user ID not assigned")
	}

	byName, err := repo.UserByLogin("alice")
	if err != nil || byName.ID != user.ID {
		t.Errorf("UserByLogin(username) = %v, %v", byName, err)
	}
	byEmail, err := repo.UserByLogin("alice@example.com")
	if err != nil || byEmail.ID != user.ID {
		t.Errorf("UserByLogin(email) = %v, %v", byEmail, err)
	}
	byID, err := repo.UserByID(user.ID)
	if err != nil || byID.Username != "alice" {
		t.Errorf("UserByID = %v, %v", byID, err)
	}
	if _, err := repo.UserByLogin("nobody"); err == nil {
		t.Error("UserByLogin(nobody) should fail")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	repo := testRepo(t)
	if _, err := repo.CreateUser("alice", "alice@example.com", "h"); err != nil {
		t.Fatal(err)
	}

	_, err := repo.CreateUser("alice", "other@example.com", "h")
	if !IsDuplicate(err) {
		t.Errorf("duplicate username: IsDuplicate = false, err = %v", err)
	}
	_, err = repo.CreateUser("bob", "alice@example.com", "h")
	if !IsDuplicate(err) {
		t.Errorf("duplicate email: IsDuplicate = false, err = %v", err)
	}
	if IsDuplicate(nil) {
		t.Error("IsDuplicate(nil) = true")
	}
}

func TestSaveUser(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	user.Email = "new@example.com"
	if err := repo.SaveUser(user); err != nil {
		t.Fatalf("SaveUser: %v", err)
	}
	again, _ := repo.UserByID(user.ID)
	if again.Email != "new@example.com" {
		t.Errorf("email = %q after save", again.Email)
	}
}
```

`internal/repository/sessions_test.go`:

```go
package repository

import (
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestSessionLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	token, err := repo.CreateSession(user.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := repo.SessionUser(token)
	if err != nil || got.ID != user.ID {
		t.Fatalf("SessionUser = %v, %v", got, err)
	}
	if _, err := repo.SessionUser("bogus-token"); err == nil {
		t.Error("bogus token accepted")
	}

	if err := repo.DeleteSession(token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := repo.SessionUser(token); err == nil {
		t.Error("deleted session still valid")
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	token, _ := repo.CreateSession(user.ID)

	// Force the session into the past.
	repo.db.Model(&model.Session{}).
		Where("user_id = ?", user.ID).
		Update("expires_at", time.Now().Add(-time.Minute))

	if _, err := repo.SessionUser(token); err == nil {
		t.Error("expired session accepted")
	}
}

func TestDeleteUserSessions(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	t1, _ := repo.CreateSession(user.ID)
	t2, _ := repo.CreateSession(user.ID)

	if err := repo.DeleteUserSessions(user.ID); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}
	if _, err := repo.SessionUser(t1); err == nil {
		t.Error("session 1 survived")
	}
	if _, err := repo.SessionUser(t2); err == nil {
		t.Error("session 2 survived")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repository/`
Expected: FAIL — undefined: CreateUser (etc.).

- [ ] **Step 3: Write the implementations**

`internal/repository/users.go`:

```go
package repository

import (
	"strings"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// IsDuplicate reports whether err is a unique-constraint violation.
func IsDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// CreateUser inserts a new user.
func (r *Repo) CreateUser(username, email, passwordHash string) (*model.User, error) {
	user := &model.User{Username: username, Email: email, PasswordHash: passwordHash}
	if err := r.db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// UserByLogin finds a user by username or email.
func (r *Repo) UserByLogin(login string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ? OR email = ?", login, login).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UserByID finds a user by primary key.
func (r *Repo) UserByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// SaveUser persists changes to an existing user.
func (r *Repo) SaveUser(user *model.User) error {
	return r.db.Save(user).Error
}
```

`internal/repository/sessions.go`:

```go
package repository

import (
	"time"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// CreateSession stores a new session and returns the raw token for the cookie.
func (r *Repo) CreateSession(userID uint) (string, error) {
	token := auth.NewToken()
	session := &model.Session{
		TokenHash: auth.HashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(auth.SessionDuration),
	}
	if err := r.db.Create(session).Error; err != nil {
		return "", err
	}
	return token, nil
}

// SessionUser returns the user owning an unexpired session token.
func (r *Repo) SessionUser(token string) (*model.User, error) {
	var session model.Session
	err := r.db.
		Where("token_hash = ? AND expires_at > ?", auth.HashToken(token), time.Now()).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return r.UserByID(session.UserID)
}

// DeleteSession removes a session by raw token.
func (r *Repo) DeleteSession(token string) error {
	return r.db.Where("token_hash = ?", auth.HashToken(token)).
		Delete(&model.Session{}).Error
}

// DeleteUserSessions removes every session belonging to a user.
func (r *Repo) DeleteUserSessions(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.Session{}).Error
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/repository/`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/repository/
git commit -m "feat: user and session repositories"
```

---

### Task 4: Backup code + password reset repositories

**Files:**
- Create: `internal/repository/backupcodes.go`, `internal/repository/passwordresets.go`
- Test: `internal/repository/recovery_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/repository/recovery_test.go`:

```go
package repository

import (
	"testing"
	"time"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestBackupCodes(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	codes := []string{"AAAA-BBBB", "CCCC-DDDD"}
	if err := repo.ReplaceBackupCodes(user.ID, codes); err != nil {
		t.Fatalf("ReplaceBackupCodes: %v", err)
	}

	if !repo.ConsumeBackupCode(user.ID, "AAAA-BBBB") {
		t.Error("valid code rejected")
	}
	if repo.ConsumeBackupCode(user.ID, "AAAA-BBBB") {
		t.Error("code consumed twice")
	}
	if repo.ConsumeBackupCode(user.ID, "XXXX-YYYY") {
		t.Error("unknown code accepted")
	}

	// Replacing wipes old codes.
	if err := repo.ReplaceBackupCodes(user.ID, []string{"EEEE-FFFF"}); err != nil {
		t.Fatal(err)
	}
	if repo.ConsumeBackupCode(user.ID, "CCCC-DDDD") {
		t.Error("old code survived replacement")
	}

	if err := repo.DeleteBackupCodes(user.ID); err != nil {
		t.Fatalf("DeleteBackupCodes: %v", err)
	}
	if repo.ConsumeBackupCode(user.ID, "EEEE-FFFF") {
		t.Error("code survived deletion")
	}
}

func TestPasswordResetLifecycle(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")

	token, err := repo.CreatePasswordReset(user.ID)
	if err != nil {
		t.Fatalf("CreatePasswordReset: %v", err)
	}

	gotID, err := repo.ConsumePasswordReset(token)
	if err != nil || gotID != user.ID {
		t.Fatalf("ConsumePasswordReset = %d, %v", gotID, err)
	}
	// Single use.
	if _, err := repo.ConsumePasswordReset(token); err == nil {
		t.Error("reset token consumed twice")
	}
	if _, err := repo.ConsumePasswordReset("bogus"); err == nil {
		t.Error("bogus reset token accepted")
	}
}

func TestExpiredPasswordResetRejected(t *testing.T) {
	repo := testRepo(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	token, _ := repo.CreatePasswordReset(user.ID)

	repo.db.Model(&model.PasswordReset{}).
		Where("user_id = ?", user.ID).
		Update("expires_at", time.Now().Add(-time.Minute))

	if _, err := repo.ConsumePasswordReset(token); err == nil {
		t.Error("expired reset token accepted")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repository/`
Expected: FAIL — undefined: ReplaceBackupCodes (etc.).

- [ ] **Step 3: Write the implementations**

`internal/repository/backupcodes.go`:

```go
package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

// ReplaceBackupCodes deletes any existing codes and stores hashes of the new set.
func (r *Repo) ReplaceBackupCodes(userID uint, codes []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).
			Delete(&model.BackupCode{}).Error; err != nil {
			return err
		}
		for _, code := range codes {
			bc := &model.BackupCode{UserID: userID, CodeHash: auth.HashToken(code)}
			if err := tx.Create(bc).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ConsumeBackupCode marks an unused matching code as used, reporting success.
func (r *Repo) ConsumeBackupCode(userID uint, code string) bool {
	res := r.db.Model(&model.BackupCode{}).
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL",
			userID, auth.HashToken(code)).
		Update("used_at", time.Now())
	return res.Error == nil && res.RowsAffected == 1
}

// DeleteBackupCodes removes all of a user's backup codes.
func (r *Repo) DeleteBackupCodes(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&model.BackupCode{}).Error
}
```

`internal/repository/passwordresets.go`:

```go
package repository

import (
	"time"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
)

const passwordResetDuration = time.Hour

// CreatePasswordReset stores a reset token and returns its raw value.
func (r *Repo) CreatePasswordReset(userID uint) (string, error) {
	token := auth.NewToken()
	reset := &model.PasswordReset{
		TokenHash: auth.HashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(passwordResetDuration),
	}
	if err := r.db.Create(reset).Error; err != nil {
		return "", err
	}
	return token, nil
}

// ConsumePasswordReset marks a valid token used and returns its user ID.
func (r *Repo) ConsumePasswordReset(token string) (uint, error) {
	var reset model.PasswordReset
	err := r.db.
		Where("token_hash = ? AND expires_at > ? AND used_at IS NULL",
			auth.HashToken(token), time.Now()).
		First(&reset).Error
	if err != nil {
		return 0, err
	}
	now := time.Now()
	reset.UsedAt = &now
	if err := r.db.Save(&reset).Error; err != nil {
		return 0, err
	}
	return reset.UserID, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/repository/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/
git commit -m "feat: backup code and password reset repositories"
```

---

### Task 5: TOTP wrapper

**Files:**
- Create: `internal/auth/totp.go`
- Test: `internal/auth/totp_test.go`

- [ ] **Step 1: Add dependency**

```bash
go get github.com/pquerna/otp@latest
```

- [ ] **Step 2: Write the failing test**

`internal/auth/totp_test.go`:

```go
package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestTOTPGenerateAndValidate(t *testing.T) {
	key, err := NewTOTPKey("alice")
	if err != nil {
		t.Fatalf("NewTOTPKey: %v", err)
	}
	if key.Secret == "" {
		t.Fatal("empty secret")
	}
	if !strings.Contains(key.URL, "BandWidth") || !strings.Contains(key.URL, "alice") {
		t.Errorf("URL %q missing issuer or account", key.URL)
	}

	code, err := totp.GenerateCode(key.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !ValidateTOTP(code, key.Secret) {
		t.Error("current code rejected")
	}
	if ValidateTOTP("000000", key.Secret) {
		t.Error("wrong code accepted") // 1-in-a-million flake; rerun if it ever fires
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/auth/`
Expected: FAIL — undefined: NewTOTPKey.

- [ ] **Step 4: Write `internal/auth/totp.go`**

```go
package auth

import "github.com/pquerna/otp/totp"

// TOTPKey is a newly generated TOTP enrollment.
type TOTPKey struct {
	Secret string // base32 secret for manual entry
	URL    string // otpauth:// URL for QR codes
}

// NewTOTPKey generates a TOTP secret for the given account name.
func NewTOTPKey(account string) (*TOTPKey, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "BandWidth",
		AccountName: account,
	})
	if err != nil {
		return nil, err
	}
	return &TOTPKey{Secret: key.Secret(), URL: key.URL()}, nil
}

// ValidateTOTP reports whether code is currently valid for secret.
func ValidateTOTP(code, secret string) bool {
	return totp.Validate(code, secret)
}
```

- [ ] **Step 5: Run test to verify it passes, commit**

Run: `go test ./internal/auth/`
Expected: PASS.

```bash
git add go.mod go.sum internal/auth/
git commit -m "feat: totp generation and validation"
```

---

### Task 6: Auth middleware + GET /api/me

**Files:**
- Create: `internal/middleware/auth.go`, `internal/handlers/api.go`, `internal/handlers/account.go`
- Test: `internal/middleware/auth_test.go`

- [ ] **Step 1: Write the failing test**

`internal/middleware/auth_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

func newAuthedServer(t *testing.T) (*echo.Echo, *repository.Repo) {
	t.Helper()
	repo, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.GET("/protected", func(c *echo.Context) error {
		return c.String(http.StatusOK, CurrentUser(c).Username)
	}, RequireAuth(repo))
	return e, repo
}

func TestRequireAuth(t *testing.T) {
	e, repo := newAuthedServer(t)
	user, _ := repo.CreateUser("alice", "alice@example.com", "h")
	token, _ := repo.CreateSession(user.ID)

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
		wantBody   string
	}{
		{name: "no cookie", cookie: nil, wantStatus: http.StatusUnauthorized},
		{
			name:       "bogus token",
			cookie:     &http.Cookie{Name: auth.SessionCookieName, Value: "bogus"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid session",
			cookie:     &http.Cookie{Name: auth.SessionCookieName, Value: token},
			wantStatus: http.StatusOK,
			wantBody:   "alice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/`
Expected: FAIL — undefined: RequireAuth / CurrentUser.

- [ ] **Step 3: Write `internal/middleware/auth.go`**

```go
// Package middleware holds application HTTP middleware.
package middleware

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

const userContextKey = "user"

// RequireAuth loads the session user from the cookie or rejects with 401.
func RequireAuth(repo *repository.Repo) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			cookie, err := c.Request().Cookie(auth.SessionCookieName)
			if err != nil || cookie.Value == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			user, err := repo.SessionUser(cookie.Value)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			c.Set(userContextKey, user)
			return next(c)
		}
	}
}

// CurrentUser returns the authenticated user stored by RequireAuth.
func CurrentUser(c *echo.Context) *model.User {
	user, _ := c.Get(userContextKey).(*model.User)
	return user
}
```

- [ ] **Step 4: Write the API struct and Me handler**

`internal/handlers/api.go`:

```go
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/mail"
	"github.com/jwhumphries/bandwidth/internal/model"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// API holds the dependencies shared by all HTTP handlers.
type API struct {
	Repo          *repository.Repo
	Mailer        mail.Mailer
	BaseURL       string
	SecureCookies bool
}

func userResponse(u *model.User) map[string]any {
	return map[string]any{
		"id":          u.ID,
		"username":    u.Username,
		"email":       u.Email,
		"totpEnabled": u.TOTPEnabled(),
	}
}

func (a *API) setSessionCookie(c *echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}
```

NOTE: `internal/mail` does not exist until Task 10. To keep this task compiling, create the minimal `internal/mail/mail.go` now (Task 10 fills in the SMTP implementation):

```go
// Package mail sends transactional email.
package mail

import "fmt"

// Mailer sends transactional email. When not configured it is disabled and
// dependent features (password reset) are hidden.
type Mailer interface {
	Enabled() bool
	Send(to, subject, body string) error
}

// Disabled is a Mailer that refuses to send.
type Disabled struct{}

// Enabled always reports false.
func (Disabled) Enabled() bool { return false }

// Send always fails.
func (Disabled) Send(string, string, string) error {
	return fmt.Errorf("mail is not configured")
}
```

`internal/handlers/account.go`:

```go
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// Me returns the authenticated user.
func (a *API) Me(c *echo.Context) error {
	return c.JSON(http.StatusOK, userResponse(appmw.CurrentUser(c)))
}
```

- [ ] **Step 5: Run tests, commit**

Run: `go test ./... && go build ./...`
Expected: PASS.

```bash
git add internal/middleware/ internal/handlers/ internal/mail/
git commit -m "feat: auth middleware and me endpoint"
```

---

### Task 7: Signup, login (with TOTP), logout handlers

**Files:**
- Create: `internal/handlers/auth.go`
- Test: `internal/handlers/auth_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/auth_test.go`:

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pquerna/otp/totp"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/mail"
	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// newTestAPI wires an API with an in-memory DB and the auth routes used in tests.
func newTestAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	repo, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	api := &API{Repo: repo, Mailer: mail.Disabled{}, BaseURL: "http://test"}
	e := echo.New()
	e.POST("/api/auth/signup", api.Signup)
	e.POST("/api/auth/login", api.Login)
	e.POST("/api/auth/logout", api.Logout)
	e.GET("/api/me", api.Me, appmw.RequireAuth(repo))
	return e, api
}

func postJSON(e *echo.Echo, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestSignup(t *testing.T) {
	e, _ := newTestAPI(t)

	rec := postJSON(e, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookie(t, rec)
	if !cookie.HttpOnly {
		t.Error("session cookie not HttpOnly")
	}

	// The session works.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(cookie)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, req)
	if mrec.Code != http.StatusOK || !strings.Contains(mrec.Body.String(), "alice") {
		t.Fatalf("me after signup: %d %s", mrec.Code, mrec.Body.String())
	}
}

func TestSignupValidation(t *testing.T) {
	e, _ := newTestAPI(t)

	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "short password", body: `{"username":"a","email":"a@b.c","password":"short"}`, want: 400},
		{name: "missing username", body: `{"email":"a@b.c","password":"hunter2hunter2"}`, want: 400},
		{name: "bad email", body: `{"username":"a","email":"nope","password":"hunter2hunter2"}`, want: 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := postJSON(e, "/api/auth/signup", tt.body); rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}

	// Duplicate username → 409.
	postJSON(e, "/api/auth/signup", `{"username":"dup","email":"dup@x.com","password":"hunter2hunter2"}`)
	rec := postJSON(e, "/api/auth/signup", `{"username":"dup","email":"other@x.com","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate signup status = %d, want 409", rec.Code)
	}
}

func TestLoginLogout(t *testing.T) {
	e, _ := newTestAPI(t)
	postJSON(e, "/api/auth/signup", `{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`)

	// Wrong password.
	rec := postJSON(e, "/api/auth/login", `{"login":"alice","password":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", rec.Code)
	}
	// Unknown user.
	rec = postJSON(e, "/api/auth/login", `{"login":"nobody","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user: %d", rec.Code)
	}
	// By username and by email.
	for _, login := range []string{"alice", "alice@example.com"} {
		rec = postJSON(e, "/api/auth/login",
			fmt.Sprintf(`{"login":%q,"password":"hunter2hunter2"}`, login))
		if rec.Code != http.StatusOK {
			t.Fatalf("login as %q: %d %s", login, rec.Code, rec.Body.String())
		}
	}
	cookie := sessionCookie(t, rec)

	// Logout invalidates the session.
	lrec := postJSON(e, "/api/auth/logout", "", cookie)
	if lrec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", lrec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(cookie)
	mrec := httptest.NewRecorder()
	e.ServeHTTP(mrec, req)
	if mrec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d, want 401", mrec.Code)
	}
}

func TestLoginWithTOTP(t *testing.T) {
	e, api := newTestAPI(t)
	postJSON(e, "/api/auth/signup", `{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`)

	// Enroll 2FA directly through the repo.
	user, _ := api.Repo.UserByLogin("alice")
	key, _ := auth.NewTOTPKey("alice")
	now := time.Now()
	user.TOTPSecret = key.Secret
	user.TOTPConfirmedAt = &now
	if err := api.Repo.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if err := api.Repo.ReplaceBackupCodes(user.ID, []string{"AAAA-BBBB"}); err != nil {
		t.Fatal(err)
	}

	// Password alone → 401 with totpRequired flag.
	rec := postJSON(e, "/api/auth/login", `{"login":"alice","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["totpRequired"] != true {
		t.Fatalf("body = %v, want totpRequired true", body)
	}

	// Wrong code → 401 without flag.
	rec = postJSON(e, "/api/auth/login", `{"login":"alice","password":"hunter2hunter2","totpCode":"000000"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code: %d", rec.Code)
	}

	// Valid TOTP code.
	code, _ := totp.GenerateCode(key.Secret, time.Now())
	rec = postJSON(e, "/api/auth/login",
		fmt.Sprintf(`{"login":"alice","password":"hunter2hunter2","totpCode":%q}`, code))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid code: %d %s", rec.Code, rec.Body.String())
	}

	// Backup code works once (case-insensitive).
	rec = postJSON(e, "/api/auth/login", `{"login":"alice","password":"hunter2hunter2","totpCode":"aaaa-bbbb"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("backup code: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(e, "/api/auth/login", `{"login":"alice","password":"hunter2hunter2","totpCode":"aaaa-bbbb"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused backup code: %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/`
Expected: FAIL — undefined: (*API).Signup (etc.).

- [ ] **Step 3: Write `internal/handlers/auth.go`**

```go
package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

type signupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Signup creates an account and logs the new user in.
func (a *API) Signup(c *echo.Context) error {
	var req signupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Username == "" || len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"username and a password of at least 8 characters are required")
	}
	if !strings.Contains(req.Email, "@") {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	user, err := a.Repo.CreateUser(req.Username, req.Email, hash)
	if err != nil {
		if repository.IsDuplicate(err) {
			return echo.NewHTTPError(http.StatusConflict, "username or email already taken")
		}
		return err
	}

	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusCreated, userResponse(user))
}

type loginRequest struct {
	Login    string `json:"login"` // username or email
	Password string `json:"password"`
	TOTPCode string `json:"totpCode"`
}

// Login authenticates by password, then by TOTP or backup code when enrolled.
func (a *API) Login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, err := a.Repo.UserByLogin(strings.TrimSpace(req.Login))
	if err != nil || !auth.VerifyPassword(req.Password, user.PasswordHash) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	if user.TOTPEnabled() {
		if req.TOTPCode == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"message":      "two-factor code required",
				"totpRequired": true,
			})
		}
		code := strings.ToUpper(strings.TrimSpace(req.TOTPCode))
		if !auth.ValidateTOTP(req.TOTPCode, user.TOTPSecret) &&
			!a.Repo.ConsumeBackupCode(user.ID, code) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid two-factor code")
		}
	}

	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusOK, userResponse(user))
}

// Logout deletes the session and clears the cookie.
func (a *API) Logout(c *echo.Context) error {
	if cookie, err := c.Request().Cookie(auth.SessionCookieName); err == nil {
		_ = a.Repo.DeleteSession(cookie.Value)
	}
	a.clearSessionCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// Features reports which optional features are available to the frontend.
func (a *API) Features(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{
		"passwordReset": a.Mailer.Enabled(),
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/`
Expected: PASS (argon2id makes these take a few seconds).

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/
git commit -m "feat: signup, login with totp, logout handlers"
```

---

### Task 8: Server wiring — DB, CSRF, rate limiting, routes

**Files:**
- Modify: `cmd/bandwidth/main.go` (config defaults)
- Modify: `cmd/bandwidth/server.go` (wire everything)
- Test: `cmd/bandwidth/server_test.go` (integration flow)

- [ ] **Step 1: Add config defaults**

In `cmd/bandwidth/main.go` `initConfig()`, the full set of defaults becomes:

```go
func initConfig() {
	viper.SetDefault("port", ":8080")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("db_path", "data/bandwidth.db")
	viper.SetDefault("secure_cookies", false)
	viper.SetDefault("base_url", "http://localhost:3000")
	viper.SetDefault("smtp_host", "")
	viper.SetDefault("smtp_port", 587)
	viper.SetDefault("smtp_user", "")
	viper.SetDefault("smtp_pass", "")
	viper.SetDefault("smtp_from", "")
	viper.SetEnvPrefix("BANDWIDTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}
```

- [ ] **Step 2: Write the failing integration test**

Replace `cmd/bandwidth/server_test.go` with:

```go
package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/handlers"
	"github.com/jwhumphries/bandwidth/internal/mail"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

func testServer(t *testing.T) *echo.Echo {
	t.Helper()
	repo, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	api := &handlers.API{Repo: repo, Mailer: mail.Disabled{}, BaseURL: "http://test"}
	e, err := newEcho(slog.New(slog.NewTextHandler(io.Discard, nil)), api, repo)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// do issues a request with same-origin fetch metadata (what browsers send).
func do(e *echo.Echo, method, path, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestNewEchoServesHealthz(t *testing.T) {
	e := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSignupLoginMeFlow(t *testing.T) {
	e := testServer(t)

	rec := do(e, http.MethodPost, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()

	mrec := do(e, http.MethodGet, "/api/me", "", cookies)
	if mrec.Code != http.StatusOK || !strings.Contains(mrec.Body.String(), "alice") {
		t.Fatalf("me: %d %s", mrec.Code, mrec.Body.String())
	}
}

func TestCSRFRejectsCrossSite(t *testing.T) {
	e := testServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup",
		strings.NewReader(`{"username":"x","email":"x@y.z","password":"hunter2hunter2"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST status = %d, want 403", rec.Code)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	e := testServer(t)
	rec := do(e, http.MethodGet, "/api/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./cmd/bandwidth/`
Expected: FAIL — `newEcho` has the wrong signature (compile error).

- [ ] **Step 4: Rewire `cmd/bandwidth/server.go`**

Replace `runServer` and `newEcho` (keep `newLogger` and `requestLogger` as they are):

```go
func runServer() error {
	logger := newLogger(viper.GetString("log_level"))

	repo, err := repository.Open(viper.GetString("db_path"))
	if err != nil {
		return err
	}
	mailer := mail.New(mail.Config{
		Host: viper.GetString("smtp_host"),
		Port: viper.GetInt("smtp_port"),
		User: viper.GetString("smtp_user"),
		Pass: viper.GetString("smtp_pass"),
		From: viper.GetString("smtp_from"),
	})
	if mailer.Enabled() {
		logger.Info("smtp configured, password reset enabled")
	}
	api := &handlers.API{
		Repo:          repo,
		Mailer:        mailer,
		BaseURL:       viper.GetString("base_url"),
		SecureCookies: viper.GetBool("secure_cookies"),
	}

	e, err := newEcho(logger, api, repo)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              viper.GetString("port"),
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// ... (the signal/shutdown block is unchanged from the current file)
}

func newEcho(logger *slog.Logger, api *handlers.API, repo *repository.Repo) (*echo.Echo, error) {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(requestLogger(logger))

	e.GET("/healthz", handlers.Healthz)

	csrfMW, err := middleware.CSRFConfig{
		CookiePath:     "/",
		CookieSameSite: http.SameSiteLaxMode,
		CookieSecure:   api.SecureCookies,
	}.ToMiddleware()
	if err != nil {
		return nil, err
	}

	apiGroup := e.Group("/api", csrfMW)

	authLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: 1, Burst: 5, ExpiresIn: 3 * time.Minute},
	))
	authGroup := apiGroup.Group("/auth")
	authGroup.POST("/signup", api.Signup, authLimiter)
	authGroup.POST("/login", api.Login, authLimiter)
	authGroup.POST("/logout", api.Logout)
	authGroup.GET("/features", api.Features)
	authGroup.POST("/password-reset", api.RequestPasswordReset, authLimiter)
	authGroup.POST("/password-reset/confirm", api.ConfirmPasswordReset, authLimiter)

	twofa := apiGroup.Group("/auth/2fa", appmw.RequireAuth(repo))
	twofa.POST("/setup", api.TwoFactorSetup)
	twofa.POST("/verify", api.TwoFactorVerify)
	twofa.POST("/disable", api.TwoFactorDisable)

	me := apiGroup.Group("/me", appmw.RequireAuth(repo))
	me.GET("", api.Me)
	me.PATCH("", api.UpdateMe)
	me.PUT("/password", api.ChangePassword)

	dist, err := fs.Sub(static.Dist, "dist")
	if err != nil {
		return nil, err
	}
	handlers.RegisterSPA(e, dist)
	return e, nil
}
```

Imports to add in server.go: `"github.com/jwhumphries/bandwidth/internal/handlers"`, `"github.com/jwhumphries/bandwidth/internal/mail"`, `appmw "github.com/jwhumphries/bandwidth/internal/middleware"`, `"github.com/jwhumphries/bandwidth/internal/repository"`. The `panic(err)` for `fs.Sub` becomes a returned error now that `newEcho` returns one.

IMPORTANT — this task references handlers that don't exist until Tasks 9–11 (`UpdateMe`, `ChangePassword`, `TwoFactorSetup/Verify/Disable`, `RequestPasswordReset`, `ConfirmPasswordReset`, `mail.New`). To keep the build green, create them as minimal stubs NOW in their final files, and let Tasks 9–11 replace the stubs (each stub returns 501):

`internal/handlers/account.go` — append:

```go
// UpdateMe is implemented in the account task.
func (a *API) UpdateMe(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// ChangePassword is implemented in the account task.
func (a *API) ChangePassword(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
```

`internal/handlers/twofa.go` — create:

```go
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// TwoFactorSetup is implemented in the 2FA task.
func (a *API) TwoFactorSetup(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// TwoFactorVerify is implemented in the 2FA task.
func (a *API) TwoFactorVerify(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// TwoFactorDisable is implemented in the 2FA task.
func (a *API) TwoFactorDisable(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
```

`internal/handlers/passwordreset.go` — create:

```go
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// RequestPasswordReset is implemented in the password reset task.
func (a *API) RequestPasswordReset(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}

// ConfirmPasswordReset is implemented in the password reset task.
func (a *API) ConfirmPasswordReset(c *echo.Context) error {
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
```

`internal/mail/mail.go` — append the constructor and config (still no SMTP yet; Task 10 adds it):

```go
// Config holds SMTP settings; empty Host or From disables mail.
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// New returns an SMTP-backed Mailer, or Disabled when unconfigured.
func New(cfg Config) Mailer {
	if cfg.Host == "" || cfg.From == "" {
		return Disabled{}
	}
	return newSMTP(cfg)
}

// newSMTP is replaced with a real implementation in the mail task.
func newSMTP(cfg Config) Mailer {
	_ = cfg
	return Disabled{}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./...`
Expected: PASS, including the CSRF cross-site rejection test. If `TestCSRFRejectsCrossSite` fails because Echo v5's CSRF treats the header differently, inspect with `go doc github.com/labstack/echo/v5/middleware CSRFConfig` and adjust ONLY the test's expectations to the middleware's actual contract (e.g., different rejection status); report the deviation.

- [ ] **Step 6: Manual smoke test**

```bash
go run ./cmd/bandwidth & sleep 2 && \
curl -s -H "Sec-Fetch-Site: same-origin" -H "Content-Type: application/json" \
  -d '{"username":"smoke","email":"smoke@test.dev","password":"hunter2hunter2"}' \
  localhost:8080/api/auth/signup; kill %1
rm -rf data/
```
Expected: a JSON user object. (`rm -rf data/` removes the smoke-test DB; data/ is gitignored.)

- [ ] **Step 7: Run the full gate and commit**

Run: `just check`
Expected: `all checks passed`.

```bash
git add cmd/ internal/
git commit -m "feat: wire database, csrf, rate limiting, and auth routes"
```

---

### Task 9: Account endpoints — PATCH /api/me, PUT /api/me/password

**Files:**
- Modify: `internal/handlers/account.go` (replace stubs)
- Test: `internal/handlers/account_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/account_test.go`:

```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// signupAndCookie creates a user via the API and returns their session cookie.
func signupAndCookie(t *testing.T, e *echo.Echo, username string) *http.Cookie {
	t.Helper()
	rec := postJSON(e, "/api/auth/signup",
		`{"username":"`+username+`","email":"`+username+`@example.com","password":"hunter2hunter2"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
	}
	return sessionCookie(t, rec)
}

func newAccountAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	e.PATCH("/api/me", api.UpdateMe, appmw.RequireAuth(api.Repo))
	e.PUT("/api/me/password", api.ChangePassword, appmw.RequireAuth(api.Repo))
	return e, api
}

func jsonReq(e *echo.Echo, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestUpdateMe(t *testing.T) {
	e, api := newAccountAPI(t)
	cookie := signupAndCookie(t, e, "alice")
	signupAndCookie(t, e, "bob")

	rec := jsonReq(e, http.MethodPatch, "/api/me",
		`{"username":"alice2","email":"alice2@example.com"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "alice2") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	user, err := api.Repo.UserByLogin("alice2")
	if err != nil || user.Email != "alice2@example.com" {
		t.Fatalf("persisted user: %v, %v", user, err)
	}

	// Taking bob's username → 409.
	rec = jsonReq(e, http.MethodPatch, "/api/me", `{"username":"bob"}`, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict: %d, want 409", rec.Code)
	}
	// Empty username → 400.
	rec = jsonReq(e, http.MethodPatch, "/api/me", `{"username":"  "}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty username: %d, want 400", rec.Code)
	}
}

func TestChangePassword(t *testing.T) {
	e, _ := newAccountAPI(t)
	cookie := signupAndCookie(t, e, "alice")

	// Wrong current password.
	rec := jsonReq(e, http.MethodPut, "/api/me/password",
		`{"currentPassword":"wrong","newPassword":"newpassword99"}`, cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong current: %d, want 401", rec.Code)
	}
	// Too-short new password.
	rec = jsonReq(e, http.MethodPut, "/api/me/password",
		`{"currentPassword":"hunter2hunter2","newPassword":"short"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short new: %d, want 400", rec.Code)
	}
	// Success.
	rec = jsonReq(e, http.MethodPut, "/api/me/password",
		`{"currentPassword":"hunter2hunter2","newPassword":"newpassword99"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	// Old sessions are revoked; the response set a fresh cookie.
	newCookie := sessionCookie(t, rec)
	if rec := jsonReq(e, http.MethodPatch, "/api/me", `{"username":"x"}`, cookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old session after password change: %d, want 401", rec.Code)
	}
	if rec := jsonReq(e, http.MethodPatch, "/api/me", `{"username":"alice9"}`, newCookie); rec.Code != http.StatusOK {
		t.Fatalf("new session: %d", rec.Code)
	}
	// New password logs in.
	if rec := postJSON(e, "/api/auth/login", `{"login":"alice9","password":"newpassword99"}`); rec.Code != http.StatusOK {
		t.Fatalf("login with new password: %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/`
Expected: FAIL — the stub handlers return 501, so assertions fail.

- [ ] **Step 3: Replace the stubs in `internal/handlers/account.go`**

The full file becomes:

```go
package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// Me returns the authenticated user.
func (a *API) Me(c *echo.Context) error {
	return c.JSON(http.StatusOK, userResponse(appmw.CurrentUser(c)))
}

type updateMeRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
}

// UpdateMe updates the authenticated user's username and/or email.
func (a *API) UpdateMe(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	var req updateMeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if username == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "username cannot be empty")
		}
		user.Username = username
	}
	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		if !strings.Contains(email, "@") {
			return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
		}
		user.Email = email
	}
	if err := a.Repo.SaveUser(user); err != nil {
		if repository.IsDuplicate(err) {
			return echo.NewHTTPError(http.StatusConflict, "username or email already taken")
		}
		return err
	}
	return c.JSON(http.StatusOK, userResponse(user))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePassword verifies the current password, sets the new one, and
// rotates every session (a fresh one is issued for this browser).
func (a *API) ChangePassword(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	var req changePasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !auth.VerifyPassword(req.CurrentPassword, user.PasswordHash) {
		return echo.NewHTTPError(http.StatusUnauthorized, "current password is incorrect")
	}
	if len(req.NewPassword) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"new password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	if err := a.Repo.DeleteUserSessions(user.ID); err != nil {
		return err
	}
	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusOK, userResponse(user))
}
```

- [ ] **Step 4: Run tests to verify they pass, commit**

Run: `go test ./internal/handlers/`
Expected: PASS.

```bash
git add internal/handlers/
git commit -m "feat: profile update and password change endpoints"
```

---

### Task 10: 2FA endpoints

**Files:**
- Modify: `internal/handlers/twofa.go` (replace stubs)
- Test: `internal/handlers/twofa_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/twofa_test.go`:

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pquerna/otp/totp"

	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

func newTwoFAAPI(t *testing.T) (*echo.Echo, *API) {
	t.Helper()
	e, api := newTestAPI(t)
	g := e.Group("/api/auth/2fa", appmw.RequireAuth(api.Repo))
	g.POST("/setup", api.TwoFactorSetup)
	g.POST("/verify", api.TwoFactorVerify)
	g.POST("/disable", api.TwoFactorDisable)
	return e, api
}

func TestTwoFactorEnrollment(t *testing.T) {
	e, api := newTwoFAAPI(t)
	cookie := signupAndCookie(t, e, "alice")

	// Setup returns a secret and otpauth URL.
	rec := jsonReq(e, http.MethodPost, "/api/auth/2fa/setup", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	var setup struct {
		Secret     string `json:"secret"`
		OtpauthURL string `json:"otpauthUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &setup); err != nil || setup.Secret == "" {
		t.Fatalf("setup body: %s (%v)", rec.Body.String(), err)
	}

	// Not yet enabled (pending verification).
	user, _ := api.Repo.UserByLogin("alice")
	if user.TOTPEnabled() {
		t.Fatal("enabled before verify")
	}

	// Wrong verify code → 400.
	rec = jsonReq(e, http.MethodPost, "/api/auth/2fa/verify", `{"code":"000000"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad verify code: %d, want 400", rec.Code)
	}

	// Correct code → enabled, returns 10 backup codes.
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	rec = jsonReq(e, http.MethodPost, "/api/auth/2fa/verify",
		fmt.Sprintf(`{"code":%q}`, code), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	var verify struct {
		BackupCodes []string `json:"backupCodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &verify); err != nil || len(verify.BackupCodes) != 10 {
		t.Fatalf("backup codes: %s (%v)", rec.Body.String(), err)
	}
	user, _ = api.Repo.UserByLogin("alice")
	if !user.TOTPEnabled() {
		t.Fatal("not enabled after verify")
	}

	// Setup again while enabled → 400.
	rec = jsonReq(e, http.MethodPost, "/api/auth/2fa/setup", "", cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("re-setup while enabled: %d, want 400", rec.Code)
	}

	// Disable with a wrong code → 400; with a backup code → 200.
	rec = jsonReq(e, http.MethodPost, "/api/auth/2fa/disable", `{"code":"000000"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("disable wrong code: %d, want 400", rec.Code)
	}
	rec = jsonReq(e, http.MethodPost, "/api/auth/2fa/disable",
		fmt.Sprintf(`{"code":%q}`, verify.BackupCodes[0]), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body.String())
	}
	user, _ = api.Repo.UserByLogin("alice")
	if user.TOTPEnabled() || user.TOTPSecret != "" {
		t.Fatal("still enabled after disable")
	}
}

func TestTwoFactorVerifyWithoutSetup(t *testing.T) {
	e, _ := newTwoFAAPI(t)
	cookie := signupAndCookie(t, e, "bob")

	rec := jsonReq(e, http.MethodPost, "/api/auth/2fa/verify", `{"code":"123456"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify without setup: %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/`
Expected: FAIL — stubs return 501.

- [ ] **Step 3: Replace `internal/handlers/twofa.go`**

```go
package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	appmw "github.com/jwhumphries/bandwidth/internal/middleware"
)

// TwoFactorSetup generates a pending TOTP secret for the user.
func (a *API) TwoFactorSetup(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user.TOTPEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest,
			"two-factor authentication is already enabled")
	}
	key, err := auth.NewTOTPKey(user.Username)
	if err != nil {
		return err
	}
	user.TOTPSecret = key.Secret
	user.TOTPConfirmedAt = nil
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{
		"secret":     key.Secret,
		"otpauthUrl": key.URL,
	})
}

type twoFactorCodeRequest struct {
	Code string `json:"code"`
}

// TwoFactorVerify confirms a pending enrollment and returns backup codes.
func (a *API) TwoFactorVerify(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if user.TOTPSecret == "" || user.TOTPEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest, "no pending two-factor enrollment")
	}
	var req twoFactorCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !auth.ValidateTOTP(strings.TrimSpace(req.Code), user.TOTPSecret) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid two-factor code")
	}

	now := time.Now()
	user.TOTPConfirmedAt = &now
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	codes := auth.NewBackupCodes()
	if err := a.Repo.ReplaceBackupCodes(user.ID, codes); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"backupCodes": codes})
}

// TwoFactorDisable turns 2FA off after validating a TOTP or backup code.
func (a *API) TwoFactorDisable(c *echo.Context) error {
	user := appmw.CurrentUser(c)
	if !user.TOTPEnabled() {
		return echo.NewHTTPError(http.StatusBadRequest,
			"two-factor authentication is not enabled")
	}
	var req twoFactorCodeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if !auth.ValidateTOTP(req.Code, user.TOTPSecret) &&
		!a.Repo.ConsumeBackupCode(user.ID, code) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid two-factor code")
	}

	user.TOTPSecret = ""
	user.TOTPConfirmedAt = nil
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	if err := a.Repo.DeleteBackupCodes(user.ID); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, userResponse(user))
}
```

- [ ] **Step 4: Run tests to verify they pass, commit**

Run: `go test ./internal/handlers/`
Expected: PASS.

```bash
git add internal/handlers/
git commit -m "feat: two-factor setup, verify, and disable endpoints"
```

---

### Task 11: Mailer + password reset endpoints

**Files:**
- Modify: `internal/mail/mail.go` (real SMTP implementation)
- Modify: `internal/handlers/passwordreset.go` (replace stubs)
- Test: `internal/mail/mail_test.go`, `internal/handlers/passwordreset_test.go`

- [ ] **Step 1: Add dependency**

```bash
go get github.com/wneessen/go-mail@latest
```

- [ ] **Step 2: Write the failing tests**

`internal/mail/mail_test.go`:

```go
package mail

import "testing"

func TestNewDisabledWhenUnconfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "empty", cfg: Config{}, want: false},
		{name: "host only", cfg: Config{Host: "smtp.example.com"}, want: false},
		{name: "from only", cfg: Config{From: "x@example.com"}, want: false},
		{
			name: "host and from",
			cfg:  Config{Host: "smtp.example.com", From: "x@example.com", Port: 587},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.cfg).Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisabledSendFails(t *testing.T) {
	if err := (Disabled{}).Send("a@b.c", "s", "b"); err == nil {
		t.Error("Disabled.Send should error")
	}
}
```

`internal/handlers/passwordreset_test.go`:

```go
package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/mail"
)

// fakeMailer records sent mail for assertions.
type fakeMailer struct {
	mu   sync.Mutex
	to   []string
	sent []string // bodies
}

func (f *fakeMailer) Enabled() bool { return true }
func (f *fakeMailer) Send(to, _, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.to = append(f.to, to)
	f.sent = append(f.sent, body)
	return nil
}

func registerResetRoutes(e *echo.Echo, api *API) {
	e.POST("/api/auth/password-reset", api.RequestPasswordReset)
	e.POST("/api/auth/password-reset/confirm", api.ConfirmPasswordReset)
}

func TestPasswordResetDisabledReturns404(t *testing.T) {
	e, api := newTestAPI(t)
	api.Mailer = mail.Disabled{}
	registerResetRoutes(e, api)

	rec := postJSON(e, "/api/auth/password-reset", `{"email":"a@b.c"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("request: %d, want 404", rec.Code)
	}
	rec = postJSON(e, "/api/auth/password-reset/confirm", `{"token":"x","newPassword":"longenough99"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("confirm: %d, want 404", rec.Code)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	e, api := newTestAPI(t)
	mailer := &fakeMailer{}
	api.Mailer = mailer
	api.BaseURL = "http://app.test"
	registerResetRoutes(e, api)

	postJSON(e, "/api/auth/signup", `{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`)

	// Unknown email: still 204, no mail sent (no account enumeration).
	rec := postJSON(e, "/api/auth/password-reset", `{"email":"nobody@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unknown email: %d, want 204", rec.Code)
	}
	if len(mailer.sent) != 0 {
		t.Fatalf("mail sent for unknown email: %v", mailer.to)
	}

	// Known email: 204 and a mail containing the reset link.
	rec = postJSON(e, "/api/auth/password-reset", `{"email":"alice@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("known email: %d, want 204", rec.Code)
	}
	if len(mailer.sent) != 1 || mailer.to[0] != "alice@example.com" {
		t.Fatalf("mail not sent correctly: to=%v", mailer.to)
	}
	tokenRe := regexp.MustCompile(`http://app\.test/reset-password\?token=([A-Za-z0-9_-]+)`)
	m := tokenRe.FindStringSubmatch(mailer.sent[0])
	if m == nil {
		t.Fatalf("no reset link in mail body: %q", mailer.sent[0])
	}
	token := m[1]

	// Bad token → 400.
	rec = postJSON(e, "/api/auth/password-reset/confirm", `{"token":"bogus","newPassword":"newpassword99"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bogus token: %d, want 400", rec.Code)
	}
	// Short password → 400.
	rec = postJSON(e, "/api/auth/password-reset/confirm",
		fmt.Sprintf(`{"token":%q,"newPassword":"short"}`, token))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password: %d, want 400", rec.Code)
	}
	// Valid → 204; new password works; token is single-use.
	rec = postJSON(e, "/api/auth/password-reset/confirm",
		fmt.Sprintf(`{"token":%q,"newPassword":"newpassword99"}`, token))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("confirm: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(e, "/api/auth/login", `{"login":"alice","password":"newpassword99"}`); rec.Code != http.StatusOK {
		t.Fatalf("login with new password: %d", rec.Code)
	}
	rec = postJSON(e, "/api/auth/password-reset/confirm",
		fmt.Sprintf(`{"token":%q,"newPassword":"anotherpass99"}`, token))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("token reuse: %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/mail/ ./internal/handlers/`
Expected: mail tests PASS already (Disabled exists); handler tests FAIL on the 501 stubs.

- [ ] **Step 4: Implement SMTP in `internal/mail/mail.go`**

Replace the `newSMTP` stub with a real implementation; the full file becomes:

```go
// Package mail sends transactional email.
package mail

import (
	"fmt"

	gomail "github.com/wneessen/go-mail"
)

// Mailer sends transactional email. When not configured it is disabled and
// dependent features (password reset) are hidden.
type Mailer interface {
	Enabled() bool
	Send(to, subject, body string) error
}

// Disabled is a Mailer that refuses to send.
type Disabled struct{}

// Enabled always reports false.
func (Disabled) Enabled() bool { return false }

// Send always fails.
func (Disabled) Send(string, string, string) error {
	return fmt.Errorf("mail is not configured")
}

// Config holds SMTP settings; empty Host or From disables mail.
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// New returns an SMTP-backed Mailer, or Disabled when unconfigured.
func New(cfg Config) Mailer {
	if cfg.Host == "" || cfg.From == "" {
		return Disabled{}
	}
	return &smtpMailer{cfg: cfg}
}

type smtpMailer struct{ cfg Config }

func (m *smtpMailer) Enabled() bool { return true }

func (m *smtpMailer) Send(to, subject, body string) error {
	msg := gomail.NewMsg()
	if err := msg.From(m.cfg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, body)

	opts := []gomail.Option{
		gomail.WithPort(m.cfg.Port),
		gomail.WithTLSPortPolicy(gomail.TLSOpportunistic),
	}
	if m.cfg.User != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(m.cfg.User),
			gomail.WithPassword(m.cfg.Pass),
		)
	}
	client, err := gomail.NewClient(m.cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	return client.DialAndSend(msg)
}
```

(If the go-mail API differs — e.g. option names — check `go doc github.com/wneessen/go-mail` and adapt minimally; report the deviation.)

- [ ] **Step 5: Replace `internal/handlers/passwordreset.go`**

```go
package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
)

// RequestPasswordReset emails a reset link. Always 204 for enabled mailers
// (no account enumeration); 404 when mail is not configured.
func (a *API) RequestPasswordReset(c *echo.Context) error {
	if !a.Mailer.Enabled() {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if user, err := a.Repo.UserByLogin(email); err == nil {
		if token, err := a.Repo.CreatePasswordReset(user.ID); err == nil {
			link := fmt.Sprintf("%s/reset-password?token=%s", a.BaseURL, token)
			_ = a.Mailer.Send(user.Email, "Reset your BandWidth password",
				"Someone (hopefully you) asked to reset your BandWidth password.\n\n"+
					"Reset it within the next hour: "+link+"\n\n"+
					"If this wasn't you, ignore this email.")
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// ConfirmPasswordReset sets a new password from a valid reset token and
// revokes all existing sessions.
func (a *API) ConfirmPasswordReset(c *echo.Context) error {
	if !a.Mailer.Enabled() {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(req.NewPassword) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"new password must be at least 8 characters")
	}

	userID, err := a.Repo.ConsumePasswordReset(req.Token)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid or expired reset token")
	}
	user, err := a.Repo.UserByID(userID)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	if err := a.Repo.SaveUser(user); err != nil {
		return err
	}
	if err := a.Repo.DeleteUserSessions(user.ID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 6: Run tests, full gate, commit**

Run: `go test ./...` then `just check`
Expected: PASS / `all checks passed`.

```bash
git add go.mod go.sum internal/mail/ internal/handlers/
git commit -m "feat: smtp mailer and password reset endpoints"
```

---

### Task 12: Frontend foundation — TanStack Query, api client, auth hooks, login/signup

**Files:**
- Create: `frontend/src/lib/api.ts`, `frontend/src/lib/types.ts`, `frontend/src/hooks/auth.ts`
- Create: `frontend/src/components/RequireAuth.tsx`, `frontend/src/components/Layout.tsx`
- Create: `frontend/src/pages/LoginPage.tsx`, `frontend/src/pages/SignupPage.tsx`
- Create: `frontend/src/test/utils.tsx`
- Modify: `frontend/src/main.tsx`, `frontend/src/App.tsx`, `frontend/src/pages/HomePage.tsx`
- Test: `frontend/src/lib/api.test.ts`, `frontend/src/pages/LoginPage.test.tsx`, `frontend/src/components/RequireAuth.test.tsx`
- Delete: the old fetch-badge tests in `frontend/src/pages/HomePage.test.tsx` (replaced)

- [ ] **Step 1: Add dependencies**

```bash
cd frontend && bun add @tanstack/react-query && cd ..
```

- [ ] **Step 2: Write the api client and types**

`frontend/src/lib/api.ts`:

```ts
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public body: unknown = null,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    headers: {'Content-Type': 'application/json', ...init.headers},
    ...init,
  });
  if (!res.ok) {
    let message = res.statusText;
    let body: unknown = null;
    try {
      body = await res.json();
      const m = (body as {message?: unknown}).message;
      if (typeof m === 'string') message = m;
    } catch {
      // non-JSON error body; keep statusText
    }
    throw new ApiError(res.status, message, body);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data?: unknown) =>
    request<T>(path, {
      method: 'POST',
      body: data === undefined ? undefined : JSON.stringify(data),
    }),
  patch: <T>(path: string, data: unknown) =>
    request<T>(path, {method: 'PATCH', body: JSON.stringify(data)}),
  put: <T>(path: string, data: unknown) =>
    request<T>(path, {method: 'PUT', body: JSON.stringify(data)}),
};
```

`frontend/src/lib/types.ts`:

```ts
export interface User {
  id: number;
  username: string;
  email: string;
  totpEnabled: boolean;
}

export interface AuthFeatures {
  passwordReset: boolean;
}

export interface TwoFactorSetupResponse {
  secret: string;
  otpauthUrl: string;
}

export interface TwoFactorVerifyResponse {
  backupCodes: string[];
}
```

- [ ] **Step 3: Write the failing api client test**

`frontend/src/lib/api.test.ts`:

```ts
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {api, ApiError} from './api';

describe('api client', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });

  it('returns parsed JSON on success', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({id: 1}), {status: 200}),
    );
    await expect(api.get('/api/me')).resolves.toEqual({id: 1});
  });

  it('returns undefined for 204 responses', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(null, {status: 204}));
    await expect(api.post('/api/auth/logout')).resolves.toBeUndefined();
  });

  it('throws ApiError with server message and body', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({message: 'nope', totpRequired: true}), {
        status: 401,
      }),
    );
    const err = await api.get('/api/me').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(401);
    expect((err as ApiError).message).toBe('nope');
    expect(((err as ApiError).body as {totpRequired: boolean}).totpRequired).toBe(true);
  });

  it('sends JSON bodies with content-type', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({}), {status: 200}),
    );
    await api.post('/api/auth/login', {login: 'a'});
    const [, init] = vi.mocked(fetch).mock.calls[0]!;
    expect(init?.method).toBe('POST');
    expect(init?.body).toBe('{"login":"a"}');
  });
});
```

Run: `cd frontend && bun run test`
Expected: FAIL — cannot resolve `./api` (then passes once Step 2's files exist — if you wrote Step 2 first, this is your green run; the order of Steps 2/3 is acceptable either way as long as you SEE the test pass).

- [ ] **Step 4: Write the auth hooks**

`frontend/src/hooks/auth.ts`:

```ts
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useNavigate} from 'react-router';
import {api, ApiError} from '../lib/api';
import type {
  AuthFeatures,
  TwoFactorSetupResponse,
  TwoFactorVerifyResponse,
  User,
} from '../lib/types';

export function useMe() {
  return useQuery<User, ApiError>({
    queryKey: ['me'],
    queryFn: () => api.get<User>('/api/me'),
    retry: false,
    staleTime: 5 * 60 * 1000,
  });
}

export function useAuthFeatures() {
  return useQuery<AuthFeatures, ApiError>({
    queryKey: ['auth-features'],
    queryFn: () => api.get<AuthFeatures>('/api/auth/features'),
    staleTime: Infinity,
  });
}

export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {login: string; password: string; totpCode?: string}>({
    mutationFn: data => api.post<User>('/api/auth/login', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useSignup() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {username: string; email: string; password: string}>({
    mutationFn: data => api.post<User>('/api/auth/signup', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => api.post<void>('/api/auth/logout'),
    onSettled: () => {
      queryClient.clear();
      void navigate('/login');
    },
  });
}

export function useUpdateMe() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {username?: string; email?: string}>({
    mutationFn: data => api.patch<User>('/api/me', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useChangePassword() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {currentPassword: string; newPassword: string}>({
    mutationFn: data => api.put<User>('/api/me/password', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useTwoFactorSetup() {
  return useMutation<TwoFactorSetupResponse, ApiError, void>({
    mutationFn: () => api.post<TwoFactorSetupResponse>('/api/auth/2fa/setup'),
  });
}

export function useTwoFactorVerify() {
  const queryClient = useQueryClient();
  return useMutation<TwoFactorVerifyResponse, ApiError, {code: string}>({
    mutationFn: data => api.post<TwoFactorVerifyResponse>('/api/auth/2fa/verify', data),
    onSuccess: () => {
      queryClient.setQueryData<User | undefined>(['me'], u =>
        u ? {...u, totpEnabled: true} : u,
      );
    },
  });
}

export function useTwoFactorDisable() {
  const queryClient = useQueryClient();
  return useMutation<User, ApiError, {code: string}>({
    mutationFn: data => api.post<User>('/api/auth/2fa/disable', data),
    onSuccess: user => queryClient.setQueryData(['me'], user),
  });
}

export function useRequestPasswordReset() {
  return useMutation<void, ApiError, {email: string}>({
    mutationFn: data => api.post<void>('/api/auth/password-reset', data),
  });
}

export function useConfirmPasswordReset() {
  return useMutation<void, ApiError, {token: string; newPassword: string}>({
    mutationFn: data => api.post<void>('/api/auth/password-reset/confirm', data),
  });
}
```

- [ ] **Step 5: Write the test helper, guard, and layout**

`frontend/src/test/utils.tsx`:

```tsx
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render} from '@testing-library/react';
import type {ReactElement} from 'react';
import {MemoryRouter} from 'react-router';

export function renderWithProviders(ui: ReactElement, {route = '/'} = {}) {
  const client = new QueryClient({
    defaultOptions: {queries: {retry: false}, mutations: {retry: false}},
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}
```

`frontend/src/components/RequireAuth.tsx`:

```tsx
import {Navigate, Outlet} from 'react-router';
import {useMe} from '../hooks/auth';

export default function RequireAuth() {
  const {isPending, isError} = useMe();
  if (isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <span className="loading loading-spinner loading-lg" aria-label="Loading" />
      </div>
    );
  }
  if (isError) {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}
```

`frontend/src/components/Layout.tsx`:

```tsx
import {Link, Outlet} from 'react-router';
import {useLogout} from '../hooks/auth';

export default function Layout() {
  const logout = useLogout();
  return (
    <div className="bg-base-200 min-h-screen">
      <nav className="navbar bg-base-100 shadow">
        <div className="flex-1">
          <Link to="/" className="btn btn-ghost text-xl">
            BandWidth
          </Link>
        </div>
        <div className="flex-none gap-2">
          <Link to="/profile" className="btn btn-ghost">
            Profile
          </Link>
          <button
            className="btn btn-ghost"
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
          >
            Log out
          </button>
        </div>
      </nav>
      <main className="mx-auto max-w-3xl p-4">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 6: Write the login and signup pages**

`frontend/src/pages/LoginPage.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useNavigate} from 'react-router';
import {useAuthFeatures, useLogin} from '../hooks/auth';

export default function LoginPage() {
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [totpRequired, setTotpRequired] = useState(false);
  const navigate = useNavigate();
  const loginMutation = useLogin();
  const {data: features} = useAuthFeatures();

  const submit = (e: FormEvent) => {
    e.preventDefault();
    loginMutation.mutate(
      {login, password, ...(totpCode ? {totpCode} : {})},
      {
        onSuccess: () => void navigate('/'),
        onError: err => {
          if ((err.body as {totpRequired?: boolean} | null)?.totpRequired) {
            setTotpRequired(true);
          }
        },
      },
    );
  };

  const error =
    loginMutation.error &&
    !(loginMutation.error.body as {totpRequired?: boolean} | null)?.totpRequired
      ? loginMutation.error.message
      : null;

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">BandWidth</h1>
        <form className="card bg-base-100 w-full p-6 shadow" onSubmit={submit}>
          <fieldset className="fieldset">
            <label className="label" htmlFor="login">
              Username or email
            </label>
            <input
              id="login"
              className="input w-full"
              value={login}
              onChange={e => setLogin(e.target.value)}
              required
            />
            <label className="label" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              className="input w-full"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
            />
            {totpRequired && (
              <>
                <label className="label" htmlFor="totp">
                  Two-factor code
                </label>
                <input
                  id="totp"
                  className="input w-full"
                  value={totpCode}
                  onChange={e => setTotpCode(e.target.value)}
                  placeholder="123456 or backup code"
                  autoFocus
                />
              </>
            )}
            {error && (
              <div role="alert" className="alert alert-error mt-2">
                {error}
              </div>
            )}
            <button className="btn btn-primary mt-4" disabled={loginMutation.isPending}>
              Log in
            </button>
          </fieldset>
        </form>
        <p className="text-sm">
          No account?{' '}
          <Link className="link" to="/signup">
            Sign up
          </Link>
          {features?.passwordReset && (
            <>
              {' · '}
              <Link className="link" to="/forgot-password">
                Forgot password?
              </Link>
            </>
          )}
        </p>
      </div>
    </main>
  );
}
```

`frontend/src/pages/SignupPage.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useNavigate} from 'react-router';
import {useSignup} from '../hooks/auth';

export default function SignupPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const navigate = useNavigate();
  const signup = useSignup();

  const submit = (e: FormEvent) => {
    e.preventDefault();
    signup.mutate(
      {username, email, password},
      {onSuccess: () => void navigate('/')},
    );
  };

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">BandWidth</h1>
        <form className="card bg-base-100 w-full p-6 shadow" onSubmit={submit}>
          <fieldset className="fieldset">
            <label className="label" htmlFor="username">
              Username
            </label>
            <input
              id="username"
              className="input w-full"
              value={username}
              onChange={e => setUsername(e.target.value)}
              required
            />
            <label className="label" htmlFor="email">
              Email
            </label>
            <input
              id="email"
              type="email"
              className="input w-full"
              value={email}
              onChange={e => setEmail(e.target.value)}
              required
            />
            <label className="label" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              className="input w-full"
              value={password}
              onChange={e => setPassword(e.target.value)}
              minLength={8}
              required
            />
            {signup.error && (
              <div role="alert" className="alert alert-error mt-2">
                {signup.error.message}
              </div>
            )}
            <button className="btn btn-primary mt-4" disabled={signup.isPending}>
              Sign up
            </button>
          </fieldset>
        </form>
        <p className="text-sm">
          Already have an account?{' '}
          <Link className="link" to="/login">
            Log in
          </Link>
        </p>
      </div>
    </main>
  );
}
```

- [ ] **Step 7: Rewire main.tsx, App.tsx, HomePage.tsx**

`frontend/src/main.tsx`:

```tsx
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router';
import App from './App';
import './index.css';

const queryClient = new QueryClient({
  defaultOptions: {queries: {retry: 1, staleTime: 5 * 60 * 1000}},
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
```

`frontend/src/App.tsx` (ForgotPassword/ResetPassword routes arrive in Task 13 — leave them out for now):

```tsx
import {Route, Routes} from 'react-router';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';
import HomePage from './pages/HomePage';
import LoginPage from './pages/LoginPage';
import SignupPage from './pages/SignupPage';

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<Layout />}>
          <Route path="/" element={<HomePage />} />
        </Route>
      </Route>
    </Routes>
  );
}
```

`frontend/src/pages/HomePage.tsx` (replaces the badge page; its job is done):

```tsx
import {useMe} from '../hooks/auth';

export default function HomePage() {
  const {data: user} = useMe();
  return (
    <div className="hero bg-base-100 rounded-box py-12">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-4xl font-bold">Welcome, {user?.username}</h1>
          <p className="py-4">Your songs will live here soon.</p>
        </div>
      </div>
    </div>
  );
}
```

`frontend/src/pages/HomePage.test.tsx` (replaces the old badge tests):

```tsx
import {screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import HomePage from './HomePage';

describe('HomePage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({id: 1, username: 'alice', email: 'a@b.c', totpEnabled: false}),
          {status: 200},
        ),
      ),
    );
  });

  it('greets the logged-in user', async () => {
    renderWithProviders(<HomePage />);
    await waitFor(() =>
      expect(screen.getByText(/welcome, alice/i)).toBeInTheDocument(),
    );
  });
});
```

- [ ] **Step 8: Write the LoginPage and RequireAuth tests**

`frontend/src/pages/LoginPage.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../test/utils';
import LoginPage from './LoginPage';

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {status});
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        if (String(input).includes('/api/auth/features')) {
          return Promise.resolve(jsonResponse(200, {passwordReset: false}));
        }
        return Promise.resolve(jsonResponse(404, {message: 'not found'}));
      }),
    );
  });

  it('shows an error for bad credentials', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('/api/auth/login')) {
        return Promise.resolve(jsonResponse(401, {message: 'invalid credentials'}));
      }
      return Promise.resolve(jsonResponse(200, {passwordReset: false}));
    });
    renderWithProviders(<LoginPage />);

    await userEvent.type(screen.getByLabelText(/username or email/i), 'alice');
    await userEvent.type(screen.getByLabelText(/password/i), 'wrong');
    await userEvent.click(screen.getByRole('button', {name: /log in/i}));

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(/invalid credentials/i),
    );
  });

  it('reveals the two-factor field when required', async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      if (String(input).includes('/api/auth/login')) {
        return Promise.resolve(
          jsonResponse(401, {message: 'two-factor code required', totpRequired: true}),
        );
      }
      return Promise.resolve(jsonResponse(200, {passwordReset: false}));
    });
    renderWithProviders(<LoginPage />);

    expect(screen.queryByLabelText(/two-factor code/i)).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText(/username or email/i), 'alice');
    await userEvent.type(screen.getByLabelText(/^password$/i), 'hunter2hunter2');
    await userEvent.click(screen.getByRole('button', {name: /log in/i}));

    await waitFor(() =>
      expect(screen.getByLabelText(/two-factor code/i)).toBeInTheDocument(),
    );
    // The "code required" response is a flow signal, not an error banner.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('hides the forgot-password link when the feature is off', async () => {
    renderWithProviders(<LoginPage />);
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(screen.queryByText(/forgot password/i)).not.toBeInTheDocument();
  });
});
```

`frontend/src/components/RequireAuth.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {Route, Routes} from 'react-router';
import {renderWithProviders} from '../test/utils';
import RequireAuth from './RequireAuth';

describe('RequireAuth', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({message: 'authentication required'}), {status: 401}),
      ),
    );
  });

  it('redirects to /login when unauthenticated', async () => {
    renderWithProviders(
      <Routes>
        <Route path="/login" element={<p>login page</p>} />
        <Route element={<RequireAuth />}>
          <Route path="/" element={<p>secret home</p>} />
        </Route>
      </Routes>,
    );
    await waitFor(() => expect(screen.getByText(/login page/i)).toBeInTheDocument());
    expect(screen.queryByText(/secret home/i)).not.toBeInTheDocument();
  });
});
```

Install the user-event dependency used above:

```bash
cd frontend && bun add -d @testing-library/user-event && cd ..
```

- [ ] **Step 9: Run all frontend checks**

Run (from `frontend/`): `bun run test && bun run typecheck && bun run lint && bun run format:check && bun run build`
Expected: all pass. Apply `bun run format` if Prettier wants whitespace-only changes.

- [ ] **Step 10: Commit**

```bash
git add frontend
git commit -m "feat: frontend auth - api client, hooks, login and signup"
```

---

### Task 13: Profile page, 2FA UI, password reset pages

**Files:**
- Create: `frontend/src/pages/ProfilePage.tsx`
- Create: `frontend/src/components/profile/AccountSettings.tsx`, `frontend/src/components/profile/PasswordSettings.tsx`, `frontend/src/components/profile/TwoFactorSettings.tsx`, `frontend/src/components/profile/EnrollTwoFactor.tsx`, `frontend/src/components/profile/DisableTwoFactor.tsx`
- Create: `frontend/src/pages/ForgotPasswordPage.tsx`, `frontend/src/pages/ResetPasswordPage.tsx`
- Modify: `frontend/src/App.tsx` (add routes)
- Test: `frontend/src/components/profile/AccountSettings.test.tsx`, `frontend/src/components/profile/TwoFactorSettings.test.tsx`

- [ ] **Step 1: Add the QR code dependency**

```bash
cd frontend && bun add qrcode && bun add -d @types/qrcode && cd ..
```

- [ ] **Step 2: Write the failing tests**

`frontend/src/components/profile/AccountSettings.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import AccountSettings from './AccountSettings';

const me = {id: 1, username: 'alice', email: 'alice@example.com', totpEnabled: false};

describe('AccountSettings', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.method === 'PATCH') {
          return Promise.resolve(
            new Response(JSON.stringify({...me, username: 'alice2'}), {status: 200}),
          );
        }
        return Promise.resolve(new Response(JSON.stringify(me), {status: 200}));
      }),
    );
  });

  it('prefills the form from the current user and saves changes', async () => {
    renderWithProviders(<AccountSettings />);

    const username = await screen.findByLabelText(/username/i);
    await waitFor(() => expect(username).toHaveValue('alice'));
    expect(screen.getByLabelText(/email/i)).toHaveValue('alice@example.com');

    await userEvent.clear(username);
    await userEvent.type(username, 'alice2');
    await userEvent.click(screen.getByRole('button', {name: /save/i}));

    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent(/saved/i));
  });
});
```

`frontend/src/components/profile/TwoFactorSettings.test.tsx`:

```tsx
import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {renderWithProviders} from '../../test/utils';
import TwoFactorSettings from './TwoFactorSettings';

const me = {id: 1, username: 'alice', email: 'alice@example.com', totpEnabled: false};

describe('TwoFactorSettings', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/2fa/setup')) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                secret: 'SECRET123',
                otpauthUrl: 'otpauth://totp/BandWidth:alice?secret=SECRET123',
              }),
              {status: 200},
            ),
          );
        }
        if (url.includes('/2fa/verify')) {
          return Promise.resolve(
            new Response(JSON.stringify({backupCodes: ['AAAA-BBBB', 'CCCC-DDDD']}), {
              status: 200,
            }),
          );
        }
        return Promise.resolve(new Response(JSON.stringify(me), {status: 200}));
      }),
    );
  });

  it('walks through enrollment and shows backup codes once', async () => {
    renderWithProviders(<TwoFactorSettings />);

    await userEvent.click(await screen.findByRole('button', {name: /enable 2fa/i}));

    // Manual-entry secret appears (QR may render async).
    await waitFor(() => expect(screen.getAllByText(/SECRET123/).length).toBeGreaterThan(0));

    await userEvent.type(screen.getByLabelText(/^code$/i), '123456');
    await userEvent.click(screen.getByRole('button', {name: /confirm/i}));

    await waitFor(() => expect(screen.getByText(/AAAA-BBBB/)).toBeInTheDocument());
    expect(screen.getByText(/will not be shown again/i)).toBeInTheDocument();

    // Dismissing the codes moves to the enabled (disable) state.
    await userEvent.click(screen.getByRole('button', {name: /i saved them/i}));
    await waitFor(() =>
      expect(screen.getByRole('button', {name: /disable 2fa/i})).toBeInTheDocument(),
    );
  });
});
```

Run: `cd frontend && bun run test`
Expected: FAIL — cannot resolve the new components.

- [ ] **Step 3: Write the profile components**

`frontend/src/pages/ProfilePage.tsx`:

```tsx
import AccountSettings from '../components/profile/AccountSettings';
import PasswordSettings from '../components/profile/PasswordSettings';
import TwoFactorSettings from '../components/profile/TwoFactorSettings';

export default function ProfilePage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-bold">Profile</h1>
      <AccountSettings />
      <PasswordSettings />
      <TwoFactorSettings />
    </div>
  );
}
```

`frontend/src/components/profile/AccountSettings.tsx`:

```tsx
import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import {useMe, useUpdateMe} from '../../hooks/auth';

export default function AccountSettings() {
  const {data: user} = useMe();
  const updateMe = useUpdateMe();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (user) {
      setUsername(user.username);
      setEmail(user.email);
    }
  }, [user]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setSaved(false);
    updateMe.mutate({username, email}, {onSuccess: () => setSaved(true)});
  };

  return (
    <section className="card bg-base-100 shadow">
      <form className="card-body" onSubmit={submit}>
        <h2 className="card-title">Account</h2>
        <label className="label" htmlFor="username">
          Username
        </label>
        <input
          id="username"
          className="input w-full"
          value={username}
          onChange={e => setUsername(e.target.value)}
          required
        />
        <label className="label" htmlFor="email">
          Email
        </label>
        <input
          id="email"
          type="email"
          className="input w-full"
          value={email}
          onChange={e => setEmail(e.target.value)}
          required
        />
        {updateMe.error && (
          <div role="alert" className="alert alert-error">
            {updateMe.error.message}
          </div>
        )}
        {saved && (
          <div role="status" className="alert alert-success">
            Saved
          </div>
        )}
        <div className="card-actions justify-end">
          <button className="btn btn-primary" disabled={updateMe.isPending}>
            Save
          </button>
        </div>
      </form>
    </section>
  );
}
```

`frontend/src/components/profile/PasswordSettings.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useChangePassword} from '../../hooks/auth';

export default function PasswordSettings() {
  const changePassword = useChangePassword();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [saved, setSaved] = useState(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setSaved(false);
    changePassword.mutate(
      {currentPassword, newPassword},
      {
        onSuccess: () => {
          setSaved(true);
          setCurrentPassword('');
          setNewPassword('');
        },
      },
    );
  };

  return (
    <section className="card bg-base-100 shadow">
      <form className="card-body" onSubmit={submit}>
        <h2 className="card-title">Password</h2>
        <label className="label" htmlFor="current-password">
          Current password
        </label>
        <input
          id="current-password"
          type="password"
          className="input w-full"
          value={currentPassword}
          onChange={e => setCurrentPassword(e.target.value)}
          required
        />
        <label className="label" htmlFor="new-password">
          New password
        </label>
        <input
          id="new-password"
          type="password"
          className="input w-full"
          value={newPassword}
          onChange={e => setNewPassword(e.target.value)}
          minLength={8}
          required
        />
        {changePassword.error && (
          <div role="alert" className="alert alert-error">
            {changePassword.error.message}
          </div>
        )}
        {saved && (
          <div role="status" className="alert alert-success">
            Password changed
          </div>
        )}
        <div className="card-actions justify-end">
          <button className="btn btn-primary" disabled={changePassword.isPending}>
            Change password
          </button>
        </div>
      </form>
    </section>
  );
}
```

`frontend/src/components/profile/TwoFactorSettings.tsx`:

```tsx
import {useState} from 'react';
import {useMe} from '../../hooks/auth';
import DisableTwoFactor from './DisableTwoFactor';
import EnrollTwoFactor from './EnrollTwoFactor';

export default function TwoFactorSettings() {
  const {data: user} = useMe();
  const [backupCodes, setBackupCodes] = useState<string[] | null>(null);

  if (!user) {
    return null;
  }
  if (backupCodes) {
    return (
      <section className="card bg-base-100 shadow">
        <div className="card-body">
          <h2 className="card-title">Two-factor authentication enabled</h2>
          <p>
            Save these one-time backup codes somewhere safe — they will not be
            shown again.
          </p>
          <pre className="bg-base-200 rounded-box p-4">{backupCodes.join('\n')}</pre>
          <div className="card-actions justify-end">
            <button className="btn" onClick={() => setBackupCodes(null)}>
              I saved them
            </button>
          </div>
        </div>
      </section>
    );
  }
  return user.totpEnabled ? (
    <DisableTwoFactor />
  ) : (
    <EnrollTwoFactor onEnrolled={setBackupCodes} />
  );
}
```

`frontend/src/components/profile/EnrollTwoFactor.tsx`:

```tsx
import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import QRCode from 'qrcode';
import {useTwoFactorSetup, useTwoFactorVerify} from '../../hooks/auth';

export default function EnrollTwoFactor({
  onEnrolled,
}: {
  onEnrolled: (codes: string[]) => void;
}) {
  const setup = useTwoFactorSetup();
  const verify = useTwoFactorVerify();
  const [code, setCode] = useState('');
  const [qr, setQr] = useState<string | null>(null);

  useEffect(() => {
    if (setup.data) {
      QRCode.toDataURL(setup.data.otpauthUrl)
        .then(setQr)
        .catch(() => setQr(null));
    }
  }, [setup.data]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    verify.mutate({code}, {onSuccess: data => onEnrolled(data.backupCodes)});
  };

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Two-factor authentication</h2>
        {setup.data ? (
          <form onSubmit={submit}>
            <p>
              Scan the QR code with your authenticator app, then enter a code
              to confirm.
            </p>
            {qr && <img src={qr} alt="TOTP enrollment QR code" className="mx-auto my-4" />}
            <p className="text-sm">
              Manual entry secret:{' '}
              <span className="font-mono">{setup.data.secret}</span>
            </p>
            <label className="label" htmlFor="verify-code">
              Code
            </label>
            <input
              id="verify-code"
              className="input w-full"
              value={code}
              onChange={e => setCode(e.target.value)}
              required
            />
            {verify.error && (
              <div role="alert" className="alert alert-error mt-2">
                {verify.error.message}
              </div>
            )}
            <div className="card-actions justify-end mt-4">
              <button className="btn btn-primary" disabled={verify.isPending}>
                Confirm
              </button>
            </div>
          </form>
        ) : (
          <>
            <p>Protect your account with an authenticator app.</p>
            {setup.error && (
              <div role="alert" className="alert alert-error">
                {setup.error.message}
              </div>
            )}
            <div className="card-actions justify-end">
              <button
                className="btn btn-primary"
                onClick={() => setup.mutate()}
                disabled={setup.isPending}
              >
                Enable 2FA
              </button>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
```

`frontend/src/components/profile/DisableTwoFactor.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {useTwoFactorDisable} from '../../hooks/auth';

export default function DisableTwoFactor() {
  const disable = useTwoFactorDisable();
  const [code, setCode] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    disable.mutate({code});
  };

  return (
    <section className="card bg-base-100 shadow">
      <form className="card-body" onSubmit={submit}>
        <h2 className="card-title">Two-factor authentication</h2>
        <p>2FA is enabled on your account.</p>
        <label className="label" htmlFor="disable-code">
          Enter a current code (or backup code) to disable
        </label>
        <input
          id="disable-code"
          className="input w-full"
          value={code}
          onChange={e => setCode(e.target.value)}
          required
        />
        {disable.error && (
          <div role="alert" className="alert alert-error">
            {disable.error.message}
          </div>
        )}
        <div className="card-actions justify-end">
          <button className="btn btn-warning" disabled={disable.isPending}>
            Disable 2FA
          </button>
        </div>
      </form>
    </section>
  );
}
```

- [ ] **Step 4: Write the password reset pages and routes**

`frontend/src/pages/ForgotPasswordPage.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link} from 'react-router';
import {useRequestPasswordReset} from '../hooks/auth';

export default function ForgotPasswordPage() {
  const request = useRequestPasswordReset();
  const [email, setEmail] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    request.mutate({email});
  };

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">Reset password</h1>
        {request.isSuccess ? (
          <p>
            If an account exists for that address, a reset link is on its way.
            Check your inbox.
          </p>
        ) : (
          <form className="card bg-base-100 w-full p-6 shadow" onSubmit={submit}>
            <fieldset className="fieldset">
              <label className="label" htmlFor="email">
                Email
              </label>
              <input
                id="email"
                type="email"
                className="input w-full"
                value={email}
                onChange={e => setEmail(e.target.value)}
                required
              />
              {request.error && (
                <div role="alert" className="alert alert-error mt-2">
                  {request.error.message}
                </div>
              )}
              <button className="btn btn-primary mt-4" disabled={request.isPending}>
                Send reset link
              </button>
            </fieldset>
          </form>
        )}
        <p className="text-sm">
          <Link className="link" to="/login">
            Back to login
          </Link>
        </p>
      </div>
    </main>
  );
}
```

`frontend/src/pages/ResetPasswordPage.tsx`:

```tsx
import {useState} from 'react';
import type {FormEvent} from 'react';
import {Link, useSearchParams} from 'react-router';
import {useConfirmPasswordReset} from '../hooks/auth';

export default function ResetPasswordPage() {
  const [params] = useSearchParams();
  const token = params.get('token') ?? '';
  const confirm = useConfirmPasswordReset();
  const [newPassword, setNewPassword] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    confirm.mutate({token, newPassword});
  };

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content w-full max-w-sm flex-col">
        <h1 className="text-4xl font-bold">Choose a new password</h1>
        {confirm.isSuccess ? (
          <p>
            Password updated.{' '}
            <Link className="link" to="/login">
              Log in
            </Link>
          </p>
        ) : (
          <form className="card bg-base-100 w-full p-6 shadow" onSubmit={submit}>
            <fieldset className="fieldset">
              <label className="label" htmlFor="new-password">
                New password
              </label>
              <input
                id="new-password"
                type="password"
                className="input w-full"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                minLength={8}
                required
              />
              {confirm.error && (
                <div role="alert" className="alert alert-error mt-2">
                  {confirm.error.message}
                </div>
              )}
              <button className="btn btn-primary mt-4" disabled={confirm.isPending}>
                Set password
              </button>
            </fieldset>
          </form>
        )}
      </div>
    </main>
  );
}
```

Update `frontend/src/App.tsx` to its final form:

```tsx
import {Route, Routes} from 'react-router';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import HomePage from './pages/HomePage';
import LoginPage from './pages/LoginPage';
import ProfilePage from './pages/ProfilePage';
import ResetPasswordPage from './pages/ResetPasswordPage';
import SignupPage from './pages/SignupPage';

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      <Route path="/reset-password" element={<ResetPasswordPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<Layout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/profile" element={<ProfilePage />} />
        </Route>
      </Route>
    </Routes>
  );
}
```

- [ ] **Step 5: Run all frontend checks**

Run (from `frontend/`): `bun run test && bun run typecheck && bun run lint && bun run format:check && bun run build`
Expected: all pass.

- [ ] **Step 6: Manual end-to-end sanity (dev loop)**

Run `just dev`, open http://localhost:3000: you should land on /login. Sign up, see the welcome page, visit Profile, change username, enroll 2FA with a real authenticator app (or skip the scan and use the manual secret with an online TOTP tool), log out, log back in with the code. Ctrl-C the loop and `rm -rf data/` afterward. Report what you verified.

- [ ] **Step 7: Commit**

```bash
git add frontend
git commit -m "feat: profile page with 2fa enrollment and password reset pages"
```

---

### Task 14: Docs and final verification

**Files:**
- Modify: `AGENTS.md`, `README.md`

- [ ] **Step 1: Update `AGENTS.md`**

Make these targeted edits (verify each claim against the repo as you go):

1. In the Architecture section, add after the `internal/static/` bullet:

```markdown
- `internal/model/` — persisted domain types (User, Session, BackupCode,
  PasswordReset)
- `internal/repository/` — GORM/SQLite persistence (`Repo` struct; CGO-free
  driver `ncruces/go-sqlite3` via gormlite, WAL mode, AutoMigrate at startup)
- `internal/auth/` — argon2id hashing, random tokens, TOTP, backup codes
- `internal/middleware/` — `RequireAuth` session middleware (import-aliased
  `appmw` where Echo's middleware package is also imported)
- `internal/mail/` — provider-agnostic SMTP mailer; disabled (and password
  reset hidden, endpoints 404) when `BANDWIDTH_SMTP_HOST`/`FROM` are unset
```

2. Add a new section after Architecture:

```markdown
## Configuration

All config is `BANDWIDTH_*` env vars via Viper (defaults in
`cmd/bandwidth/main.go`): `PORT` (:8080), `LOG_LEVEL` (info), `DB_PATH`
(data/bandwidth.db), `SECURE_COOKIES` (false; set true behind TLS),
`BASE_URL` (http://localhost:3000; used in password-reset links),
`SMTP_HOST`, `SMTP_PORT` (587), `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`.

Sessions are opaque 256-bit tokens stored hashed (SHA-256) in the DB and
carried in an HttpOnly Lax cookie — there is no cookie-signing secret.
CSRF uses Echo v5's fetch-metadata-aware middleware (`Sec-Fetch-Site`);
tests must set `Sec-Fetch-Site: same-origin` on mutating requests.
Login/signup/reset endpoints are rate limited (1 req/s, burst 5, per IP).
```

3. In the Testing section, add:

```markdown
- Repository tests run against in-memory SQLite (`repository.Open(":memory:")`,
  fresh DB per test). Handler tests register routes directly on a bare
  `echo.New()` (no CSRF) via helpers in `internal/handlers/auth_test.go`.
```

- [ ] **Step 2: Update `README.md`**

Replace the Stack section body with:

```markdown
Go + Echo + SQLite (GORM, CGO-free) backend; React 19 + TypeScript + Vite +
Tailwind CSS/DaisyUI frontend with TanStack Query. Accounts with TOTP 2FA
and optional SMTP password reset. Planned per the design doc: song and band
tracking, installable PWA, single container on fly.io.
```

- [ ] **Step 3: Final verification**

Run: `just check`
Expected: `all checks passed`.

Run: `git status --porcelain`
Expected: only AGENTS.md and README.md modified.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md README.md
git commit -m "docs: document auth architecture and configuration"
```

---

## Done criteria

- `just check` green (all six gates).
- Signup → login (with and without 2FA) → profile editing → password change → logout all work end-to-end through the dev loop.
- Password reset endpoints return 404 (and the UI hides the link) when SMTP is unconfigured; the full email flow is covered by handler tests with a fake mailer.
- 2FA: enroll with QR + manual secret, verify, backup codes shown exactly once, login accepts TOTP or backup codes (single-use), disable works.
- Cross-site mutating requests are rejected (CSRF); login endpoints are rate limited.
- Next: Plan 3 (Personal Songs) gets written against this codebase.
