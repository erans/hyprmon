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

func readMonitors() ([]Monitor, error) {
	cmd := exec.Command("hyprctl", "monitors", "-j")
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

	inactiveMonitors := readInactiveMonitors()
	monitors = append(monitors, inactiveMonitors...)

	return monitors, nil
}

func readInactiveMonitors() []Monitor {
	cmd := exec.Command("wlr-randr")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var monitors []Monitor
	lines := strings.Split(string(output), "\n")
	var currentMonitor *Monitor

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "(disabled)") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				if currentMonitor != nil {
					monitors = append(monitors, *currentMonitor)
				}
				currentMonitor = &Monitor{
					Name:   parts[0],
					Active: false,
					Scale:  1.0,
				}
			}
		} else if currentMonitor != nil && strings.Contains(line, "x") && strings.Contains(line, "@") {
			if mode := parseWlrMode(line); mode != nil {
				currentMonitor.Modes = append(currentMonitor.Modes, *mode)
				if strings.Contains(line, "current") && currentMonitor.PxW == 0 {
					currentMonitor.PxW = mode.W
					currentMonitor.PxH = mode.H
					currentMonitor.Hz = mode.Hz
				}
			}
		}
	}

	if currentMonitor != nil {
		monitors = append(monitors, *currentMonitor)
	}

	return monitors
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

func parseWlrMode(line string) *Mode {
	parts := strings.Fields(line)
	for _, part := range parts {
		if strings.Contains(part, "x") && !strings.Contains(part, "px") {
			resParts := strings.Split(part, "x")
			if len(resParts) == 2 {
				w, errW := strconv.ParseUint(resParts[0], 10, 32)
				h, errH := strconv.ParseUint(resParts[1], 10, 32)
				if errW == nil && errH == nil {
					hz := float32(60.0)
					for _, p := range parts {
						if strings.HasSuffix(p, "Hz") {
							hzStr := strings.TrimSuffix(p, "Hz")
							if hzVal, err := strconv.ParseFloat(hzStr, 32); err == nil {
								hz = float32(hzVal)
								break
							}
						}
					}
					return &Mode{
						W:  uint32(w),
						H:  uint32(h),
						Hz: hz,
					}
				}
			}
		}
	}
	return nil
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
