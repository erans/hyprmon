package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestGetConfigTargetUsesHYPRLANDCONFIGLua(t *testing.T) {
	t.Setenv("HYPRLAND_CONFIG", "/tmp/hyprland.lua")

	target, err := getConfigTarget()
	if err != nil {
		t.Fatalf("getConfigTarget() error = %v", err)
	}
	if target.Path != "/tmp/hyprland.lua" {
		t.Fatalf("Path = %q, want /tmp/hyprland.lua", target.Path)
	}
	if target.Format != configFormatLua {
		t.Fatalf("Format = %v, want configFormatLua", target.Format)
	}
}

func TestGetConfigTargetUsesHYPRLANDCONFIGConf(t *testing.T) {
	t.Setenv("HYPRLAND_CONFIG", "/tmp/hyprland.conf")

	target, err := getConfigTarget()
	if err != nil {
		t.Fatalf("getConfigTarget() error = %v", err)
	}
	if target.Path != "/tmp/hyprland.conf" {
		t.Fatalf("Path = %q, want /tmp/hyprland.conf", target.Path)
	}
	if target.Format != configFormatHyprlang {
		t.Fatalf("Format = %v, want configFormatHyprlang", target.Format)
	}
}

func TestGetConfigTargetRejectsUnsafeHYPRLANDCONFIG(t *testing.T) {
	t.Setenv("HYPRLAND_CONFIG", "relative/hyprland.lua")

	if _, err := getConfigTarget(); err == nil {
		t.Fatalf("getConfigTarget() error = nil, want error")
	}
}

func TestGetConfigTargetPrefersDefaultLuaWhenPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYPRLAND_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	hyprDir := filepath.Join(dir, "hypr")
	if err := os.MkdirAll(hyprDir, 0700); err != nil {
		t.Fatal(err)
	}
	luaPath := filepath.Join(hyprDir, "hyprland.lua")
	if err := os.WriteFile(luaPath, []byte("-- lua"), 0600); err != nil {
		t.Fatal(err)
	}

	target, err := getConfigTarget()
	if err != nil {
		t.Fatalf("getConfigTarget() error = %v", err)
	}
	if target.Path != luaPath {
		t.Fatalf("Path = %q, want %q", target.Path, luaPath)
	}
	if target.Format != configFormatLua {
		t.Fatalf("Format = %v, want configFormatLua", target.Format)
	}
}

func TestGetConfigTargetFallsBackToDefaultConf(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYPRLAND_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", dir)

	target, err := getConfigTarget()
	if err != nil {
		t.Fatalf("getConfigTarget() error = %v", err)
	}
	want := filepath.Join(dir, "hypr", "hyprland.conf")
	if target.Path != want {
		t.Fatalf("Path = %q, want %q", target.Path, want)
	}
	if target.Format != configFormatHyprlang {
		t.Fatalf("Format = %v, want configFormatHyprlang", target.Format)
	}
}

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

