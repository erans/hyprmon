package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// hyprctlTimeout is the timeout for hyprctl commands
	hyprctlTimeout = 5 * time.Second

	// File permissions
	configFileMode  = 0600 // rw------- (user-only access for config files)
	backupFileMode  = 0600 // rw------- (user-only access for backups)
	profileDirMode  = 0700 // rwx------ (user-only access for profile directory)
	profileFileMode = 0600 // rw------- (user-only access for profile files)

	// Default world dimensions
	defaultWorldWidth  = 3840
	defaultWorldHeight = 2160
	defaultWorldScale  = 1.0

	// World bounds padding
	worldPaddingPx = 500

	// UI layout constants
	desktopBorderMargin = 3  // Border (2) + margin (1)
	desktopFooterHeight = 10 // Height reserved for footer

	hyprmonLuaRequireComment = "-- hyprmon: managed monitor profile include"
	hyprmonLuaRequireLine    = `require("hyprmon")`
)

type configFormat int

const (
	configFormatHyprlang configFormat = iota
	configFormatLua
)

type configTarget struct {
	Path   string
	Format configFormat
}

// isValidMonitorName validates that a monitor name is safe to use in commands
func isValidMonitorName(name string) bool {
	if name == "" {
		return false
	}
	// Monitor names should only contain alphanumeric, dash, underscore, and dot
	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		isSpecial := r == '-' || r == '_' || r == '.'
		if !isLower && !isUpper && !isDigit && !isSpecial {
			return false
		}
	}
	return true
}

// isValidColorMode validates that a color mode is from the allowed set
func isValidColorMode(mode string) bool {
	validModes := map[string]bool{
		"auto":    true,
		"srgb":    true,
		"wide":    true,
		"edid":    true,
		"hdr":     true,
		"hdredid": true,
		"":        true, // empty is valid (default)
	}
	return validModes[mode]
}

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

// execHyprctl executes a hyprctl command with the given arguments and returns the output
func execHyprctl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hyprctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute hyprctl %v: %w", args, err)
	}
	return output, nil
}

// execHyprctlJSON executes a hyprctl command and unmarshals the JSON output into the provided result
func execHyprctlJSON(result interface{}, args ...string) error {
	output, err := execHyprctl(args...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(output, result); err != nil {
		return fmt.Errorf("failed to parse JSON from hyprctl %v: %w", args, err)
	}
	return nil
}

type hyprMonitor struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Serial          string  `json:"serial"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	RefreshRate     float64 `json:"refreshRate"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	ActiveWorkspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
	Reserved        []int    `json:"reserved"`
	Scale           float64  `json:"scale"`
	Transform       int      `json:"transform"`
	Focused         bool     `json:"focused"`
	DpmsStatus      bool     `json:"dpmsStatus"`
	VRR             bool     `json:"vrr"`
	ActivelyTearing bool     `json:"activelyTearing"`
	Disabled        bool     `json:"disabled"`
	CurrentFormat   string   `json:"currentFormat"`
	MirrorOf        string   `json:"mirrorOf"`
	AvailableModes  []string `json:"availableModes"`
}

type hyprWorkspace struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Monitor    string `json:"monitor"`
	MonitorID  int    `json:"monitorID"`
	Windows    int    `json:"windows"`
	Persistent bool   `json:"ispersistent"`
}

