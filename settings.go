package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MonitorPref holds per-monitor hyprmon preferences, keyed in Settings by
// the monitor's HardwareID.
type MonitorPref struct {
	UseDescFormat bool `json:"use_desc_format,omitempty"`
}

// Settings is the on-disk hyprmon settings file.
type Settings struct {
	MonitorPrefs map[string]MonitorPref `json:"monitor_prefs,omitempty"`
}

// getSettingsDir returns the directory that holds settings.json. It mirrors
// getProfilesDir but at the config root instead of the profiles subdir.
func getSettingsDir() string {
	if customConfigPath != "" {
		return customConfigPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "hyprmon")
}

func getSettingsPath() string {
	dir := getSettingsDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "settings.json")
}

// loadSettings reads settings.json. A missing file is NOT an error; an
// empty Settings is returned. Corrupted files return an error.
func loadSettings() (*Settings, error) {
	path := getSettingsPath()
	if path == "" {
		return &Settings{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{}, nil
		}
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}
	return &s, nil
}

// saveSettings writes settings.json atomically (write-then-rename).
func saveSettings(s *Settings) error {
	dir := getSettingsDir()
	if dir == "" {
		return fmt.Errorf("could not determine settings directory")
	}
	if err := os.MkdirAll(dir, profileDirMode); err != nil {
		return fmt.Errorf("failed to ensure settings directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	path := filepath.Join(dir, "settings.json")
	tmp, err := os.CreateTemp(dir, "settings.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	if err := tmp.Chmod(configFileMode); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to chmod temp settings file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write settings: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to sync settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close settings: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to rename settings into place: %w", err)
	}
	return nil
}

// getMonitorPref returns the preference for a HardwareID. Returns zero
// value when the key is missing or settings is nil.
func getMonitorPref(s *Settings, hwid string) MonitorPref {
	if s == nil || hwid == "" {
		return MonitorPref{}
	}
	return s.MonitorPrefs[hwid]
}

// setMonitorPref writes a preference into the in-memory Settings. Caller
// is responsible for persisting via saveSettings.
func setMonitorPref(s *Settings, hwid string, pref MonitorPref) {
	if s == nil || hwid == "" {
		return
	}
	if s.MonitorPrefs == nil {
		s.MonitorPrefs = make(map[string]MonitorPref)
	}
	s.MonitorPrefs[hwid] = pref
}
