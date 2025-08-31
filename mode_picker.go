package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type modePickerModel struct {
	modes    []DisplayMode
	selected int
	current  DisplayMode
	monitor  string
}

type DisplayMode struct {
	Width       uint32
	Height      uint32
	RefreshRate float32
	ModeString  string // Original string like "1920x1080@60.00Hz"
}

func newModePicker(monitor string, currentWidth, currentHeight uint32, currentHz float32, availableModes []string) modePickerModel {
	modes := parseDisplayModes(availableModes)

	// Find current mode
	currentMode := DisplayMode{
		Width:       currentWidth,
		Height:      currentHeight,
		RefreshRate: currentHz,
		ModeString:  fmt.Sprintf("%dx%d@%.2fHz", currentWidth, currentHeight, currentHz),
	}

	selected := 0
	for i, mode := range modes {
		if mode.Width == currentWidth && mode.Height == currentHeight &&
			abs32(mode.RefreshRate-currentHz) < 0.1 {
			selected = i
			break
		}
	}

	return modePickerModel{
		modes:    modes,
		selected: selected,
		current:  currentMode,
		monitor:  monitor,
	}
}

func parseDisplayModes(modeStrings []string) []DisplayMode {
	var modes []DisplayMode
	modeRegex := regexp.MustCompile(`(\d+)x(\d+)@([\d.]+)Hz`)

	for _, modeStr := range modeStrings {
		matches := modeRegex.FindStringSubmatch(modeStr)
		if len(matches) == 4 {
			width, _ := strconv.ParseUint(matches[1], 10, 32)
			height, _ := strconv.ParseUint(matches[2], 10, 32)
			refreshRate, _ := strconv.ParseFloat(matches[3], 32)

			modes = append(modes, DisplayMode{
				Width:       uint32(width),
				Height:      uint32(height),
				RefreshRate: float32(refreshRate),
				ModeString:  modeStr,
			})
		}
	}

	// Sort by resolution (width * height) descending, then by refresh rate descending
	sort.Slice(modes, func(i, j int) bool {
		resI := int(modes[i].Width) * int(modes[i].Height)
		resJ := int(modes[j].Width) * int(modes[j].Height)
		if resI != resJ {
			return resI > resJ
		}
		return modes[i].RefreshRate > modes[j].RefreshRate
	})

	return modes
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

type modeSelectedMsg struct {
	mode DisplayMode
}

type modeCancelledMsg struct{}

func (m modePickerModel) Init() tea.Cmd {
	return nil
}

func (m modePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, func() tea.Msg { return modeCancelledMsg{} }

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.modes)-1 {
				m.selected++
			}

		case "home", "g":
			m.selected = 0

		case "end", "G":
			m.selected = len(m.modes) - 1

		case "enter", " ":
			return m, func() tea.Msg {
				return modeSelectedMsg{mode: m.modes[m.selected]}
			}

		// Quick select common resolutions
		case "1":
			// Quick select 1080p (highest refresh rate)
			for i, mode := range m.modes {
				if mode.Width == 1920 && mode.Height == 1080 {
					m.selected = i
					return m, func() tea.Msg {
						return modeSelectedMsg{mode: mode}
					}
				}
			}

		case "4":
			// Quick select 4K (highest refresh rate)
			for i, mode := range m.modes {
				if mode.Width == 3840 && mode.Height == 2160 {
					m.selected = i
					return m, func() tea.Msg {
						return modeSelectedMsg{mode: mode}
					}
				}
			}
		}
	}

	return m, nil
}

func (m modePickerModel) View() string {
	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	s.WriteString(titleStyle.Render(fmt.Sprintf("Select Resolution & Refresh Rate for %s", m.monitor)))
	s.WriteString("\n\n")

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(lipgloss.Color("214")).
		Bold(true)

	currentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	recommendedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Italic(true)

	for i, mode := range m.modes {
		// Format: "1920x1080@144.00Hz"
		modeStr := fmt.Sprintf("%dx%d@%.2fHz", mode.Width, mode.Height, mode.RefreshRate)

		// Add indicators
		indicator := ""
		if mode.Width == m.current.Width && mode.Height == m.current.Height &&
			abs32(mode.RefreshRate-m.current.RefreshRate) < 0.1 {
			indicator = currentStyle.Render(" (current)")
		}

		// Add recommendations based on resolution
		recommendation := ""
		switch {
		case mode.Width == 1920 && mode.Height == 1080:
			if mode.RefreshRate >= 144 {
				recommendation = recommendedStyle.Render(" - Full HD Gaming")
			} else if mode.RefreshRate >= 60 {
				recommendation = recommendedStyle.Render(" - Full HD")
			}
		case mode.Width == 2560 && mode.Height == 1440:
			if mode.RefreshRate >= 144 {
				recommendation = recommendedStyle.Render(" - 1440p Gaming")
			} else {
				recommendation = recommendedStyle.Render(" - 1440p")
			}
		case mode.Width == 3840 && mode.Height == 2160:
			if mode.RefreshRate >= 60 {
				recommendation = recommendedStyle.Render(" - 4K UHD")
			}
		case mode.Width == 3440 && mode.Height == 1440:
			recommendation = recommendedStyle.Render(" - Ultrawide")
		}

		line := fmt.Sprintf("%s%s%s", modeStr, indicator, recommendation)

		if i == m.selected {
			s.WriteString(selectedStyle.Render("▶ " + line))
		} else {
			s.WriteString(itemStyle.Render("  " + line))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	help := "↑/↓: Navigate  •  Enter: Select  •  1: 1080p  •  4: 4K  •  Esc: Cancel"
	s.WriteString(helpStyle.Render(help))

	// Add preview of selected mode
	s.WriteString("\n\n")
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	selectedMode := m.modes[m.selected]
	aspectRatio := float32(selectedMode.Width) / float32(selectedMode.Height)
	aspectStr := fmt.Sprintf("%.2f:1", aspectRatio)
	if aspectRatio >= 2.35 {
		aspectStr = "Ultrawide (21:9)"
	} else if aspectRatio >= 1.7 {
		aspectStr = "Widescreen (16:9/16:10)"
	} else if aspectRatio >= 1.6 {
		aspectStr = "Standard (16:10)"
	} else if aspectRatio >= 1.3 {
		aspectStr = "Classic (4:3)"
	}

	preview := fmt.Sprintf("Resolution: %dx%d • Aspect: %s • Refresh: %.2fHz",
		selectedMode.Width, selectedMode.Height, aspectStr, selectedMode.RefreshRate)
	s.WriteString(previewStyle.Render(preview))

	return s.String()
}
