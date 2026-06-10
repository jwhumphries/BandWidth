package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v5"
)

func TestSPA(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app shell</html>")},
		"assets/app.js": {Data: []byte("console.log('hi');")},
	}
	e := echo.New()
	RegisterSPA(e, fsys)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "root serves index", path: "/", want: "app shell"},
		{name: "existing asset served", path: "/assets/app.js", want: "console.log"},
		{name: "client route falls back to index", path: "/songs/42", want: "app shell"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.want) {
				t.Fatalf("body = %q, want it to contain %q", body, tt.want)
			}
		})
	}
}
