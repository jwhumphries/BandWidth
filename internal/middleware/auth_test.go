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

func newAdminServer(t *testing.T) (*echo.Echo, *repository.Repo) {
	t.Helper()
	repo, err := repository.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	isAdmin := func(email string) bool { return email == "admin@example.com" }
	e := echo.New()
	e.GET("/admin-only", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, RequireAuth(repo), RequireAdmin(isAdmin))
	return e, repo
}

func TestRequireAdmin(t *testing.T) {
	e, repo := newAdminServer(t)
	admin, _ := repo.CreateUser("admin", "admin@example.com", "h")
	adminToken, _ := repo.CreateSession(admin.ID)
	member, _ := repo.CreateUser("bob", "bob@example.com", "h")
	memberToken, _ := repo.CreateSession(member.ID)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "non-admin", token: memberToken, wantStatus: http.StatusForbidden},
		{name: "admin", token: adminToken, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tt.token})
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
