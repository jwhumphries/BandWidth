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
	// Email-shaped username → 400 (would collide with UserByLogin's email match).
	rec = jsonReq(e, http.MethodPatch, "/api/me", `{"username":"bob@example.com"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("email-shaped username: %d, want 400", rec.Code)
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
