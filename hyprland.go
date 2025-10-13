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
)

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
			Name:     hm.Name,
			PxW:      uint32(hm.Width),
			PxH:      uint32(hm.Height),
			Hz:       float32(hm.RefreshRate),
			Scale:    float32(hm.Scale),
			X:        int32(hm.X),
			Y:        int32(hm.Y),
			Active:   !hm.Disabled,
			EDIDName: hm.Description,
			Modes:    modes,

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

func getConfigPath() string {
	if envPath := os.Getenv("HYPRLAND_CONFIG"); envPath != "" {
		// Validate that the path is absolute and clean to prevent path traversal
		cleanPath := filepath.Clean(envPath)
		if !filepath.IsAbs(cleanPath) {
			return ""
		}
		// Ensure the path doesn't contain directory traversal attempts
		if strings.Contains(envPath, "..") {
			return ""
		}
		return cleanPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "hypr", "hyprland.conf")
}

// generateMonitorLine creates the monitor configuration line for a monitor
func generateMonitorLine(m Monitor) string {
	// Validate monitor name (defensive check)
	if !isValidMonitorName(m.Name) {
		return fmt.Sprintf("# Invalid monitor name: %s", m.Name)
	}

	if !m.Active {
		return fmt.Sprintf("monitor=%s,disable", m.Name)
	}

	var monLine string
	if m.IsMirrored && m.MirrorSource != "" {
		// Validate mirror source name (defensive check)
		if !isValidMonitorName(m.MirrorSource) {
			return fmt.Sprintf("# Invalid mirror source: %s", m.MirrorSource)
		}
		// Mirror syntax: monitor=NAME,resolution,position,scale,mirror,SOURCE_MONITOR
		monLine = fmt.Sprintf("monitor=%s,%dx%d@%.2f,%dx%d,%.2f,mirror,%s",
			m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale, m.MirrorSource)
	} else {
		// Regular monitor configuration
		monLine = fmt.Sprintf("monitor=%s,%dx%d@%.2f,%dx%d,%.2f",
			m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale)

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

func writeConfig(monitors []Monitor) error {
	configPath := getConfigPath()
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
