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
		{name: "bare at-sign email", body: `{"username":"a","email":"@","password":"hunter2hunter2"}`, want: 400},
		{name: "missing domain email", body: `{"username":"a","email":"user@","password":"hunter2hunter2"}`, want: 400},
		{name: "email-shaped username", body: `{"username":"who@else.com","email":"a@b.c","password":"hunter2hunter2"}`, want: 400},
		{name: "oversized username", body: `{"username":"` + strings.Repeat("u", 101) + `","email":"a@b.c","password":"hunter2hunter2"}`, want: 400},
		{name: "oversized email", body: `{"username":"a","email":"` + strings.Repeat("e", 251) + `@b.c","password":"hunter2hunter2"}`, want: 400},
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

	// Whitespace around the code is tolerated.
	code2, _ := totp.GenerateCode(key.Secret, time.Now())
	rec = postJSON(e, "/api/auth/login",
		fmt.Sprintf(`{"login":"alice","password":"hunter2hunter2","totpCode":" %s "}`, code2))
	if rec.Code != http.StatusOK {
		t.Fatalf("padded code: %d %s", rec.Code, rec.Body.String())
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
