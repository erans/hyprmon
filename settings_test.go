package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsLoadSaveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	orig := customConfigPath
	customConfigPath = tmp
	t.Cleanup(func() { customConfigPath = orig })

	// Missing file returns empty settings, no error.
	s, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings on missing file: %v", err)
	}
	if s == nil {
		t.Fatal("loadSettings returned nil Settings; want empty struct")
	}

	// Set a preference and persist.
	setMonitorPref(s, "Dell Inc./DELL U3419W/5HJB6T2", MonitorPref{UseDescFormat: true})
	if err := saveSettings(s); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}

	// File exists with correct permissions.
	path := filepath.Join(tmp, "settings.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat settings.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("settings.json mode = %v, want 0600", perm)
	}

	// Reload and confirm value preserved.
	s2, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings after save: %v", err)
	}
	pref := getMonitorPref(s2, "Dell Inc./DELL U3419W/5HJB6T2")
	if !pref.UseDescFormat {
		t.Errorf("preference lost across reload: %+v", pref)
	}

	// Lookup for an unknown HardwareID returns zero value.
	zero := getMonitorPref(s2, "Unknown/Model/XYZ")
	if zero.UseDescFormat {
		t.Errorf("unknown key should return zero value, got %+v", zero)
	}
}

func TestSettingsSaveAtomic(t *testing.T) {
	tmp := t.TempDir()
	orig := customConfigPath
	customConfigPath = tmp
	t.Cleanup(func() { customConfigPath = orig })

	s := &Settings{}
	setMonitorPref(s, "Foo/Bar/123", MonitorPref{UseDescFormat: true})
	if err := saveSettings(s); err != nil {
		t.Fatalf("saveSettings: %v", err)
	}

	// No stray temp file should remain.
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "settings.json" {
			t.Errorf("unexpected file in config dir: %s", e.Name())
		}
	}
}
