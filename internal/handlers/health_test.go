package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestHealthz(t *testing.T) {
	e := echo.New()
	e.GET("/healthz", Healthz)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %q, want it to contain %q", body, `"status":"ok"`)
	}
}
