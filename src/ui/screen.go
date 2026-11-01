package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"

	"vexiqtools/iqsync"
)

const maxLogEntries = 500

type LogEntry struct {
	Time    time.Time
	Message string
}

type Snapshot struct {
	Dir      string
	Watching bool
	Err      string
	Log      []LogEntry
}

func (s Snapshot) StatusText() string {
	switch {
	case s.Err != "":
		return "Error: " + s.Err
	case s.Watching:
		return "Watching " + s.Dir
	default:
		return "Idle"
	}
}

// Screen holds the UI state and pushes changes to every open SSE stream.
type Screen struct {
	mu      sync.Mutex
	snap    Snapshot
	subs    map[chan string]struct{}
	watcher *iqsync.Watcher
}

func NewScreen() *Screen {
	return &Screen{
		subs: make(map[chan string]struct{}),
	}
}

func (s *Screen) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snap
	snap.Log = slices.Clone(s.snap.Log)
	return snap
}

func (s *Screen) SetDirectory(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap.Watching {
		return
	}
	s.snap.Dir = dir
	s.snap.Err = ""
	s.publishLocked()
}

func (s *Screen) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap.Watching {
		return
	}
	if msg := checkWatchable(s.snap.Dir); msg != "" {
		s.snap.Err = msg
		s.publishLocked()
		return
	}
	w, err := iqsync.Start(s.snap.Dir, s.appendLog, s.fail)
	if err != nil {
		s.snap.Err = err.Error()
		s.publishLocked()
		return
	}
	s.watcher = w
	s.snap.Watching = true
	s.snap.Err = ""
	s.logLocked("Started watching " + s.snap.Dir)
	s.publishLocked()
}

func (s *Screen) Stop() {
	s.mu.Lock()
	if !s.snap.Watching {
		s.mu.Unlock()
		return
	}
	w := s.watcher
	s.watcher = nil
	s.snap.Watching = false
	s.snap.Err = ""
	s.logLocked("Stopped watching")
	s.publishLocked()
	s.mu.Unlock()

	// Stop outside the lock: the watcher's goroutine may be waiting for the lock to log.
	if w != nil {
		w.Stop()
	}
}

// appendLog is called by the watcher goroutine.
func (s *Screen) appendLog(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logLocked(msg)
	s.publishLocked()
}

// fail is called by the watcher goroutine on a watcher-level failure (FR-5.2).
func (s *Screen) fail(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watcher = nil
	s.snap.Watching = false
	s.snap.Err = msg
	s.publishLocked()
}

// checkWatchable returns a user-facing reason why dir cannot be watched, or "" if it can.
func checkWatchable(dir string) string {
	if dir == "" {
		return "Choose a directory first."
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "Folder does not exist."
	}
	// Create and remove a probe file to confirm the folder is writable. The name does not
	// match .py or .iqpython, so a watcher ignores it.
	probe, err := os.CreateTemp(dir, ".vexiqtools-probe-*")
	if err != nil {
		return "Folder is not writable."
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	return ""
}

func (s *Screen) logLocked(msg string) {
	s.snap.Log = append(s.snap.Log, LogEntry{Time: time.Now(), Message: msg})
	if n := len(s.snap.Log); n > maxLogEntries {
		s.snap.Log = s.snap.Log[n-maxLogEntries:]
	}
}

func (s *Screen) publishLocked() {
	for _, msg := range s.sseMessagesLocked() {
		for ch := range s.subs {
			select {
			case ch <- msg:
			default:
				// A stalled client drops updates; the next change re-sends full state.
			}
		}
	}
}

func (s *Screen) sseMessagesLocked() []string {
	return []string{
		sseMessage("panel", render(PanelBody(s.snap))),
		sseMessage("log", render(LogItems(s.snap.Log))),
	}
}

func (s *Screen) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch := make(chan string, 64)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	initial := s.sseMessagesLocked()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	for _, msg := range initial {
		_, _ = io.WriteString(w, msg)
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = io.WriteString(w, msg)
			flusher.Flush()
		}
	}
}

var lineBreaks = strings.NewReplacer("\r", "", "\n", "")

// render returns a component as a single line of HTML, so it can be sent as one SSE data field.
func render(c templ.Component) string {
	var b strings.Builder
	if err := c.Render(context.Background(), &b); err != nil {
		log.Printf("render: %v", err)
		return ""
	}
	return lineBreaks.Replace(b.String())
}

func sseMessage(event, data string) string {
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
}