func readMonitors() ([]Monitor, error) {
	var hyprMonitors []hyprMonitor
	if err := execHyprctlJSON(&hyprMonitors, "monitors", "all", "-j"); err != nil {
		return nil, err
	}

	monitors := make([]Monitor, 0, len(hyprMonitors))

	// First pass: create monitors without mirror relationships
	for _, hm := range hyprMonitors {
		modes := make([]Mode, 0, len(hm.AvailableModes))
		for _, modeStr := range hm.AvailableModes {
			if mode := parseMode(modeStr); mode != nil {
				modes = append(modes, *mode)
			}
		}

		monitor := Monitor{
			Name:       hm.Name,
			Make:       hm.Make,
			Model:      hm.Model,
			Serial:     hm.Serial,
			HardwareID: buildHardwareID(hm.Make, hm.Model, hm.Serial),
			PxW:        uint32(hm.Width),
			PxH:        uint32(hm.Height),
			Hz:         float32(hm.RefreshRate),
			Scale:      float32(hm.Scale),
			X:          int32(hm.X),
			Y:          int32(hm.Y),
			Active:     !hm.Disabled,
			EDIDName:   hm.Description,
			Modes:      modes,

			// Advanced display settings
			Transform: int(hm.Transform),
			VRR: func() int {
				if hm.VRR {
					return 1
				}
				return 0
			}(),

			// Mirror settings
			IsMirrored: hm.MirrorOf != "" && hm.MirrorOf != "none",
			MirrorSource: func() string {
				if hm.MirrorOf != "" && hm.MirrorOf != "none" {
					return hm.MirrorOf
				}
				return ""
			}(),
			MirrorTargets: []string{}, // Will be populated in second pass
		}
		monitors = append(monitors, monitor)
	}

	// Second pass: build mirror targets lists
	for i := range monitors {
		if monitors[i].IsMirrored && monitors[i].MirrorSource != "" {
			// Find the source monitor and add this monitor to its targets
			for j := range monitors {
				if monitors[j].Name == monitors[i].MirrorSource {
					monitors[j].MirrorTargets = append(monitors[j].MirrorTargets, monitors[i].Name)
					break
				}
			}
		}
	}

	// Disambiguate monitors with identical HardwareIDs
	disambiguateHardwareIDs(monitors)

	// Merge per-monitor preferences from hyprmon settings file. Best-effort:
	// on read errors we log and continue with defaults.
	if s, err := loadSettings(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load hyprmon settings: %v\n", err)
	} else {
		applyMonitorPrefs(monitors, s)
	}

	return monitors, nil
}

// getAvailableModes returns the available modes for a specific monitor
func getAvailableModes(monitorName string) ([]string, error) {
	var hyprMonitors []hyprMonitor
	if err := execHyprctlJSON(&hyprMonitors, "monitors", "all", "-j"); err != nil {
		return nil, err
	}

	for _, hm := range hyprMonitors {
		if hm.Name == monitorName {
			return hm.AvailableModes, nil
		}
	}

	return nil, fmt.Errorf("monitor %s not found", monitorName)
}

func parseMode(modeStr string) *Mode {
	parts := strings.Split(modeStr, "@")
	if len(parts) != 2 {
		return nil
	}

	resParts := strings.Split(parts[0], "x")
	if len(resParts) != 2 {
		return nil
	}

	w, err := strconv.ParseUint(resParts[0], 10, 32)
	if err != nil {
		return nil
	}

	h, err := strconv.ParseUint(resParts[1], 10, 32)
	if err != nil {
		return nil
	}

	hzStr := strings.TrimSuffix(parts[1], "Hz")
	hz, err := strconv.ParseFloat(hzStr, 32)
	if err != nil {
		return nil
	}

	return &Mode{
		W:  uint32(w),
		H:  uint32(h),
		Hz: float32(hz),
	}
}

func applyMonitor(m Monitor) error {
	target, err := getConfigTarget()
	if err != nil {
		return fmt.Errorf("could not determine config path: %w", err)
	}

	switch target.Format {
	case configFormatLua:
		return applyMonitorLua(m)
	default:
		return applyMonitorHyprlang(m)
	}
}

func applyMonitorLua(m Monitor) error {
	_, err := execHyprctl("eval", generateLuaMonitorRule(m))
	return err
}