func TestGenerateLuaMonitorRule(t *testing.T) {
	m := Monitor{
		Name:   "DP-1",
		PxW:    2560,
		PxH:    1440,
		Hz:     144,
		X:      0,
		Y:      -200,
		Scale:  1.25,
		Active: true,
	}

	got := generateLuaMonitorRule(m)
	want := `hl.monitor({ output = "DP-1", mode = "2560x1440@144.00", position = "0x-200", scale = 1.25 })`
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

func TestGenerateLuaMonitorRuleDisabled(t *testing.T) {
	m := Monitor{Name: "eDP-1", Active: false}

	got := generateLuaMonitorRule(m)
	want := `hl.monitor({ output = "eDP-1", disabled = true })`
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

func TestGenerateLuaMonitorRuleUsesDescFormat(t *testing.T) {
	m := Monitor{
		Name:          "DP-9",
		HardwareID:    "Dell Inc./DELL U3419W/5HJB6T2",
		EDIDName:      "Dell Inc. DELL U3419W 5HJB6T2",
		UseDescFormat: true,
		PxW:           3440,
		PxH:           1440,
		Hz:            60,
		Scale:         1,
		Active:        true,
	}

	got := generateLuaMonitorRule(m)
	want := `hl.monitor({ output = "desc:Dell Inc. DELL U3419W 5HJB6T2", mode = "3440x1440@60.00", position = "0x0", scale = 1.00 })`
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

func TestGenerateLuaMonitorRuleMirrored(t *testing.T) {
	m := Monitor{
		Name:         "DP-2",
		PxW:          1920,
		PxH:          1080,
		Hz:           60,
		Scale:        1,
		Active:       true,
		IsMirrored:   true,
		MirrorSource: "DP-1",
	}

	got := generateLuaMonitorRule(m)
	want := `hl.monitor({ output = "DP-2", mode = "1920x1080@60.00", position = "0x0", scale = 1.00, mirror = "DP-1" })`
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

func TestGenerateLuaMonitorRuleAdvanced(t *testing.T) {
	m := Monitor{
		Name:          "DP-1",
		PxW:           3840,
		PxH:           2160,
		Hz:            60,
		X:             0,
		Y:             0,
		Scale:         1,
		Active:        true,
		BitDepth:      10,
		ColorMode:     "hdr",
		SDRBrightness: 1.2,
		SDRSaturation: 0.9,
		VRR:           1,
		Transform:     1,
	}

	got := generateLuaMonitorRule(m)
	want := `hl.monitor({ output = "DP-1", mode = "3840x2160@60.00", position = "0x0", scale = 1.00, bitdepth = 10, cm = "hdr", sdrbrightness = 1.20, sdrsaturation = 0.90, vrr = 1, transform = 1 })`
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

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

func TestWriteConfigUsesHyprlangWriterForConf(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "hyprland.conf")
	input := strings.Join([]string{
		"# before",
		"monitor=OLD,preferred,auto,1",
		"# after",
	}, "\n")
	if err := os.WriteFile(confPath, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYPRLAND_CONFIG", confPath)

	monitors := []Monitor{{
		Name:   "DP-1",
		PxW:    1920,
		PxH:    1080,
		Hz:     60,
		Scale:  1,
		Active: true,
	}}

	if err := writeConfig(monitors); err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "monitor=DP-1,1920x1080@60.00,0x0,1.00") {
		t.Fatalf("hyprland.conf did not contain generated monitor line:\n%s", got)
	}
	if fileExists(filepath.Join(dir, "hyprmon.lua")) {
		t.Fatalf("hyprmon.lua should not be created for hyprland.conf writer")
	}
}

func TestWriteConfigUsesLuaSidecarForLua(t *testing.T) {
	dir := t.TempDir()
	luaPath := filepath.Join(dir, "hyprland.lua")
	if err := os.WriteFile(luaPath, []byte("-- user config\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYPRLAND_CONFIG", luaPath)

	monitors := []Monitor{{
		Name:   "DP-1",
		PxW:    2560,
		PxH:    1440,
		Hz:     144,
		X:      0,
		Y:      -200,
		Scale:  1.25,
		Active: true,
	}}

	if err := writeConfig(monitors); err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}

	mainData, err := os.ReadFile(luaPath)
	if err != nil {
		t.Fatal(err)
	}
	mainConfig := string(mainData)
	if !strings.Contains(mainConfig, "-- user config") {
		t.Fatalf("hyprland.lua did not preserve existing content:\n%s", mainConfig)
	}
	if strings.Count(mainConfig, `require("hyprmon")`) != 1 {
		t.Fatalf("hyprland.lua should contain exactly one managed require:\n%s", mainConfig)
	}

	sidecarPath := filepath.Join(dir, "hyprmon.lua")
	sidecarData, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("failed to read hyprmon.lua: %v", err)
	}
	sidecar := string(sidecarData)
	wantRule := `hl.monitor({ output = "DP-1", mode = "2560x1440@144.00", position = "0x-200", scale = 1.25 })`
	if !strings.Contains(sidecar, wantRule) {
		t.Fatalf("hyprmon.lua did not contain generated rule %q:\n%s", wantRule, sidecar)
	}
}

func TestWriteLuaConfigDoesNotDuplicateRequire(t *testing.T) {
	dir := t.TempDir()
	luaPath := filepath.Join(dir, "hyprland.lua")
	input := strings.Join([]string{
		"-- user config",
		"-- hyprmon: managed monitor profile include",
		`require("hyprmon")`,
	}, "\n")
	if err := os.WriteFile(luaPath, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYPRLAND_CONFIG", luaPath)

	monitors := []Monitor{{Name: "eDP-1", Active: false}}

	if err := writeConfig(monitors); err != nil {
		t.Fatalf("writeConfig() first call error = %v", err)
	}
	if err := writeConfig(monitors); err != nil {
		t.Fatalf("writeConfig() second call error = %v", err)
	}

	mainData, err := os.ReadFile(luaPath)
	if err != nil {
		t.Fatal(err)
	}
	mainConfig := string(mainData)
	if strings.Count(mainConfig, `require("hyprmon")`) != 1 {
		t.Fatalf("hyprland.lua should contain exactly one managed require after repeated writes:\n%s", mainConfig)
	}

	sidecarData, err := os.ReadFile(filepath.Join(dir, "hyprmon.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sidecarData), `hl.monitor({ output = "eDP-1", disabled = true })`) {
		t.Fatalf("hyprmon.lua did not contain disabled monitor rule:\n%s", string(sidecarData))
	}
}

func TestEnsureHyprmonLuaRequireIgnoresCommentedRequire(t *testing.T) {
	input := strings.Join([]string{
		"-- user config",
		`-- require("hyprmon")`,
	}, "\n")

	got := ensureHyprmonLuaRequire(input)

	if strings.Count(got, `require("hyprmon")`) != 2 {
		t.Fatalf("expected commented require plus active managed require:\n%s", got)
	}
	if !strings.Contains(got, hyprmonLuaRequireComment+"\n"+hyprmonLuaRequireLine) {
		t.Fatalf("expected managed require block:\n%s", got)
	}
}
