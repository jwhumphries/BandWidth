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