func applyMonitorHyprlang(m Monitor) error {
	// Validate monitor name to prevent command injection
	if !isValidMonitorName(m.Name) {
		return fmt.Errorf("invalid monitor name: %s", m.Name)
	}

	// Validate color mode if set
	if !isValidColorMode(m.ColorMode) {
		return fmt.Errorf("invalid color mode: %s", m.ColorMode)
	}

	var cmd string
	if m.Active {
		if m.IsMirrored && m.MirrorSource != "" {
			// Validate mirror source name
			if !isValidMonitorName(m.MirrorSource) {
				return fmt.Errorf("invalid mirror source name: %s", m.MirrorSource)
			}
			// Mirror syntax: monitor=NAME,resolution,position,scale,mirror,SOURCE_MONITOR
			cmd = fmt.Sprintf("hyprctl keyword monitor \"%s,%dx%d@%.2f,%dx%d,%.2f,mirror,%s\"",
				m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale, m.MirrorSource)
		} else {
			// Build base command for regular monitor
			cmd = fmt.Sprintf("hyprctl keyword monitor \"%s,%dx%d@%.2f,%dx%d,%.2f",
				m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale)

			// Add advanced settings (only for non-mirrored monitors)
			if m.BitDepth == 10 {
				cmd += ",bitdepth,10"
			}

			if m.ColorMode != "" && m.ColorMode != "srgb" {
				cmd += fmt.Sprintf(",cm,%s", m.ColorMode)
			}

			// SDR settings only apply when in HDR mode
			if m.ColorMode == "hdr" || m.ColorMode == "hdredid" {
				if m.SDRBrightness != 0 && m.SDRBrightness != 1.0 {
					cmd += fmt.Sprintf(",sdrbrightness,%.2f", m.SDRBrightness)
				}
				if m.SDRSaturation != 0 && m.SDRSaturation != 1.0 {
					cmd += fmt.Sprintf(",sdrsaturation,%.2f", m.SDRSaturation)
				}
			}

			if m.VRR > 0 {
				cmd += fmt.Sprintf(",vrr,%d", m.VRR)
			}

			if m.Transform > 0 {
				cmd += fmt.Sprintf(",transform,%d", m.Transform)
			}

			cmd += "\""
		}
	} else {
		cmd = fmt.Sprintf("hyprctl keyword monitor \"%s,disable\"", m.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
	defer cancel()

	return exec.CommandContext(ctx, "sh", "-c", cmd).Run()
}

func applyMonitors(monitors []Monitor) error {
	for _, m := range monitors {
		if err := applyMonitor(m); err != nil {
			return fmt.Errorf("failed to apply monitor %s: %w", m.Name, err)
		}
	}
	return nil
}

func cleanAbsoluteConfigPath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("config path must be absolute: %s", path)
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("config path must not contain '..': %s", path)
	}
	return cleanPath, nil
}

