package ui

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func newTestHandler() http.Handler {
	static := fstest.MapFS{"htmx.min.js": {Data: []byte("//htmx")}}
	return NewHandler(static, func() (string, error) { return "/tmp/vex", nil })
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
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

func TestStartWithoutDirectoryShowsError(t *testing.T) {
	h := NewHandler(fstest.MapFS{}, func() (string, error) { return "", nil })
	rec := postForm(t, h, "/start", nil)

	if !strings.Contains(rec.Body.String(), "Error: Choose a directory first.") {
		t.Errorf("POST /start body = %q, want no-directory error", rec.Body.String())
	}
}

func TestStartAndStopWatching(t *testing.T) {
	dir := t.TempDir()
	h := NewHandler(fstest.MapFS{}, func() (string, error) { return dir, nil })
	postForm(t, h, "/pick-directory", nil)

	rec := postForm(t, h, "/start", nil)
	if !strings.Contains(rec.Body.String(), "Stop Watching") {
		t.Errorf("POST /start body does not show Stop Watching: %q", rec.Body.String())
	}

	rec = postForm(t, h, "/stop", nil)
	if !strings.Contains(rec.Body.String(), "Start Watching") {
		t.Errorf("POST /stop body does not show Start Watching: %q", rec.Body.String())
	}
}

func TestLogIsCapped(t *testing.T) {
	s := NewScreen()
	s.mu.Lock()
	for range maxLogEntries + 50 {
		s.logLocked("entry")
	}
	s.mu.Unlock()

	if n := len(s.Snapshot().Log); n != maxLogEntries {
		t.Errorf("log length = %d, want %d", n, maxLogEntries)
	}
}

func TestEventsStreamSendsPanelAndLog(t *testing.T) {
	srv := httptest.NewServer(newTestHandler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	events := map[string]bool{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if event, ok := strings.CutPrefix(scanner.Text(), "event: "); ok {
			events[event] = true
		}
		if events["panel"] && events["log"] {
			return
		}
	}
	t.Fatalf("stream ended before panel and log events; got %v, err %v", events, scanner.Err())
}
