# Hyprland Lua Config Writer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add persistence support for Hyprland 0.55+ Lua monitor configuration while preserving the existing hyprlang writer.

**Architecture:** Split config persistence into target detection plus format-specific writers. Existing `monitor=...` generation remains the hyprlang backend; Lua support adds `hl.monitor({...})` rendering and writes monitor rules through a HyprMon-managed `hyprmon.lua` sidecar required by `hyprland.lua`.

**Tech Stack:** Go standard library, existing HyprMon monitor model, existing `go test ./...` suite.

---

### Task 1: Detect Config Target Format

**Files:**
- Modify: `hyprland.go`
- Modify: `hyprland_test.go`

- [ ] **Step 1: Write failing tests**

Add tests covering:

```go
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
	if target.Format != configFormatHyprlang {
		t.Fatalf("Format = %v, want configFormatHyprlang", target.Format)
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
	if target.Path != luaPath || target.Format != configFormatLua {
		t.Fatalf("target = %#v, want %q lua", target, luaPath)
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
	if target.Path != want || target.Format != configFormatHyprlang {
		t.Fatalf("target = %#v, want %q hyprlang", target, want)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`

Expected: compile failure because `getConfigTarget`, `configFormatLua`, and `configFormatHyprlang` do not exist.

- [ ] **Step 3: Implement detection**

Add `configFormat`, `configTarget`, `fileExists`, `getHyprConfigDir`, and `getConfigTarget` to `hyprland.go`. Keep `getConfigPath` as a compatibility wrapper returning only the selected path.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./...`

Expected: all tests pass.

### Task 2: Render Lua Monitor Rules

**Files:**
- Modify: `hyprland.go`
- Modify: `hyprland_test.go`

- [ ] **Step 1: Write failing tests**

Add tests for active, disabled, mirrored, and advanced Lua monitor rules:

```go
func TestGenerateLuaMonitorRule(t *testing.T) {
	m := Monitor{Name: "DP-1", PxW: 2560, PxH: 1440, Hz: 144, X: 0, Y: -200, Scale: 1.25, Active: true}
	got := generateLuaMonitorRule(m)
	want := "hl.monitor({ output = \"DP-1\", mode = \"2560x1440@144.00\", position = \"0x-200\", scale = 1.25 })"
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

func TestGenerateLuaMonitorRuleDisabled(t *testing.T) {
	m := Monitor{Name: "eDP-1", Active: false}
	got := generateLuaMonitorRule(m)
	want := "hl.monitor({ output = \"eDP-1\", disabled = true })"
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}

func TestGenerateLuaMonitorRuleAdvanced(t *testing.T) {
	m := Monitor{Name: "DP-1", PxW: 3840, PxH: 2160, Hz: 60, X: 0, Y: 0, Scale: 1, Active: true, BitDepth: 10, ColorMode: "hdr", SDRBrightness: 1.2, SDRSaturation: 0.9, VRR: 1, Transform: 1}
	got := generateLuaMonitorRule(m)
	want := "hl.monitor({ output = \"DP-1\", mode = \"3840x2160@60.00\", position = \"0x0\", scale = 1.00, bitdepth = 10, cm = \"hdr\", sdrbrightness = 1.20, sdrsaturation = 0.90, vrr = 1, transform = 1 })"
	if got != want {
		t.Fatalf("generateLuaMonitorRule() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`

Expected: compile failure because `generateLuaMonitorRule` does not exist.

- [ ] **Step 3: Implement Lua renderer**

Add `generateLuaMonitorRule`, using `resolveMonitorIdentifier` shared with `generateMonitorLine`. Escape Lua strings with `%q`; validate monitor and mirror names like the existing writer.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./...`

Expected: all tests pass.

### Task 3: Dispatch Writers and Manage Lua Sidecar

**Files:**
- Modify: `hyprland.go`
- Modify: `hyprland_test.go`

- [ ] **Step 1: Write failing tests**

Add temp-file tests that verify:
- `writeConfig` dispatches to hyprlang when `HYPRLAND_CONFIG` points at `.conf`.
- `writeConfig` dispatches to Lua when `HYPRLAND_CONFIG` points at `.lua`.
- Lua mode creates or updates `hyprmon.lua` next to `hyprland.lua`.
- Lua mode appends exactly one `require("hyprmon")` line to `hyprland.lua`.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./...`

Expected: tests fail because `writeConfig` still only edits monitor lines in the selected file.

- [ ] **Step 3: Implement dispatch and Lua sidecar writing**

Rename the existing body to `writeHyprlangConfig(configPath string, monitors []Monitor) error`. Make `writeConfig` call `getConfigTarget` and dispatch. Add `writeLuaConfig(configPath string, monitors []Monitor) error`, which backs up `hyprland.lua`, writes `hyprmon.lua`, and ensures a managed `require("hyprmon")` line exists in `hyprland.lua` without duplicating it.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./...`

Expected: all tests pass.

### Task 4: Documentation and Final Verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README**

Document that HyprMon now writes either:
- `hyprland.conf` monitor lines for legacy hyprlang config.
- `hyprmon.lua` plus `require("hyprmon")` for Lua config.

- [ ] **Step 2: Format and test**

Run:

```bash
gofmt -w hyprland.go hyprland_test.go
go test ./...
git status --short
```

Expected: tests pass and only planned files are modified.
