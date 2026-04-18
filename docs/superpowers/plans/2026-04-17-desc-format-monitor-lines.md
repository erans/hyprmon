# Desc-Format Monitor Lines — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-monitor opt-in to write monitor lines as `monitor=desc:<description>,…` in `hyprland.conf`, persisted in a new hyprmon settings file and in profile JSON.

**Architecture:** Extend `Monitor` with a `UseDescFormat` flag. Introduce a small `settings.go` keyed by `HardwareID` for cross-session persistence. `generateMonitorLine` in `hyprland.go` gains an identifier selector that substitutes `desc:<description>` for the connector name when the toggle is on and the description is unambiguous and safe. Live apply via `hyprctl keyword monitor` is unchanged. UI adds one row to the advanced settings dialog; disabled with a reason when `canUseDescFormat` returns false.

**Tech Stack:** Go, Bubble Tea (existing), stdlib `encoding/json`, stdlib `testing`.

**Spec:** [`docs/superpowers/specs/2026-04-17-desc-format-monitor-lines-design.md`](../specs/2026-04-17-desc-format-monitor-lines-design.md)

---

## Preconditions

Before Task 1, confirm the current state matches the design:

- `Monitor` struct lives in `models.go` with `json:"name"` and `json:"hardware_id,omitempty"` tags on selected fields.
- `generateMonitorLine(m Monitor) string` in `hyprland.go` builds monitor lines using `m.Name` as the first token.
- `readMonitors()` in `hyprland.go` calls `disambiguateHardwareIDs(monitors)` before returning.
- `customConfigPath` is a package-level var in `profiles.go`, set from `--cfg` in `main.go`.
- Advanced settings dialog in `advanced_settings.go` holds a `*Monitor` and mutates it through that pointer.
- Tests live in `*_test.go` files alongside sources, package `main`.

If any of these have drifted, stop and reconcile before proceeding.

---

## Task 1: Add `UseDescFormat` field to `Monitor`

**Files:**
- Modify: `models.go` (Monitor struct, ~line 13)
- Test: `models_test.go` (add new test)

- [ ] **Step 1: Write the failing test**

Append to `models_test.go`:

```go
func TestMonitorUseDescFormatRoundTrip(t *testing.T) {
	orig := Monitor{
		Name:          "DP-9",
		HardwareID:    "Dell Inc./DELL U3419W/5HJB6T2",
		UseDescFormat: true,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Monitor
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.UseDescFormat {
		t.Errorf("UseDescFormat lost in round-trip: got %v, want true", got.UseDescFormat)
	}

	// omitempty: zero value should not appear in JSON output
	zero := Monitor{Name: "DP-1"}
	zeroData, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	if strings.Contains(string(zeroData), "use_desc_format") {
		t.Errorf("omitempty failed: zero value appeared in JSON: %s", zeroData)
	}
}
```

If `models_test.go` does not already import `encoding/json` and `strings`, add them to the existing import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestMonitorUseDescFormatRoundTrip ./...`
Expected: FAIL (undefined `UseDescFormat` or missing field).

- [ ] **Step 3: Add the field to `Monitor`**

In `models.go`, inside the `Monitor` struct, add this line after the `Serial` field (around line 19):

```go
	UseDescFormat bool   `json:"use_desc_format,omitempty"`
```

(Preserve existing struct field alignment — Go will reformat on `gofmt`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestMonitorUseDescFormatRoundTrip ./...`
Expected: PASS.

Also run the full suite to catch collateral damage:
Run: `go test ./...`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add models.go models_test.go
git commit -m "feat: add UseDescFormat field to Monitor (#75)"
```

---

## Task 2: Add `sanitizeDesc` helper

**Files:**
- Modify: `hyprland.go` (append helper near other monitor helpers, e.g., after `isValidColorMode`)
- Test: `hyprland_test.go` (create if missing)

- [ ] **Step 1: Write the failing test**

If `hyprland_test.go` does not exist, create it with this content:

```go
package main

import "testing"

