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
	// Calculate available viewport dimensions - leave margin to prevent cutoff
	viewportHeight := m.World.TermH - 6 // Leave space for margins and prevent cutoff
	viewportWidth := m.World.TermW - 10 // Account for border and padding

	// Ensure minimum size
	if viewportHeight < 10 {
		viewportHeight = 10
	}
	if viewportWidth < 40 {
		viewportWidth = 40
	}

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

	// Build all content lines
	var allLines []string

	// Title and version
	allLines = append(allLines, titleStyle.Render(fmt.Sprintf("HyprMon %s", ShortVersion())))
	allLines = append(allLines, "Copyright © 2025 Eran Sandler")
	allLines = append(allLines, "")
	allLines = append(allLines, "A visual monitor configuration tool for Hyprland window manager.")
	allLines = append(allLines, "")

	// Keyboard shortcuts
	allLines = append(allLines, sectionStyle.Render("Keyboard Shortcuts:"))

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
		allLines = append(allLines, fmt.Sprintf("%s %s",
			keyStyle.Render(s.key), s.desc))
	}

	// Mouse controls
	allLines = append(allLines, "")
	allLines = append(allLines, sectionStyle.Render("Mouse Controls:"))

	mouseControls := []struct {
		action string
		desc   string
	}{
		{"Left Click", "Select monitor"},
		{"Drag", "Move selected monitor"},
		{"Right Click", "Toggle monitor on/off"},
		{"Scroll Wheel", "Adjust scale"},
	}

	for _, mc := range mouseControls {
		allLines = append(allLines, fmt.Sprintf("%s %s",
			keyStyle.Render(mc.action), mc.desc))
	}

	// Navigation help
	allLines = append(allLines, "")
	allLines = append(allLines, sectionStyle.Render("Navigation (in this help):"))
	allLines = append(allLines, fmt.Sprintf("%s %s", keyStyle.Render("↑/↓"), "Scroll up/down"))
	allLines = append(allLines, fmt.Sprintf("%s %s", keyStyle.Render("PgUp/PgDn"), "Page up/down"))
	allLines = append(allLines, fmt.Sprintf("%s %s", keyStyle.Render("Home/End"), "Jump to top/bottom"))
	allLines = append(allLines, fmt.Sprintf("%s %s", keyStyle.Render("ESC/q"), "Close help"))

	// Calculate visible content based on scroll offset
	totalLines := len(allLines)
	contentHeight := viewportHeight - 5 // Reserve space for header/footer
	maxScroll := totalLines - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Ensure scroll offset is within bounds
	scrollOffset := m.HelpScrollOffset
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	// Build content without scroll bar
	visibleLines := []string{}

	// Get visible content lines
	for i := scrollOffset; i < len(allLines) && i < scrollOffset+contentHeight; i++ {
		visibleLines = append(visibleLines, allLines[i])
	}

	// Pad to fill viewport
	for len(visibleLines) < contentHeight {
		visibleLines = append(visibleLines, "")
	}

	// Add footer with navigation info
	visibleLines = append(visibleLines, strings.Repeat("─", min(viewportWidth-6, 70)))

	if totalLines > contentHeight {
		// Show scroll info and navigation instructions
		footerText := fmt.Sprintf("Lines %d-%d of %d • Use ↑↓ or PgUp/PgDn to scroll • ESC to close",
			scrollOffset+1,
			min(scrollOffset+contentHeight, totalLines),
			totalLines)
		visibleLines = append(visibleLines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(footerText))
	} else {
		// Just show close instruction if no scrolling needed
		visibleLines = append(visibleLines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("ESC or q to close"))
	}

	// Build final content
	content := strings.Join(visibleLines, "\n")

	// Apply help box styling with less padding
	helpStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Width(viewportWidth).
		Height(viewportHeight).
		MarginTop(1) // Add top margin to prevent cutoff

	return helpStyle.Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) renderHeader() string {
	grid := fmt.Sprintf("Grid: %d px", m.GridPx)
	snapNames := []string{"Off", "Edges", "Centers", "Both"}
	snap := fmt.Sprintf("Snap: %s", snapNames[m.Snap])

	// Add version if not "dev"
	header := fmt.Sprintf("%s   %s", grid, snap)
	if Version != "dev" {
		header = fmt.Sprintf("HyprMon %s  |  %s", ShortVersion(), header)
	}

	return headerStyle.Render(header)
}

func (m model) renderDesktop() string {
	// Content width: terminal width minus border (2) and potential margin (1)
	width := m.World.TermW - 3
	// Calculate available height: total - header(2) - details(1) - footer(up to 3) - margins(3)
	// Be conservative and reserve space for 3-line footer
	height := m.World.TermH - 10

	// Ensure minimum dimensions
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}

	// Create the internal content area (this is what goes inside the border)
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
	// Don't set explicit width - let lipgloss calculate based on content + border
	return desktopStyle.Render(content)
}

