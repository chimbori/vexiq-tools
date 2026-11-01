package iqsync

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounce is how long a file must be quiet before it is synced (FR-2.4).
const debounce = 300 * time.Millisecond

// Watcher syncs changed .py and .iqpython files in one directory until Stop is called.
type Watcher struct {
	dir       string
	hashes    map[string][sha256.Size]byte
	timers    map[string]*time.Timer
	lastEvent map[string]time.Time
	logf      func(string)
	failf     func(string)
	fsw       *fsnotify.Watcher
	fire      chan string
	done      chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

// Start watches the top level of dir (FR-2.1). Both .py and .iqpython files are watched (FR-2.2).
// Existing files are hashed but not synced (FR-2.8). Log messages go to logf. A watcher-level
// failure calls failf and stops the watcher (FR-5.2).
func Start(dir string, logf, failf func(string)) (*Watcher, error) {
	dir = filepath.Clean(dir)
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(dir); err != nil {
		_ = fsw.Close()
		return nil, err
	}
	w := &Watcher{
		dir:       dir,
		hashes:    seedHashes(dir),
		timers:    map[string]*time.Timer{},
		lastEvent: map[string]time.Time{},
		logf:      logf,
		failf:     failf,
		fsw:       fsw,
		fire:      make(chan string),
		done:      make(chan struct{}),
	}
	w.wg.Add(1)
	go w.run()
	return w, nil
}

// Stop ends watching and waits for the event loop to exit. It is safe to call more than once.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() { close(w.done) })
	w.wg.Wait()
}

func seedHashes(dir string) map[string][sha256.Size]byte {
	hashes := map[string][sha256.Size]byte{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !isPairFile(e.Name()) {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			hashes[e.Name()] = sha256.Sum256(data)
		}
	}
	return hashes
}

// isPairFile reports whether name is a .py or .iqpython file with a non-empty stem.
func isPairFile(name string) bool {
	for _, ext := range []string{".py", ".iqpython"} {
		if strings.HasSuffix(name, ext) && len(name) > len(ext) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// pairOther returns the name of the file that pairs with name, such as Foo.iqpython for Foo.py.
func pairOther(name string) string {
	if strings.HasSuffix(name, ".iqpython") {
		return strings.TrimSuffix(name, ".iqpython") + ".py"
	}
	return strings.TrimSuffix(name, ".py") + ".iqpython"
}

func (w *Watcher) run() {
	defer w.wg.Done()
	defer func() { _ = w.fsw.Close() }()
	defer func() {
		for _, t := range w.timers {
			t.Stop()
		}
	}()
	for {
		select {
		case <-w.done:
			return
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.failf(err.Error())
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if filepath.Clean(ev.Name) == w.dir {
				if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
					w.failf("Folder is no longer available.")
					return
				}
				continue
			}
			name := filepath.Base(ev.Name)
			if !isPairFile(name) {
				continue
			}
			// An atomic save replaces a file by renaming over it, which can also report Remove.
			// It is a deletion only if the file is gone.
			if ev.Has(fsnotify.Remove) && !fileExists(ev.Name) {
				delete(w.hashes, name)
				w.logf("Ignored deletion of " + name)
				continue
			}
			if !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Rename) &&
				!ev.Has(fsnotify.Remove) {
				continue
			}
			w.lastEvent[name] = time.Now()
			if t, ok := w.timers[name]; ok {
				t.Reset(debounce)
				continue
			}
			w.timers[name] = time.AfterFunc(debounce, func() {
				select {
				case w.fire <- name:
				case <-w.done:
				}
			})
		case name := <-w.fire:
			delete(w.timers, name)
			if w.superseded(name) {
				continue
			}
			w.syncFile(name)
		}
	}
}

// superseded reports whether the paired file changed later and is still waiting to sync. That
// later change wins (FR-2.9), so name must not overwrite it. A later event with unchanged content
// (such as a touch) does not count, so name still syncs.
func (w *Watcher) superseded(name string) bool {
	other := pairOther(name)
	if _, pending := w.timers[other]; !pending {
		return false
	}
	if !w.lastEvent[other].After(w.lastEvent[name]) {
		return false
	}
	data, err := os.ReadFile(filepath.Join(w.dir, other))
	if err != nil {
		return false
	}
	return sha256.Sum256(data) != w.hashes[other]
}

// syncFile syncs the pair that name belongs to, unless the file content is the same as the last
// content seen. Writing a target records its hash, so the watcher event caused by that write is
// skipped and the two directions never trigger each other (FR-2.6, FR-2.7).
func (w *Watcher) syncFile(name string) {
	data, err := os.ReadFile(filepath.Join(w.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		// A rename away from the folder reports the old name, which no longer exists.
		return
	}
	if err != nil {
		w.logf("Error: " + err.Error())
		return
	}
	sum := sha256.Sum256(data)
	if prev, ok := w.hashes[name]; ok && prev == sum {
		return
	}
	w.hashes[name] = sum

	res, err := SyncFile(w.dir, name)
	if err != nil {
		w.logf("Error: " + err.Error())
		return
	}
	if res.Target != "" {
		w.hashes[res.Target] = sha256.Sum256(res.Data)
	}
	if res.Message != "" {
		w.logf(res.Message)
	}
}
