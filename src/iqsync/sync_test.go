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
	res, err := PyToIQ(dir, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Created Foo.iqpython from template" || res.Target != "Foo.iqpython" {
		t.Fatalf("result = %+v", res)
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
	res, err := PyToIQ(dir, "Foo")
	if err != nil || res.Message != "Updated Foo.iqpython from Foo.py" {
		t.Fatalf("result = %+v, %v", res, err)
	}
	res, err = PyToIQ(dir, "Foo")
	if err != nil || res.Message != "" || res.Data != nil {
		t.Fatalf("unchanged sync returned %+v, %v", res, err)
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
	res, err := IQToPy(dir, "Foo")
	if err != nil || res.Message != "Created Foo.py from Foo.iqpython" {
		t.Fatalf("result = %+v, %v", res, err)
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

// startWatcher starts a watcher on dir. Log messages arrive on logs and failures on failures.
func startWatcher(t *testing.T, dir string) (chan string, chan string) {
	t.Helper()
	logs := make(chan string, 64)
	failures := make(chan string, 1)
	w, err := Start(dir, func(m string) { logs <- m }, func(m string) { failures <- m })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return logs, failures
}

// waitFor returns every log message up to and including want.
func waitFor(t *testing.T, logs, failures chan string, want string) []string {
	t.Helper()
	var seen []string
	deadline := time.After(10 * time.Second)
	for {
		select {
		case m := <-logs:
			seen = append(seen, m)
			if m == want {
				return seen
			}
		case m := <-failures:
			t.Fatalf("watcher failed: %s", m)
		case <-deadline:
			t.Fatalf("timed out waiting for %q, saw %q", want, seen)
		}
	}
}

func containsPrefix(msgs []string, prefix string) bool {
	for _, m := range msgs {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

func TestWatcherSyncsNewPythonFile(t *testing.T) {
	dir := t.TempDir()
	logs, failures := startWatcher(t, dir)

	writeTemp(t, dir, "Foo.py", "print(1)\n")
	waitFor(t, logs, failures, "Created Foo.iqpython from template")
	if !strings.Contains(readTemp(t, dir, "Foo.iqpython"), `"textContent"`) {
		t.Fatal("Foo.iqpython has no textContent")
	}
}

func TestWatcherSkipsUnchangedSave(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "a\n")
	if _, err := PyToIQ(dir, "Foo"); err != nil {
		t.Fatal(err)
	}
	logs, failures := startWatcher(t, dir)

	// Same content saved again. The marker file shows the watcher has handled everything before it.
	writeTemp(t, dir, "Foo.py", "a\n")
	writeTemp(t, dir, "Marker.py", "")
	seen := waitFor(t, logs, failures, "Created Marker.iqpython from template")
	if containsPrefix(seen, "Updated Foo") {
		t.Fatalf("unchanged save triggered a conversion: %q", seen)
	}
}

func TestWatcherDoesNotEchoBetweenDirections(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "a\n")
	if _, err := PyToIQ(dir, "Foo"); err != nil {
		t.Fatal(err)
	}
	logs, failures := startWatcher(t, dir)

	writeTemp(t, dir, "Foo.py", "b\n")
	waitFor(t, logs, failures, "Updated Foo.iqpython from Foo.py")

	// The .iqpython event caused by the write above must not write Foo.py back.
	writeTemp(t, dir, "Marker.py", "")
	seen := waitFor(t, logs, failures, "Created Marker.iqpython from template")
	if containsPrefix(seen, "Updated Foo.py") || containsPrefix(seen, "Created Foo.py") {
		t.Fatalf("update echoed back to Foo.py: %q", seen)
	}
}

func TestWatcherDoesNotSyncExistingFilesOnStart(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "a\n")
	if _, err := PyToIQ(dir, "Foo"); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "Foo.py", "changed while stopped\n")
	logs, failures := startWatcher(t, dir)

	writeTemp(t, dir, "Marker.py", "")
	seen := waitFor(t, logs, failures, "Created Marker.iqpython from template")
	if containsPrefix(seen, "Updated Foo") || containsPrefix(seen, "Created Foo") {
		t.Fatalf("start synced existing files: %q", seen)
	}
}

func TestWatcherLogsDeletionAndIgnoresIt(t *testing.T) {
	dir := t.TempDir()
	logs, failures := startWatcher(t, dir)
	writeTemp(t, dir, "Foo.py", "a\n")
	waitFor(t, logs, failures, "Created Foo.iqpython from template")

	if err := os.Remove(filepath.Join(dir, "Foo.py")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, logs, failures, "Ignored deletion of Foo.py")
	if _, err := os.Stat(filepath.Join(dir, "Foo.iqpython")); err != nil {
		t.Fatalf("Foo.iqpython was removed: %v", err)
	}
}

func TestWatcherLaterChangeWinsWhenBothFilesChange(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "Foo.py", "a\n")
	if _, err := PyToIQ(dir, "Foo"); err != nil {
		t.Fatal(err)
	}
	logs, failures := startWatcher(t, dir)

	// Both files change inside one debounce window. The .iqpython edit is the later change.
	writeTemp(t, dir, "Foo.py", "b\n")
	iq := readTemp(t, dir, "Foo.iqpython")
	if !strings.Contains(iq, `a\n`) {
		t.Fatalf("Foo.iqpython has no a\\n to replace: %q", iq)
	}
	writeTemp(t, dir, "Foo.iqpython", strings.Replace(iq, `a\n`, `c\n`, 1))

	waitFor(t, logs, failures, "Updated Foo.py from Foo.iqpython")
	if got := readTemp(t, dir, "Foo.py"); got != "c\n" {
		t.Fatalf("Foo.py = %q, want the later change %q", got, "c\n")
	}
}
