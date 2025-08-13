package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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
	cmd := exec.Command("hyprctl", "monitors", "all", "-j")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute hyprctl: %w", err)
	}

	var hyprMonitors []hyprMonitor
	if err := json.Unmarshal(output, &hyprMonitors); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	monitors := make([]Monitor, 0, len(hyprMonitors))
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
		}
		monitors = append(monitors, monitor)
	}

	return monitors, nil
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
	var cmd string
	if m.Active {
		cmd = fmt.Sprintf("hyprctl keyword monitor \"%s,%dx%d@%.2f,%dx%d,%.2f\"",
			m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale)
	} else {
		cmd = fmt.Sprintf("hyprctl keyword monitor \"%s,disable\"", m.Name)
	}

	return exec.Command("sh", "-c", cmd).Run()
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
		return envPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "hypr", "hyprland.conf")
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

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
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
					var monLine string
					if m.Active {
						monLine = fmt.Sprintf("monitor=%s,%dx%d@%.2f,%dx%d,%.2f",
							m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale)
					} else {
						monLine = fmt.Sprintf("monitor=%s,disable", m.Name)
					}
					newLines = append(newLines, monLine)
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
			var monLine string
			if m.Active {
				monLine = fmt.Sprintf("monitor=%s,%dx%d@%.2f,%dx%d,%.2f",
					m.Name, m.PxW, m.PxH, m.Hz, m.X, m.Y, m.Scale)
			} else {
				monLine = fmt.Sprintf("monitor=%s,disable", m.Name)
			}
			newLines = append(newLines, monLine)
		}
	}

	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		return fmt.Errorf("failed to replace config: %w", err)
	}

	return nil
}

func reloadConfig() error {
	return exec.Command("hyprctl", "reload").Run()
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
	cmd := exec.Command("hyprctl", "workspaces", "-j")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute hyprctl workspaces: %w", err)
	}

	var workspaces []hyprWorkspace
	if err := json.Unmarshal(output, &workspaces); err != nil {
		return nil, fmt.Errorf("failed to parse workspaces JSON: %w", err)
	}

	return workspaces, nil
}

func getCurrentMonitorNames() ([]string, error) {
	cmd := exec.Command("hyprctl", "monitors", "-j")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute hyprctl monitors: %w", err)
	}

	var hyprMonitors []hyprMonitor
	if err := json.Unmarshal(output, &hyprMonitors); err != nil {
		return nil, fmt.Errorf("failed to parse monitors JSON: %w", err)
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
				if err := exec.Command("sh", "-c", cmd).Run(); err != nil {
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
