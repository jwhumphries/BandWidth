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
