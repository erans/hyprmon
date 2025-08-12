package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	monitorBoxActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("42")).
				Foreground(lipgloss.Color("42"))

	monitorBoxInactive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("244")).
				Foreground(lipgloss.Color("244"))

	monitorBoxSelected = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("214")).
				Foreground(lipgloss.Color("214"))

	desktopStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
)

func (m model) View() string {
	// Show help if active
	if m.ShowHelp {
		return m.renderHelp()
	}

	// Show profile input if active
	if m.ShowProfileInput {
		return m.ProfileInput.View()
	}

	// Show scale picker if active
	if m.ShowScalePicker {
		return m.ScalePicker.View()
	}

	// Allow rendering even with default sizes
	if m.World.TermW <= 0 {
		m.World.TermW = 80
	}
	if m.World.TermH <= 0 {
		m.World.TermH = 24
	}

	var b strings.Builder

	header := m.renderHeader()
	desktop := m.renderDesktop()
	details := m.renderDetails()
	footer := m.renderFooter()

	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(desktop)
	b.WriteString("\n")
	b.WriteString(details)
	b.WriteString("\n")
	b.WriteString(footer)

	return b.String()
}

func (m model) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Padding(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42")).
		MarginTop(1).
		MarginBottom(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(20)

	var content strings.Builder

	// Title and version
	content.WriteString(titleStyle.Render(fmt.Sprintf("HyprMon %s", ShortVersion())))
	content.WriteString("\n")
	content.WriteString("Copyright © 2025 Eran Sandler\n\n")
	content.WriteString("A visual monitor configuration tool for Hyprland window manager.\n")

	// Keyboard shortcuts
	content.WriteString(sectionStyle.Render("\nKeyboard Shortcuts:"))
	content.WriteString("\n")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"↑↓←→", "Move selected monitor"},
		{"Shift+↑↓←→", "Move by 10x step"},
		{"Tab / Shift+Tab", "Select next/previous monitor"},
		{"Enter / Space", "Toggle monitor on/off"},
		{"G", "Cycle grid size (1, 8, 16, 32, 64 px)"},
		{"L", "Cycle snap mode (Off, Edges, Centers, Both)"},
		{"R", "Open scale adjustment dialog"},
		{"A", "Apply changes to Hyprland"},
		{"S", "Save configuration to file"},
		{"O", "Open profiles page"},
		{"P", "Save as profile"},
		{"Z", "Revert to previous configuration"},
		{"?", "Show this help"},
		{"Q / Ctrl+C", "Quit"},
	}

	for _, s := range shortcuts {
		content.WriteString(fmt.Sprintf("%s %s\n",
			keyStyle.Render(s.key), s.desc))
	}

	// Mouse controls
	content.WriteString(sectionStyle.Render("\nMouse Controls:"))
	content.WriteString("\n")

	mouseControls := []struct {
		action string
		desc   string
	}{
		{"Left Click", "Select monitor"},
		{"Drag", "Move selected monitor"},
		{"Right Click", "Toggle monitor on/off"},
		{"Scroll Wheel", "Adjust scale"},
	}

	for _, m := range mouseControls {
		content.WriteString(fmt.Sprintf("%s %s\n",
			keyStyle.Render(m.action), m.desc))
	}

	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press any key to close help"))

	return helpStyle.Render(content.String())
}

func (m model) renderHeader() string {
	legend := "[ON] Active  [OFF] Inactive"
	grid := fmt.Sprintf("Grid: %d px", m.GridPx)
	snapNames := []string{"Off", "Edges", "Centers", "Both"}
	snap := fmt.Sprintf("Snap: %s", snapNames[m.Snap])

	// Add version if not "dev"
	header := fmt.Sprintf("Legend: %s   %s   %s", legend, grid, snap)
	if Version != "dev" {
		header = fmt.Sprintf("HyprMon %s  |  %s", ShortVersion(), header)
	}

	return headerStyle.Render(header)
}

func (m model) renderDesktop() string {
	width := m.World.TermW
	height := m.World.TermH - 8

	// Ensure minimum dimensions
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}

	desktop := make([][]rune, height)
	for i := range desktop {
		desktop[i] = make([]rune, width)
		for j := range desktop[i] {
			desktop[i][j] = ' '
		}
	}

	for _, guide := range m.Guides {
		m.renderGuide(desktop, guide)
	}

	for i, mon := range m.Monitors {
		m.renderMonitor(desktop, mon, i == m.Selected)
	}

	var lines []string
	for _, row := range desktop {
		lines = append(lines, string(row))
	}

	content := strings.Join(lines, "\n")
	return desktopStyle.Width(width).Height(height).Render(content)
}