func configFormatForPath(path string) configFormat {
	if strings.EqualFold(filepath.Ext(path), ".lua") {
		return configFormatLua
	}
	return configFormatHyprlang
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func getHyprConfigDir() (string, error) {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		cleanPath := filepath.Clean(xdgConfigHome)
		if filepath.IsAbs(cleanPath) {
			return filepath.Join(cleanPath, "hypr"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "hypr"), nil
}

func getConfigTarget() (configTarget, error) {
	if envPath := os.Getenv("HYPRLAND_CONFIG"); envPath != "" {
		cleanPath, err := cleanAbsoluteConfigPath(envPath)
		if err != nil {
			return configTarget{}, err
		}
		return configTarget{
			Path:   cleanPath,
			Format: configFormatForPath(cleanPath),
		}, nil
	}

	hyprDir, err := getHyprConfigDir()
	if err != nil {
		return configTarget{}, err
	}

	luaPath := filepath.Join(hyprDir, "hyprland.lua")
	if fileExists(luaPath) {
		return configTarget{
			Path:   luaPath,
			Format: configFormatLua,
		}, nil
	}

	return configTarget{
		Path:   filepath.Join(hyprDir, "hyprland.conf"),
		Format: configFormatHyprlang,
	}, nil
}

func resolveMonitorIdentifier(m Monitor) string {
	identifier := m.Name
	if m.UseDescFormat && canUseDescFormat(m) {
		if desc := sanitizeDesc(m.EDIDName); desc != "" {
			identifier = "desc:" + desc
		}
	}
	return identifier
}

// generateMonitorLine creates the monitor configuration line for a monitor
func generateMonitorLine(m Monitor) string {
	// Validate monitor name (defensive check)
	if !isValidMonitorName(m.Name) {
		return fmt.Sprintf("# Invalid monitor name: %s", m.Name)
	}

	// Resolve identifier: desc:<description> when the user opted in and the
	// description is unambiguous and safe; otherwise the connector name.
	identifier := resolveMonitorIdentifier(m)

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

func luaString(s string) string {
	return fmt.Sprintf("%q", s)
}

func generateLuaMonitorRule(m Monitor) string {
	if !isValidMonitorName(m.Name) {
		return fmt.Sprintf("-- Invalid monitor name: %s", m.Name)
	}

	identifier := resolveMonitorIdentifier(m)
	fields := []string{fmt.Sprintf("output = %s", luaString(identifier))}

	if !m.Active {
		fields = append(fields, "disabled = true")
		return fmt.Sprintf("hl.monitor({ %s })", strings.Join(fields, ", "))
	}

	fields = append(fields,
		fmt.Sprintf("mode = %s", luaString(fmt.Sprintf("%dx%d@%.2f", m.PxW, m.PxH, m.Hz))),
		fmt.Sprintf("position = %s", luaString(fmt.Sprintf("%dx%d", m.X, m.Y))),
		fmt.Sprintf("scale = %.2f", m.Scale),
		fmt.Sprintf("disabled = false"),
	)

	if m.IsMirrored && m.MirrorSource != "" {
		if !isValidMonitorName(m.MirrorSource) {
			return fmt.Sprintf("-- Invalid mirror source: %s", m.MirrorSource)
		}
		fields = append(fields, fmt.Sprintf("mirror = %s", luaString(m.MirrorSource)))
	} else {
		if m.BitDepth == 10 {
			fields = append(fields, "bitdepth = 10")
		}
		if m.ColorMode != "" && m.ColorMode != "srgb" && isValidColorMode(m.ColorMode) {
			fields = append(fields, fmt.Sprintf("cm = %s", luaString(m.ColorMode)))
		}
		if m.ColorMode == "hdr" || m.ColorMode == "hdredid" {
			if m.SDRBrightness != 0 && m.SDRBrightness != 1.0 {
				fields = append(fields, fmt.Sprintf("sdrbrightness = %.2f", m.SDRBrightness))
			}
			if m.SDRSaturation != 0 && m.SDRSaturation != 1.0 {
				fields = append(fields, fmt.Sprintf("sdrsaturation = %.2f", m.SDRSaturation))
			}
		}
		if m.VRR > 0 {
			fields = append(fields, fmt.Sprintf("vrr = %d", m.VRR))
		}
		if m.Transform > 0 {
			fields = append(fields, fmt.Sprintf("transform = %d", m.Transform))
		}
	}

	return fmt.Sprintf("hl.monitor({ %s })", strings.Join(fields, ", "))
}

func writeConfig(monitors []Monitor) error {
	target, err := getConfigTarget()
	if err != nil {
		return fmt.Errorf("could not determine config path: %w", err)
	}

	switch target.Format {
	case configFormatLua:
		return writeLuaConfig(target.Path, monitors)
	default:
		return writeHyprlangConfig(target.Path, monitors)
	}
}

func writeHyprlangConfig(configPath string, monitors []Monitor) error {
	if configPath == "" {
		return fmt.Errorf("could not determine config path")
	}

	backupPath := fmt.Sprintf("%s.bak.%d", configPath, time.Now().Unix())

	input, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := os.WriteFile(backupPath, input, backupFileMode); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	lines := strings.Split(string(input), "\n")
	var newLines []string
	inMonitorSection := false
	monitorLinesWritten := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "monitor=") || strings.HasPrefix(trimmed, "monitor ") {
			if !monitorLinesWritten {
				for _, m := range monitors {
					newLines = append(newLines, generateMonitorLine(m))
				}
				monitorLinesWritten = true
			}
			inMonitorSection = true
			continue
		}

		if inMonitorSection && trimmed != "" && !strings.HasPrefix(trimmed, "monitor") {
			inMonitorSection = false
		}

		if !inMonitorSection || trimmed == "" {
			newLines = append(newLines, line)
		}
	}

	if !monitorLinesWritten {
		newLines = append(newLines, "")
		for _, m := range monitors {
			newLines = append(newLines, generateMonitorLine(m))
		}
	}

	// Open the file once to avoid TOCTOU race condition
	// This also preserves symlinks by writing through them
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("failed to open config for writing: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close config: %w", closeErr)
		}
	}()

	// Write the new content
	content := []byte(strings.Join(newLines, "\n"))
	if _, err = file.Write(content); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Ensure data is written to disk
	if err = file.Sync(); err != nil {
		return fmt.Errorf("failed to sync config: %w", err)
	}

	return nil
}

func generateLuaMonitorConfig(monitors []Monitor) string {
	lines := []string{
		"-- Generated by HyprMon. Manual changes may be overwritten.",
		"",
	}
	for _, m := range monitors {
		lines = append(lines, generateLuaMonitorRule(m))
	}
	return strings.Join(lines, "\n") + "\n"
}

func hasHyprmonLuaRequire(input string) bool {
	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == hyprmonLuaRequireLine {
			return true
		}
	}
	return false
}

