package iqsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTemp(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTemp(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteFileAtomicReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "a.py", "old")
	if err := WriteFileAtomic(filepath.Join(dir, "a.py"), []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got := readTemp(t, dir, "a.py"); got != "new" {
		t.Fatalf("got %q, want %q", got, "new")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only a.py, found %d entries", len(entries))
	}
}

func TestPyToIQCreatesFromTemplate(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "print(1)\n")
	msg, err := PyToIQ(dir, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if msg != "Created Foo.iqpython from template" {
		t.Fatalf("message = %q", msg)
	}
	back, err := IQToPython([]byte(readTemp(t, dir, "Foo.iqpython")))
	if err != nil || string(back) != "print(1)\n" {
		t.Fatalf("round trip = %q, %v", back, err)
	}
}

func TestPyToIQUpdatesExistingAndSkipsWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "a\n")
	if _, err := PyToIQ(dir, "Foo"); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "Foo.py", "b\n")
	msg, err := PyToIQ(dir, "Foo")
	if err != nil || msg != "Updated Foo.iqpython from Foo.py" {
		t.Fatalf("message = %q, %v", msg, err)
	}
	msg, err = PyToIQ(dir, "Foo")
	if err != nil || msg != "" {
		t.Fatalf("unchanged sync logged %q, %v", msg, err)
	}
}

func TestPyToIQRefusesToOverwriteInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "a\n")
	writeTemp(t, dir, "Foo.iqpython", "{ not json")
	if _, err := PyToIQ(dir, "Foo"); err == nil {
		t.Fatal("expected an error")
	}
	if got := readTemp(t, dir, "Foo.iqpython"); got != "{ not json" {
		t.Fatalf("invalid file was overwritten: %q", got)
	}
}

func TestIQToPyCreatesAndUpdates(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "")
	if _, err := PyToIQ(dir, "Foo"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "Foo.py")); err != nil {
		t.Fatal(err)
	}
	msg, err := IQToPy(dir, "Foo")
	if err != nil || msg != "Created Foo.py from Foo.iqpython" {
		t.Fatalf("message = %q, %v", msg, err)
	}
	if got := readTemp(t, dir, "Foo.py"); got != "" {
		t.Fatalf("Foo.py = %q, want empty", got)
	}
}

func TestIQToPyLeavesPyUntouchedOnBadJSON(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "keep\n")
	writeTemp(t, dir, "Foo.iqpython", `{"textContent": 5}`)
	if _, err := IQToPy(dir, "Foo"); err == nil {
		t.Fatal("expected an error")
	}
	if got := readTemp(t, dir, "Foo.py"); got != "keep\n" {
		t.Fatalf("Foo.py changed to %q", got)
	}
}

func TestWatcherSyncsNewPythonFile(t *testing.T) {
	dir := t.TempDir()
	logs := make(chan string, 16)
	failures := make(chan string, 1)
	w, err := Start(dir, true, func(m string) { logs <- m }, func(m string) { failures <- m })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	writeTemp(t, dir, "Foo.py", "print(1)\n")
	want := "Created Foo.iqpython from template"
	deadline := time.After(10 * time.Second)
	for {
		select {
		case m := <-logs:
			if m == want {
				if !strings.Contains(readTemp(t, dir, "Foo.iqpython"), `"textContent"`) {
					t.Fatal("Foo.iqpython has no textContent")
				}
				return
			}
		case m := <-failures:
			t.Fatalf("watcher failed: %s", m)
		case <-deadline:
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}
