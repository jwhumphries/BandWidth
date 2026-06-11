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
	t.Cleanup(func() {
		if err := repo.Close(); err != nil {
			t.Errorf("closing test repo: %v", err)
		}
	})
	api := &handlers.API{Repo: repo, Mailer: mail.Disabled{}, BaseURL: "http://test"}
	e, err := newEcho(slog.New(slog.NewTextHandler(io.Discard, nil)), api)
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

	// Issue a plain GET (no Sec-Fetch-Site) so the CSRF middleware falls
	// through to the legacy token path, which is where it sets the cookie.
	// Browsers that omit Sec-Fetch-Site (direct navigation, curl, non-Fetch
	// requests) receive the _csrf cookie so they can supply the token on
	// subsequent state-changing requests.
	featReq := httptest.NewRequest(http.MethodGet, "/api/auth/features", nil)
	featRec := httptest.NewRecorder()
	e.ServeHTTP(featRec, featReq)

	// The CSRF middleware must issue its cookie to new clients.
	hasCSRF := false
	for _, c := range featRec.Result().Cookies() {
		if c.Name == "_csrf" || c.Name == "csrf" {
			hasCSRF = true
		}
	}
	if !hasCSRF {
		t.Error("no CSRF cookie issued on first response")
	}

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

func TestSongLibraryFlow(t *testing.T) {
	e := testServer(t)

	rec := do(e, http.MethodPost, "/api/auth/signup",
		`{"username":"alice","email":"alice@example.com","password":"hunter2hunter2"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d", rec.Code)
	}
	cookies := rec.Result().Cookies()

	rec = do(e, http.MethodPost, "/api/songs", `{"title":"Wonderwall","artist":"Oasis"}`, cookies)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create song: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, "/api/songs", "", cookies)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Wonderwall") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(e, http.MethodGet, "/api/folders", "", cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("folders: %d", rec.Code)
	}
}