func (m model) renderMonitor(desktop [][]rune, mon Monitor, selected bool) {
	tx1, ty1 := m.worldToTerm(mon.X, mon.Y)
	tx2, ty2 := m.worldToTerm(mon.X+int32(mon.PxW), mon.Y+int32(mon.PxH))

	if tx1 < 0 {
		tx1 = 0
	}
	if ty1 < 0 {
		ty1 = 0
	}
	if tx2 >= len(desktop[0]) {
		tx2 = len(desktop[0]) - 1
	}
	if ty2 >= len(desktop) {
		ty2 = len(desktop) - 1
	}

	if tx2-tx1 < 3 {
		tx2 = tx1 + 3
	}
	if ty2-ty1 < 2 {
		ty2 = ty1 + 2
	}

	var style lipgloss.Style
	if selected {
		style = monitorBoxSelected
	} else if mon.Active {
		style = monitorBoxActive
	} else {
		style = monitorBoxInactive
	}

	boxRunes := m.getBoxRunes(style)

	// Fill background with dots for inactive monitors
	if !mon.Active {
		for y := ty1 + 1; y < ty2 && y < len(desktop); y++ {
			for x := tx1 + 1; x < tx2 && x < len(desktop[0]); x++ {
				desktop[y][x] = '·'
			}
		}
	}

	for y := ty1; y <= ty2 && y < len(desktop); y++ {
		for x := tx1; x <= tx2 && x < len(desktop[0]); x++ {
			if y == ty1 {
				if x == tx1 {
					desktop[y][x] = boxRunes.topLeft
				} else if x == tx2 {
					desktop[y][x] = boxRunes.topRight
				} else {
					desktop[y][x] = boxRunes.horizontal
				}
			} else if y == ty2 {
				if x == tx1 {
					desktop[y][x] = boxRunes.bottomLeft
				} else if x == tx2 {
					desktop[y][x] = boxRunes.bottomRight
				} else {
					desktop[y][x] = boxRunes.horizontal
				}
			} else {
				if x == tx1 || x == tx2 {
					desktop[y][x] = boxRunes.vertical
				}
			}
		}
	}

	// Add monitor name with [ON]/[OFF] status
	statusLabel := "[ON]"
	if !mon.Active {
		statusLabel = "[OFF]"
	}
	nameLabel := fmt.Sprintf("%s %s", mon.Name, statusLabel)
	if len(nameLabel) > tx2-tx1-2 {
		nameLabel = nameLabel[:tx2-tx1-2]
	}
	if ty1+1 < len(desktop) && tx1+1 < len(desktop[0]) {
		for i, r := range nameLabel {
			if tx1+1+i < tx2 {
				desktop[ty1+1][tx1+1+i] = r
			}
		}
	}

	// Only show details for active monitors (dimmed effect for inactive)
	if mon.Active {
		resLabel := fmt.Sprintf("%dx%d@%.0fHz", mon.PxW, mon.PxH, mon.Hz)
		if len(resLabel) > tx2-tx1-2 {
			resLabel = resLabel[:tx2-tx1-2]
		}
		if ty1+2 < len(desktop) && ty1+2 < ty2 && tx1+1 < len(desktop[0]) {
			for i, r := range resLabel {
				if tx1+1+i < tx2 {
					desktop[ty1+2][tx1+1+i] = r
				}
			}
		}

		scaleLabel := fmt.Sprintf("x%.2f", mon.Scale)
		if len(scaleLabel) > tx2-tx1-2 {
			scaleLabel = scaleLabel[:tx2-tx1-2]
		}
		if ty1+3 < len(desktop) && ty1+3 < ty2 && tx1+1 < len(desktop[0]) {
			for i, r := range scaleLabel {
				if tx1+1+i < tx2 {
					desktop[ty1+3][tx1+1+i] = r
				}
			}
		}
	}
}

func (m model) renderGuide(desktop [][]rune, guide guide) {
	if guide.Type == "vertical" {
		x, _ := m.worldToTerm(guide.Value, 0)
		if x >= 0 && x < len(desktop[0]) {
			for y := 0; y < len(desktop); y++ {
				desktop[y][x] = '│'
			}
		}
	} else if guide.Type == "horizontal" {
		_, y := m.worldToTerm(0, guide.Value)
		if y >= 0 && y < len(desktop) {
			for x := 0; x < len(desktop[0]); x++ {
				desktop[y][x] = '─'
			}
		}
	}
}

func (m model) renderDetails() string {
	if m.Selected < 0 || m.Selected >= len(m.Monitors) {
		return "No monitor selected"
	}

	mon := m.Monitors[m.Selected]
	details := fmt.Sprintf("Details: %s  pos %d,%d  size %dx%d @%.0fHz  scale %.2f",
		mon.Name, mon.X, mon.Y, mon.PxW, mon.PxH, mon.Hz, mon.Scale)

	if m.Status != "" {
		details += "  |  " + statusStyle.Render(m.Status)
	}

	return details
}

func (m model) renderFooter() string {
	keys := []string{
		"↑↓←→ move",
		"Shift+↑↓←→ step×10",
		"Tab select",
		"Enter toggle",
		"G grid",
		"L snap",
		"R scale",
		"A apply",
		"S save",
		"O profiles",
		"P save profile",
		"Z revert",
		"? help",
		"Q quit",
	}

	if m.World.TermW > 100 {
		keys = append(keys, "Drag to move", "Right-click toggle", "Wheel to scale")
	}

	return footerStyle.Render(strings.Join(keys, "  •  "))
}

type boxRunes struct {
	topLeft     rune
	topRight    rune
	bottomLeft  rune
	bottomRight rune
	horizontal  rune
	vertical    rune
}

func (m model) getBoxRunes(style lipgloss.Style) boxRunes {
	borderStyle := style.GetBorderStyle()

	return boxRunes{
		topLeft:     []rune(borderStyle.TopLeft)[0],
		topRight:    []rune(borderStyle.TopRight)[0],
		bottomLeft:  []rune(borderStyle.BottomLeft)[0],
		bottomRight: []rune(borderStyle.BottomRight)[0],
		horizontal:  []rune(borderStyle.Top)[0],
		vertical:    []rune(borderStyle.Left)[0],
	}
}