func TestSanitizeDesc(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain description", "Dell Inc. DELL U3419W 5HJB6T2", "Dell Inc. DELL U3419W 5HJB6T2"},
		{"trims surrounding whitespace", "  Dell Inc. DELL U3419W  ", "Dell Inc. DELL U3419W"},
		{"rejects embedded comma", "Apple Computer Inc., Apple Studio Display", ""},
		{"rejects embedded double quote", `Dell "pro" U3419W`, ""},
		{"rejects newline", "Dell\nU3419W", ""},
		{"rejects control character", "Dell\x01U3419W", ""},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDesc(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeDesc(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

If `hyprland_test.go` already exists, append just the `TestSanitizeDesc` function (and `"testing"` import if missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestSanitizeDesc ./...`
Expected: FAIL — `sanitizeDesc` undefined.

- [ ] **Step 3: Implement `sanitizeDesc`**

In `hyprland.go`, add after the `isValidColorMode` function (around line 68):

```go
// sanitizeDesc validates and trims an EDID description for use in a
// monitor=desc:... line. Returns "" when the string is unsafe for the
// Hyprland parser or for shell-quoted inclusion elsewhere.
func sanitizeDesc(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, r := range s {
		if r == ',' || r == '"' || r == '\n' || r < 0x20 {
			return ""
		}
	}
	return s
}
```

(`strings` is already imported in `hyprland.go`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestSanitizeDesc ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add hyprland.go hyprland_test.go
git commit -m "feat: add sanitizeDesc helper for desc: lines (#75)"
```

---

## Task 3: Add `canUseDescFormat` helper

**Files:**
- Modify: `hyprland.go` (append helper near `sanitizeDesc`)
- Test: `hyprland_test.go`

- [ ] **Step 1: Write the failing test**

Append to `hyprland_test.go`:

```go
func TestCanUseDescFormat(t *testing.T) {
	tests := []struct {
		name    string
		monitor Monitor
		want    bool
	}{
		{
			name: "valid description with serial",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
				EDIDName:   "Dell Inc. DELL U3419W 5HJB6T2",
			},
			want: true,
		},
		{
			name: "empty EDIDName",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
				EDIDName:   "",
			},
			want: false,
		},
		{
			name: "ambiguous: disambiguated HardwareID",
			monitor: Monitor{
				Name:       "DP-9",
				HardwareID: "Dell Inc./DELL U3419W/#1",
				EDIDName:   "Dell Inc. DELL U3419W",
			},
			want: false,
		},
		{
			name: "description contains comma",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "Apple Inc./Studio Display/ABC",
				EDIDName:   "Apple Computer Inc., Apple Studio Display ABC",
			},
			want: false,
		},
		{
			name: "empty HardwareID (no EDID make/model)",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "",
				EDIDName:   "Some Description",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canUseDescFormat(tt.monitor)
			if got != tt.want {
				t.Errorf("canUseDescFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestCanUseDescFormat ./...`
Expected: FAIL — `canUseDescFormat` undefined.

- [ ] **Step 3: Implement `canUseDescFormat`**

In `hyprland.go`, add after `sanitizeDesc`:

```go
// canUseDescFormat reports whether a monitor can safely be written as
// monitor=desc:<description>,... A monitor qualifies when it has a
// non-empty HardwareID that is NOT disambiguated (no /# suffix), and its
// EDIDName survives sanitizeDesc unchanged (non-empty after sanitization).
func canUseDescFormat(m Monitor) bool {
	if m.HardwareID == "" {
		return false
	}
	if strings.Contains(m.HardwareID, "/#") {
		return false
	}
	return sanitizeDesc(m.EDIDName) != ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestCanUseDescFormat ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add hyprland.go hyprland_test.go
git commit -m "feat: add canUseDescFormat gatekeeper (#75)"
```

---

## Task 4: Teach `generateMonitorLine` to emit `desc:` lines

**Files:**
- Modify: `hyprland.go` (function `generateMonitorLine`, around lines 355–406)
- Test: `hyprland_test.go`

- [ ] **Step 1: Write the failing test**

Append to `hyprland_test.go`:

```go
func TestGenerateMonitorLineDescFormat(t *testing.T) {
	base := Monitor{
		Name:       "DP-9",
		HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
		EDIDName:   "Dell Inc. DELL U3419W 5HJB6T2",
		PxW:        3440,
		PxH:        1440,
		Hz:         60,
		X:          0,
		Y:          0,
		Scale:      1.0,
		Active:     true,
	}

	t.Run("desc off writes connector name", func(t *testing.T) {
		m := base
		m.UseDescFormat = false
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on writes description line", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on with disabled monitor", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.Active = false
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,disable"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on with mirror keeps source connector", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.IsMirrored = true
		m.MirrorSource = "DP-1"
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60.00,0x0,1.00,mirror,DP-1"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on but ambiguous falls back to connector name", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.HardwareID = "Dell Inc./DELL U3419W/#1"
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on but empty description falls back", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.EDIDName = ""
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on but description has comma falls back", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.EDIDName = "Apple Computer Inc., Studio Display"
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on with advanced options", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.BitDepth = 10
		m.VRR = 1
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60.00,0x0,1.00,bitdepth,10,vrr,1"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestGenerateMonitorLineDescFormat ./...`
Expected: FAIL — current `generateMonitorLine` always uses `m.Name`.

- [ ] **Step 3: Modify `generateMonitorLine`**

Replace the body of `generateMonitorLine` in `hyprland.go` (lines 355–406) with:

```go
// generateMonitorLine creates the monitor configuration line for a monitor
func generateMonitorLine(m Monitor) string {
	// Validate monitor name (defensive check)
	if !isValidMonitorName(m.Name) {
		return fmt.Sprintf("# Invalid monitor name: %s", m.Name)
	}

	// Resolve identifier: desc:<description> when the user opted in and the
	// description is unambiguous and safe; otherwise the connector name.
	identifier := m.Name
	if m.UseDescFormat && canUseDescFormat(m) {
		if desc := sanitizeDesc(m.EDIDName); desc != "" {
			identifier = "desc:" + desc
		}
	}

	if !m.Active {
		return fmt.Sprintf("monitor=%s,disable", identifier)
	}

	var monLine string
	if m.IsMirrored && m.MirrorSource != "" {
		// Validate mirror source name (defensive check)
		if !isValidMonitorName(m.MirrorSource) {
			return fmt.Sprintf("# Invalid mirror source: %s", m.MirrorSource)
		}
		// Mirror syntax: monitor=IDENT,resolution,position,scale,mirror,SOURCE_CONNECTOR
		monLine = fmt.Sprintf("monitor=%s,%dx%d@%.2f,%dx%d,%.2f,mirror,%s",
			identifier, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale, m.MirrorSource)
	} else {
		// Regular monitor configuration
		monLine = fmt.Sprintf("monitor=%s,%dx%d@%.2f,%dx%d,%.2f",
			identifier, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale)

		// Add advanced settings (only for non-mirrored monitors)
		if m.BitDepth == 10 {
			monLine += ",bitdepth,10"
		}
		if m.ColorMode != "" && m.ColorMode != "srgb" {
			// Validate color mode (defensive check)
			if isValidColorMode(m.ColorMode) {
				monLine += fmt.Sprintf(",cm,%s", m.ColorMode)
			}
		}
		if m.ColorMode == "hdr" || m.ColorMode == "hdredid" {
			if m.SDRBrightness != 0 && m.SDRBrightness != 1.0 {
				monLine += fmt.Sprintf(",sdrbrightness,%.2f", m.SDRBrightness)
			}
			if m.SDRSaturation != 0 && m.SDRSaturation != 1.0 {
				monLine += fmt.Sprintf(",sdrsaturation,%.2f", m.SDRSaturation)
			}
		}
		if m.VRR > 0 {
			monLine += fmt.Sprintf(",vrr,%d", m.VRR)
		}
		if m.Transform > 0 {
			monLine += fmt.Sprintf(",transform,%d", m.Transform)
		}
	}

	return monLine
}
```

Note the change: `m.Name` → `identifier` in the three `fmt.Sprintf` calls (the disable line, the mirror line, and the regular line).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestGenerateMonitorLine ./...`
Expected: all subtests PASS.

Then run the full suite:
Run: `go test ./...`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add hyprland.go hyprland_test.go
git commit -m "feat: generateMonitorLine emits desc: when opted in (#75)"
```

---

## Task 5: Create `settings.go` with load/save/get/set

**Files:**
- Create: `settings.go`
- Create: `settings_test.go`

- [ ] **Step 1: Write the failing test**

Create `settings_test.go` with:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestSettings ./...`
Expected: FAIL — `loadSettings`, `saveSettings`, `getMonitorPref`, `setMonitorPref`, `Settings`, `MonitorPref` undefined.

- [ ] **Step 3: Implement `settings.go`**

Create `settings.go` with:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestSettings ./...`
Expected: PASS.

Run: `go test ./...`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add settings.go settings_test.go
git commit -m "feat: add hyprmon settings file for per-monitor prefs (#75)"
```

---

## Task 6: Merge settings into `readMonitors`

**Files:**
- Modify: `hyprland.go` (`readMonitors`, around lines 134–206)
- Test: `hyprland_test.go`

- [ ] **Step 1: Write the failing test**

Append to `hyprland_test.go`:

```go
func TestApplyMonitorPrefs(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-9", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2"},
		{Name: "DP-10", HardwareID: "Dell Inc./DELL U3419W/OTHER"},
		{Name: "eDP-1", HardwareID: ""},
	}

	s := &Settings{MonitorPrefs: map[string]MonitorPref{
		"Dell Inc./DELL U3419W/5HJB6T2": {UseDescFormat: true},
	}}

	applyMonitorPrefs(monitors, s)

	if !monitors[0].UseDescFormat {
		t.Errorf("monitors[0] (matching hwid) should have UseDescFormat=true")
	}
	if monitors[1].UseDescFormat {
		t.Errorf("monitors[1] (non-matching hwid) should be unchanged (false)")
	}
	if monitors[2].UseDescFormat {
		t.Errorf("monitors[2] (empty hwid) should be unchanged (false)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestApplyMonitorPrefs ./...`
Expected: FAIL — `applyMonitorPrefs` undefined.

- [ ] **Step 3: Add `applyMonitorPrefs` helper and wire it in**

In `hyprland.go`, add after `canUseDescFormat`:

```go
// applyMonitorPrefs merges per-monitor preferences from the hyprmon
// settings file into the given monitor slice. Monitors without a
// HardwareID are skipped (nothing to key on).
func applyMonitorPrefs(monitors []Monitor, s *Settings) {
	if s == nil {
		return
	}
	for i := range monitors {
		if monitors[i].HardwareID == "" {
			continue
		}
		pref := getMonitorPref(s, monitors[i].HardwareID)
		monitors[i].UseDescFormat = pref.UseDescFormat
	}
}
```

Then in `readMonitors()`, after the existing `disambiguateHardwareIDs(monitors)` call (around line 203), add:

```go
	// Merge per-monitor preferences from hyprmon settings file. Best-effort:
	// on read errors we log and continue with defaults.
	if s, err := loadSettings(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load hyprmon settings: %v\n", err)
	} else {
		applyMonitorPrefs(monitors, s)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestApplyMonitorPrefs ./...`
Expected: PASS.

Run: `go test ./...`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add hyprland.go hyprland_test.go
git commit -m "feat: merge settings prefs in readMonitors (#75)"
```

---

## Task 7: Add `desc:` toggle to advanced settings dialog

**Files:**
- Modify: `advanced_settings.go`

No dedicated test — the UI dialog is a `bubbletea` model whose state mutations are exercised indirectly via the monitor pointer. We verify behavior through running the app and the existing test suite compiling.

- [ ] **Step 1: Add the new field constant**

In `advanced_settings.go`, extend the field enum (around lines 18–26):

```go
const (
	fieldBitDepth = iota
	fieldColorMode
	fieldSDRBrightness
	fieldSDRSaturation
	fieldVRR
	fieldTransform
	fieldUseDescFormat
	fieldCount
)
```

- [ ] **Step 2: Handle toggling in `toggleValue`**

Extend the `switch m.focusedField` in `toggleValue()` (around line 124) with a new case, after the `fieldTransform` case:

```go
	case fieldUseDescFormat:
		if canUseDescFormat(*m.monitor) {
			m.monitor.UseDescFormat = !m.monitor.UseDescFormat
			// Persist immediately so the setting survives across sessions
			// even when the user never saves a profile.
			if s, err := loadSettings(); err == nil {
				setMonitorPref(s, m.monitor.HardwareID, MonitorPref{
					UseDescFormat: m.monitor.UseDescFormat,
				})
				_ = saveSettings(s) // best-effort; UI flow continues on error
			}
		}
```

- [ ] **Step 3: Render the new row in `View()`**

In `advanced_settings.go`, after the Transform row rendering (just before the trailing `"\n\n"` around line 281), insert:

```go
	// Write as desc:
	label = "Write as desc:"
	value, disabled := m.renderUseDescFormat()
	if disabled {
		dimLabelStyle := labelStyle.Foreground(lipgloss.Color("238"))
		dimValueStyle := valueStyle.Foreground(lipgloss.Color("238"))
		content.WriteString(dimLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(dimValueStyle.Render(value))
	} else if m.focusedField == fieldUseDescFormat {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")
```

- [ ] **Step 4: Add `renderUseDescFormat`**

Append to `advanced_settings.go`, after `renderTransform`:

```go
// renderUseDescFormat returns (value, disabled). When disabled is true the
// toggle cannot be flipped (the row is rendered dimmed by the caller) and
// `value` explains why.
func (m advancedSettingsModel) renderUseDescFormat() (string, bool) {
	if m.monitor.EDIDName == "" {
		return "(unavailable — no EDID description)", true
	}
	if m.monitor.HardwareID == "" {
		return "(unavailable — no EDID description)", true
	}
	if strings.Contains(m.monitor.HardwareID, "/#") {
		return "(unavailable — description not unique)", true
	}
	if sanitizeDesc(m.monitor.EDIDName) == "" {
		return "(unavailable — description contains unsupported characters)", true
	}
	if m.monitor.UseDescFormat {
		return "● On  ○ Off", false
	}
	return "○ On  ● Off", false
}
```

- [ ] **Step 5: Skip the disabled field in navigation**

Both `navigateDown` (lines 67–80) and `navigateUp` (lines 82–99) should skip `fieldUseDescFormat` when it's unavailable. Replace the two functions with:

```go
func (m *advancedSettingsModel) navigateDown() {
	isHDR := strings.Contains(m.monitor.ColorMode, "hdr")
	descDisabled := !canUseDescFormat(*m.monitor)

	for i := 0; i < fieldCount; i++ {
		m.focusedField++
		if m.focusedField >= fieldCount {
			m.focusedField = 0
		}
		if !isHDR && (m.focusedField == fieldSDRBrightness || m.focusedField == fieldSDRSaturation) {
			continue
		}
		if descDisabled && m.focusedField == fieldUseDescFormat {
			continue
		}
		return
	}
}

func (m *advancedSettingsModel) navigateUp() {
	isHDR := strings.Contains(m.monitor.ColorMode, "hdr")
	descDisabled := !canUseDescFormat(*m.monitor)

	for i := 0; i < fieldCount; i++ {
		m.focusedField--
		if m.focusedField < 0 {
			m.focusedField = fieldCount - 1
		}
		if !isHDR && (m.focusedField == fieldSDRBrightness || m.focusedField == fieldSDRSaturation) {
			continue
		}
		if descDisabled && m.focusedField == fieldUseDescFormat {
			continue
		}
		return
	}
}
```

This is a small refactor: the loop form is simpler and correctly handles the newly added skippable field.

- [ ] **Step 6: Verify the build and tests**

Run: `go build ./...`
Expected: no errors.

Run: `go test ./...`
Expected: all tests PASS.

Run the app manually (optional, requires a live Hyprland session):
```bash
make build && ./hyprmon
```
Press `a` on a monitor, verify the new "Write as desc:" row appears, toggles with Space, is dimmed for monitors with non-unique descriptions, and persists to `~/.config/hyprmon/settings.json`.

- [ ] **Step 7: Commit**

```bash
git add advanced_settings.go
git commit -m "feat: add 'Write as desc:' toggle to advanced settings (#75)"
```

---

## Task 8: Documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update README.md**

Find an appropriate place — ideally after any existing "Profiles" or "Configuration" section. Add a subsection:

```markdown
### Stable monitor matching with `desc:` format

By default, HyprMon writes monitor lines keyed by connector name (e.g., `monitor=DP-9,…`). For daisy-chained or otherwise indistinguishable monitors, the kernel may assign connector names in a different order across reboots or replugs, which swaps monitor positions.

Hyprland supports matching by EDID description instead:

```
monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60,0x0,1.00
```

To opt in per monitor:

1. Select the monitor and press `a` to open advanced settings.
2. Toggle **Write as desc:** to On.
3. Save your configuration (`Shift+S`) to write `hyprland.conf` in the new format.

The toggle is unavailable when the monitor has no EDID description, when two or more connected monitors share the same description (typically identical monitors without a serial number), or when the description contains characters Hyprland cannot parse. The preference persists across sessions in `~/.config/hyprmon/settings.json` and is also stored inside any profile you save that includes the monitor.

Live application via `hyprctl` continues to use connector names — the `desc:` format applies only to the persisted `hyprland.conf`.
```

- [ ] **Step 2: Update CLAUDE.md**

In the "Monitor Data Flow" section, append a new numbered point (after the existing profile point):

```markdown
5. Per-monitor `UseDescFormat` preference: stored by HardwareID in `~/.config/hyprmon/settings.json` and also serialized into profile JSON; when true, `generateMonitorLine` writes `monitor=desc:<description>,…` to hyprland.conf in place of the connector name. `applyMonitor` is unaffected — live `hyprctl keyword monitor` continues to use the connector name.
```

Also add an entry under "Important File Paths":

```markdown
- HyprMon settings: `~/.config/hyprmon/settings.json` (per-monitor preferences keyed by HardwareID)
```

- [ ] **Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: document desc: monitor line format (#75)"
```

---

## Final verification

- [ ] Run the full test suite:

```bash
go test ./...
```
Expected: all tests PASS.

- [ ] Format and lint:

```bash
make fmt
make lint
```
Expected: no changes from fmt, no lint issues.

- [ ] Manual smoke test (requires live Hyprland):

```bash
make build && ./hyprmon
```

1. Select a monitor with a unique EDID description. Press `a` → "Write as desc:" is available.
2. Toggle on, press Enter, press `Shift+S` to save.
3. Inspect `~/.config/hypr/hyprland.conf` — the monitor line should start with `monitor=desc:<description>`.
4. Inspect `~/.config/hyprmon/settings.json` — preference recorded with the monitor's HardwareID.
5. For a monitor with `#` in its disambiguated HardwareID, the toggle is dim and labeled "(unavailable — description not unique)".

---

## Self-review notes

Spec coverage map:

- Spec §"Data model → Monitor struct" → Task 1.
- Spec §"Data model → Settings file" → Task 5.
- Spec §"Data model → Precedence layering" → Task 6 (settings → monitors) + Task 7 (UI → settings) + existing profile flow (untouched).
- Spec §"Write logic → generateMonitorLine" → Task 4.
- Spec §"Write logic → canUseDescFormat" → Task 3.
- Spec §"Write logic → sanitizeDesc" → Task 2.
- Spec §"Write logic → applyMonitor unchanged" → not a task; verified by NOT modifying `applyMonitor`.
- Spec §"Read logic" → Task 6.
- Spec §"UI" → Task 7.
- Spec §"Lifecycle details" → Task 7 (toggle → immediate save; no profile-apply write to settings) + profile save (no code change needed, Monitor JSON round-trip confirmed in Task 1).
- Spec §"Testing" → Tasks 1–6 each include tests from the listed matrix.
- Spec §"Documentation" → Task 8.
- Spec §"File impact summary" → every entry maps to a task above.
