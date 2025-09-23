package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// wouldCreateCircularMirror checks if setting currentMonitor to mirror sourceMonitor
// would create a circular mirroring chain
func wouldCreateCircularMirror(currentMonitor, sourceMonitor string, allMonitors []Monitor) bool {
	// Build a map of current mirror relationships
	mirrorMap := make(map[string]string)
	for _, mon := range allMonitors {
		if mon.IsMirrored && mon.MirrorSource != "" {
			mirrorMap[mon.Name] = mon.MirrorSource
		}
	}

	// Check if sourceMonitor is already mirroring currentMonitor (direct or indirect)
	visited := make(map[string]bool)
	current := sourceMonitor

	for {
		if visited[current] {
			// We've seen this monitor before - there's already a cycle
			return true
		}
		visited[current] = true

		// If the current monitor would mirror our target monitor, that's a cycle
		if current == currentMonitor {
			return true
		}

		// Follow the chain: what does the current monitor mirror?
		next, exists := mirrorMap[current]
		if !exists {
			// This monitor doesn't mirror anything, no cycle
			break
		}
		current = next
	}

	return false
}

// validateMirrorConfiguration checks for various mirror configuration issues
func validateMirrorConfiguration(monitors []Monitor) []string {
	var warnings []string

	// Check for resolution mismatches
	for _, mon := range monitors {
		if mon.IsMirrored && mon.MirrorSource != "" {
			for _, source := range monitors {
				if source.Name == mon.MirrorSource {
					if mon.PxW != source.PxW || mon.PxH != source.PxH {
						warnings = append(warnings, fmt.Sprintf(
							"Resolution mismatch: %s (%dx%d) mirroring %s (%dx%d)",
							mon.Name, mon.PxW, mon.PxH,
							source.Name, source.PxW, source.PxH))
					}
					break
				}
			}
		}
	}

	// Check for disabled source monitors
	for _, mon := range monitors {
		if mon.IsMirrored && mon.MirrorSource != "" {
			for _, source := range monitors {
				if source.Name == mon.MirrorSource && !source.Active {
					warnings = append(warnings, fmt.Sprintf(
						"Mirror source %s is disabled but %s is trying to mirror it",
						source.Name, mon.Name))
					break
				}
			}
		}
	}

	return warnings
}

type mirrorPickerModel struct {
	availableMonitors []string // List of monitors that can be mirrored
	selected          int      // Currently selected monitor index
	currentMonitor    string   // Monitor being configured
	currentSource     string   // Current mirror source (empty if not mirrored)
}

func newMirrorPicker(currentMonitor string, currentSource string, allMonitors []Monitor) mirrorPickerModel {
	var availableMonitors []string

	// Add "None" option to disable mirroring
	availableMonitors = append(availableMonitors, "None")

	// Add all other active monitors as potential sources
	for _, mon := range allMonitors {
		if mon.Name != currentMonitor && mon.Active && !mon.IsMirrored {
			// Additional validation: prevent circular mirroring
			if !wouldCreateCircularMirror(currentMonitor, mon.Name, allMonitors) {
				availableMonitors = append(availableMonitors, mon.Name)
			}
		}
	}

	// Find current selection
	selected := 0 // Default to "None"
	if currentSource != "" {
		for i, name := range availableMonitors {
			if name == currentSource {
				selected = i
				break
			}
		}
	}

	return mirrorPickerModel{
		availableMonitors: availableMonitors,
		selected:          selected,
		currentMonitor:    currentMonitor,
		currentSource:     currentSource,
	}
}

type mirrorSelectedMsg struct {
	source string // Empty string means disable mirroring
}

type mirrorCancelledMsg struct{}

func (m mirrorPickerModel) Init() tea.Cmd {
	return nil
}

func (m mirrorPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.availableMonitors)-1 {
				m.selected++
			}
		case "enter":
			source := ""
			if m.selected > 0 { // Skip "None" option
				source = m.availableMonitors[m.selected]
			}
			return m, func() tea.Msg {
				return mirrorSelectedMsg{source: source}
			}
		case "esc", "q":
			return m, func() tea.Msg {
				return mirrorCancelledMsg{}
			}
		}
	}

	return m, nil
}

func (m mirrorPickerModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("Mirror Configuration for %s", m.currentMonitor)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(title))
	b.WriteString("\n\n")

	if len(m.availableMonitors) == 1 {
		b.WriteString("No monitors available for mirroring.\n")
		b.WriteString("(Only active, non-mirrored monitors can be mirror sources)\n\n")

		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		b.WriteString(warningStyle.Render("Note: Circular mirroring is not allowed"))
		b.WriteString("\n\n")

		b.WriteString("Press ESC to cancel")
		return b.String()
	}

	b.WriteString("Select a monitor to mirror from:\n\n")

	for i, monitor := range m.availableMonitors {
		prefix := "  "
		suffix := ""

		if i == m.selected {
			prefix = "> "
			suffix = " <"
		}

		style := lipgloss.NewStyle()
		if i == m.selected {
			style = style.Bold(true).Foreground(lipgloss.Color("214"))
		} else if monitor == "None" {
			style = style.Foreground(lipgloss.Color("244"))
		} else {
			style = style.Foreground(lipgloss.Color("42"))
		}

		line := prefix + monitor + suffix
		switch monitor {
		case "None":
			line += " (Disable mirroring)"
		case m.currentSource:
			line += " (current)"
		}

		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Add warning about resolution mismatches if applicable
	if len(m.availableMonitors) > 1 {
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		b.WriteString(warningStyle.Render("⚠  Mirroring monitors with different resolutions may cause stretching"))
		b.WriteString("\n")
	}

	b.WriteString("↑/↓: Navigate  Enter: Select  ESC: Cancel")

	return b.String()
}
