package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreResumesWatchingFromSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := saveSettings(path, settings{Dir: dir, Watching: true}); err != nil {
		t.Fatal(err)
	}

	s := NewScreen(path)
	s.Restore()
	defer s.Stop()
	if snap := s.Snapshot(); !snap.Watching || snap.Dir != dir {
		t.Fatalf("snapshot after restore = %+v", snap)
	}
}

func TestRestoreStaysStoppedWhenFolderIsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := saveSettings(path, settings{Dir: missing, Watching: true}); err != nil {
		t.Fatal(err)
	}

	s := NewScreen(path)
	s.Restore()
	snap := s.Snapshot()
	if snap.Watching || snap.Dir != missing || snap.Err != "Folder does not exist." {
		t.Fatalf("snapshot after restore = %+v", snap)
	}
}

func TestStopSavesStoppedState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "settings.json")
	s := NewScreen(path)
	s.SetDirectory(dir)
	s.Start()
	s.Stop()

	saved, err := loadSettings(path)
	if err != nil || saved.Dir != dir || saved.Watching {
		t.Fatalf("saved settings = %+v, %v", saved, err)
	}
}

func TestNotInGitWarning(t *testing.T) {
	root := t.TempDir()
	s := NewScreen("")
	s.SetDirectory(root)
	if !s.Snapshot().NotInGit {
		t.Fatal("expected a warning outside a Git repository")
	}

	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	s.SetDirectory(sub)
	if s.Snapshot().NotInGit {
		t.Fatal("warning shown inside a Git repository")
	}
}
