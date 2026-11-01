package ui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"vexiqtools/iqsync"
)

// settings is the state restored on launch (FR-1.5).
type settings struct {
	Dir      string `json:"dir"`
	Watching bool   `json:"watching"`
}

// DefaultSettingsPath returns settings.json in the app folder under the user's config directory.
func DefaultSettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Tools for Vex IQ", "settings.json"), nil
}

func loadSettings(path string) (settings, error) {
	var s settings
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}

func saveSettings(path string, s settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return iqsync.WriteFileAtomic(path, data)
}
