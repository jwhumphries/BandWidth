package handlers

import (
	"net/http"
	netmail "net/mail"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"

	"github.com/jwhumphries/bandwidth/internal/auth"
	"github.com/jwhumphries/bandwidth/internal/repository"
)

// Length caps for identity fields; the email cap is RFC 5321's address limit.
const (
	maxUsernameLen = 100
	maxEmailLen    = 254
)

// validEmail reports whether s is a plain RFC 5322 address (no display name).
func validEmail(s string) bool {
	addr, err := netmail.ParseAddress(s)
	return err == nil && addr.Address == s && len(s) <= maxEmailLen
}

// validUsername reports whether s works as a username. Usernames share the
// login field with emails (UserByLogin matches either column), so an
// embedded @ would make lookups ambiguous and is rejected.
func validUsername(s string) bool {
	return s != "" && len(s) <= maxUsernameLen && !strings.Contains(s, "@")
}

// dummyPasswordHash is compared when login names no account, so response
// timing does not reveal whether a username or email exists. Computed
// lazily: the argon2id hash is memory-hard and would otherwise slow every
// process start.
var dummyPasswordHash = sync.OnceValue(func() string {
	hash, err := auth.HashPassword(auth.NewToken())
	if err != nil {
		panic(err)
	}
	return hash
})

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
	if !validUsername(req.Username) {
		return echo.NewHTTPError(http.StatusBadRequest,
			"a username of at most 100 characters (without @) is required")
	}
	if len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest,
			"a password of at least 8 characters is required")
	}
	if !validEmail(req.Email) {
		return echo.NewHTTPError(http.StatusBadRequest, "a valid email address is required")
	}
	if !a.IsAdminEmail(req.Email) {
		enabled, err := a.Repo.AccessPolicyEnabled()
		if err != nil {
			return err
		}
		if enabled {
			allowed, err := a.Repo.EmailAllowed(req.Email)
			if err != nil {
				return err
			}
			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "registration is not open")
			}
		}
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
	return c.JSON(http.StatusCreated, a.userResponse(user))
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
	if err != nil {
		// Burn a comparable hash verification so unknown accounts are not
		// distinguishable from wrong passwords by timing.
		auth.VerifyPassword(req.Password, dummyPasswordHash())
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
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
		if !auth.ValidateTOTP(code, user.TOTPSecret) &&
			!a.Repo.ConsumeBackupCode(user.ID, code) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid two-factor code")
		}
	}

	token, err := a.Repo.CreateSession(user.ID)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token)
	return c.JSON(http.StatusOK, a.userResponse(user))
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
