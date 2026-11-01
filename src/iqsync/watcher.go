package iqsync

import (
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

// Watcher syncs changed files in one directory until Stop is called.
type Watcher struct {
	dir      string
	pyToIQ   bool
	logf     func(string)
	failf    func(string)
	fsw      *fsnotify.Watcher
	fire     chan string
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// Start watches the top level of dir (FR-2.1). In Python → IQ mode (pyToIQ) only .py files are
// synced, otherwise only .iqpython files (FR-2.2). Log messages go to logf. A watcher-level
// failure calls failf and stops the watcher (FR-5.2).
func Start(dir string, pyToIQ bool, logf, failf func(string)) (*Watcher, error) {
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
		dir:    dir,
		pyToIQ: pyToIQ,
		logf:   logf,
		failf:  failf,
		fsw:    fsw,
		fire:   make(chan string),
		done:   make(chan struct{}),
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

func (w *Watcher) ext() string {
	if w.pyToIQ {
		return ".py"
	}
	return ".iqpython"
}

func (w *Watcher) run() {
	defer w.wg.Done()
	defer func() { _ = w.fsw.Close() }()
	timers := map[string]*time.Timer{}
	defer func() {
		for _, t := range timers {
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
			if !strings.HasSuffix(name, w.ext()) || len(name) == len(w.ext()) {
				continue
			}
			if ev.Has(fsnotify.Remove) {
				w.logf("Ignored deletion of " + name)
				continue
			}
			if !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Rename) {
				continue
			}
			if t, ok := timers[name]; ok {
				t.Reset(debounce)
				continue
			}
			timers[name] = time.AfterFunc(debounce, func() {
				select {
				case w.fire <- name:
				case <-w.done:
				}
			})
		case name := <-w.fire:
			w.syncFile(name)
		}
	}
}

func (w *Watcher) syncFile(name string) {
	// A rename away from the folder reports the old name, which no longer exists. Skip it.
	if _, err := os.Stat(filepath.Join(w.dir, name)); errors.Is(err, fs.ErrNotExist) {
		return
	}
	stem := strings.TrimSuffix(name, w.ext())
	var msg string
	var err error
	if w.pyToIQ {
		msg, err = PyToIQ(w.dir, stem)
	} else {
		msg, err = IQToPy(w.dir, stem)
	}
	if err != nil {
		w.logf("Error: " + err.Error())
		return
	}
	if msg != "" {
		w.logf(msg)
	}
}