func (m model) renderMonitor(desktop [][]rune, mon Monitor, selected bool) {
	// Use scaled dimensions for rendering
	scaledWidth := int32(float32(mon.PxW) / mon.Scale)
	scaledHeight := int32(float32(mon.PxH) / mon.Scale)

	tx1, ty1 := m.worldToTerm(mon.X, mon.Y)
	tx2, ty2 := m.worldToTerm(mon.X+scaledWidth, mon.Y+scaledHeight)

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

// keyCommand represents a keyboard/mouse command with different verbosity levels
type keyCommand struct {
	full     string
	medium   string
	short    string
	priority int // 1 = essential, 2 = important, 3 = nice to have
}

func (m model) renderFooter() string {
	commands := []keyCommand{
		{"↑↓←→ move", "↑↓←→ move", "↑↓←→", 1},
		{"Shift+↑↓←→ step×10", "Shift+↑↓←→ ×10", "S+↑↓←→", 2},
		{"Tab select", "Tab sel", "Tab", 2},
		{"Enter toggle", "Enter on/off", "⏎", 2},
		{"G grid", "G grid", "G", 2},
		{"L snap", "L snap", "L", 2},
		{"R scale", "R scale", "R", 1},
		{"A apply", "A apply", "A", 2},
		{"S save", "S save", "S", 2},
		{"O profiles", "O prof", "O", 3},
		{"P save profile", "P save prof", "P", 3},
		{"Z revert", "Z undo", "Z", 2},
		{"? help", "? help", "? Help", 1},
		{"Q quit", "Q quit", "Q", 1},
	}

	// Determine format based on terminal width
	var keys []string
	separator := "  •  "

	width := m.World.TermW

	if width < 60 {
		// Very narrow: only essential commands, shortest form
		separator = " "
		for _, cmd := range commands {
			if cmd.priority == 1 {
				keys = append(keys, cmd.short)
			}
		}
	} else if width < 80 {
		// Narrow: essential and important, short form
		separator = " • "
		for _, cmd := range commands {
			if cmd.priority <= 2 {
				keys = append(keys, cmd.short)
			}
		}
	} else if width < 100 {
		// Medium: all keyboard commands, medium form
		separator = " • "
		for _, cmd := range commands {
			keys = append(keys, cmd.medium)
		}
	} else {
		// Wide: all keyboard commands, full form
		for _, cmd := range commands {
			keys = append(keys, cmd.full)
		}
	}

	// Always try multi-line layout first, up to 3 lines
	return m.renderMultiLineFooter(commands, width, keys, separator)
}

func (m model) renderMultiLineFooter(commands []keyCommand, width int, singleLineKeys []string, separator string) string {
	var lines []string
	var currentLine []string
	var currentLength int

	sepLen := len(separator)

	// Helper function to add text to current line or start new line
	addToLine := func(text string) {
		textLen := len(text)
		wouldOverflow := len(currentLine) > 0 && currentLength+sepLen+textLen > width-4

		if wouldOverflow && len(lines) < 3 {
			// Start new line if we haven't reached 3 lines yet
			lines = append(lines, strings.Join(currentLine, separator))
			currentLine = []string{text}
			currentLength = textLen
		} else if !wouldOverflow {
			// Add to current line
			currentLine = append(currentLine, text)
			if len(currentLine) > 1 {
				currentLength += sepLen
			}
			currentLength += textLen
		}
		// If we would overflow and already have 3 lines, skip this item
	}

	// First, try with the keys we already have (based on width)
	for _, key := range singleLineKeys {
		addToLine(key)
	}

	// Add remaining line if exists
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, separator))
	}

	// If we're using more than 3 lines, we need to be more selective
	if len(lines) > 3 {
		lines = []string{}
		currentLine = []string{}
		currentLength = 0

		// Use progressively shorter forms until it fits in 3 lines
		attempts := []struct {
			priority int
			format   string
		}{
			{3, "medium"}, // All commands, medium form
			{3, "short"},  // All commands, short form
			{2, "short"},  // Important and essential only, short form
			{1, "short"},  // Essential only, short form
		}

		for _, attempt := range attempts {
			lines = []string{}
			currentLine = []string{}
			currentLength = 0

			for _, cmd := range commands {
				if cmd.priority <= attempt.priority {
					var text string
					switch attempt.format {
					case "full":
						text = cmd.full
					case "medium":
						text = cmd.medium
					case "short":
						text = cmd.short
					}
					addToLine(text)
				}
			}

			if len(currentLine) > 0 {
				lines = append(lines, strings.Join(currentLine, separator))
			}

			// If it fits in 3 lines, we're done
			if len(lines) <= 3 {
				break
			}
		}
	}

	return footerStyle.Render(strings.Join(lines, "\n"))
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
