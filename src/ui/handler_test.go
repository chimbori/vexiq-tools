package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestHandler() http.Handler {
	static := fstest.MapFS{"htmx.min.js": {Data: []byte("//htmx")}}
	return NewHandler(static, func() (string, error) { return "/tmp/vex", nil })
}

func TestPageRendersChooseButton(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Choose Directory") {
		t.Errorf("GET / body missing directory button")
	}
}

func TestStaticAssetServed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/htmx.min.js status = %d, want 200", rec.Code)
	}
}

func TestPickDirectoryReturnsFragment(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pick-directory", nil))

	if !strings.Contains(rec.Body.String(), "/tmp/vex") {
		t.Errorf("POST /pick-directory body = %q, want selected path", rec.Body.String())
	}
}