func ensureHyprmonLuaRequire(input string) string {
	if hasHyprmonLuaRequire(input) {
		return input
	}

	if strings.TrimSpace(input) == "" {
		return hyprmonLuaRequireComment + "\n" + hyprmonLuaRequireLine + "\n"
	}

	var b strings.Builder
	b.WriteString(input)
	if !strings.HasSuffix(input, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(hyprmonLuaRequireComment)
	b.WriteString("\n")
	b.WriteString(hyprmonLuaRequireLine)
	b.WriteString("\n")
	return b.String()
}

func writeLuaConfig(configPath string, monitors []Monitor) error {
	if configPath == "" {
		return fmt.Errorf("could not determine config path")
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, profileDirMode); err != nil {
		return fmt.Errorf("failed to ensure lua config directory: %w", err)
	}

	input, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read lua config: %w", err)
	}

	if err == nil {
		backupPath := fmt.Sprintf("%s.bak.%d", configPath, time.Now().Unix())
		if err := os.WriteFile(backupPath, input, backupFileMode); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	sidecarPath := filepath.Join(configDir, "hyprmon.lua")
	if err := os.WriteFile(sidecarPath, []byte(generateLuaMonitorConfig(monitors)), configFileMode); err != nil {
		return fmt.Errorf("failed to write lua monitor config: %w", err)
	}

	mainConfig := ensureHyprmonLuaRequire(string(input))
	if err := os.WriteFile(configPath, []byte(mainConfig), configFileMode); err != nil {
		return fmt.Errorf("failed to write lua config include: %w", err)
	}

	return nil
}

func reloadConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
	defer cancel()

	return exec.CommandContext(ctx, "hyprctl", "reload").Run()
}

var previousMonitors []Monitor

func saveRollback(monitors []Monitor) {
	previousMonitors = make([]Monitor, len(monitors))
	copy(previousMonitors, monitors)
}

func rollback() error {
	if previousMonitors == nil {
		return fmt.Errorf("no previous state to rollback to")
	}
	return applyMonitors(previousMonitors)
}

func readWorkspaces() ([]hyprWorkspace, error) {
	var workspaces []hyprWorkspace
	if err := execHyprctlJSON(&workspaces, "workspaces", "-j"); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func getCurrentMonitorNames() ([]string, error) {
	var hyprMonitors []hyprMonitor
	if err := execHyprctlJSON(&hyprMonitors, "monitors", "-j"); err != nil {
		return nil, err
	}

	var names []string
	for _, m := range hyprMonitors {
		if !m.Disabled {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

func migrateOrphanedWorkspaces(previousMonitorNames, currentMonitorNames []string) error {
	// Check if we're switching to a single monitor setup
	if len(currentMonitorNames) == 1 {
		// Move all workspaces to the single active monitor
		workspaces, err := readWorkspaces()
		if err != nil {
			return fmt.Errorf("failed to read workspaces: %w", err)
		}

		targetMonitor := currentMonitorNames[0]
		for _, workspace := range workspaces {
			// Move workspace if it's not already on the target monitor
			if workspace.Monitor != targetMonitor {
				cmd := fmt.Sprintf("hyprctl dispatch moveworkspacetomonitor %d %s", workspace.ID, targetMonitor)
				ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
				defer cancel()
				if err := exec.CommandContext(ctx, "sh", "-c", cmd).Run(); err != nil {
					return fmt.Errorf("failed to migrate workspace %d to monitor %s: %w", workspace.ID, targetMonitor, err)
				}
			}
		}
		return nil
	}

	// Original logic for multiple monitors - only migrate from removed monitors
	removedMonitors := findRemovedMonitors(previousMonitorNames, currentMonitorNames)
	if len(removedMonitors) == 0 {
		return nil
	}

	workspaces, err := readWorkspaces()
	if err != nil {
		return fmt.Errorf("failed to read workspaces: %w", err)
	}

	for _, workspace := range workspaces {
		for _, removedMonitor := range removedMonitors {
			if workspace.Monitor == removedMonitor {
				cmd := fmt.Sprintf("hyprctl dispatch moveworkspacetomonitor %d current", workspace.ID)
				ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
				defer cancel()
				if err := exec.CommandContext(ctx, "sh", "-c", cmd).Run(); err != nil {
					return fmt.Errorf("failed to migrate workspace %d: %w", workspace.ID, err)
				}
			}
		}
	}

	return nil
}

func findRemovedMonitors(previous, current []string) []string {
	currentSet := make(map[string]bool)
	for _, name := range current {
		currentSet[name] = true
	}

	var removed []string
	for _, name := range previous {
		if !currentSet[name] {
			removed = append(removed, name)
		}
	}
	return removed
}
