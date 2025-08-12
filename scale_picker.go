package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type scalePickerModel struct {
	scales   []float32
	selected int
	current  float32
	monitor  string
	width    uint32
	height   uint32
}

var commonScales = []float32{
	0.50, 0.75, 0.90, 1.00, 1.10, 1.25, 1.33, 1.50, 1.66, 1.75, 2.00, 2.25, 2.50, 2.75, 3.00,
}

func newScalePicker(monitor string, currentScale float32, width, height uint32) scalePickerModel {
	selected := 3 // Default to 1.00
	for i, scale := range commonScales {
		if scale == currentScale {
			selected = i
			break
		}
	}

	return scalePickerModel{
		scales:   commonScales,
		selected: selected,
		current:  currentScale,
		monitor:  monitor,
		width:    width,
		height:   height,
	}
}

type scaleSelectedMsg struct {
	scale float32
}

type scaleCancelledMsg struct{}

func (m scalePickerModel) Init() tea.Cmd {
	return nil
}

func (m scalePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, func() tea.Msg { return scaleCancelledMsg{} }

		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if m.selected < len(m.scales)-1 {
				m.selected++
			}

		case "home", "g":
			m.selected = 0

		case "end", "G":
			m.selected = len(m.scales) - 1

		case "enter", " ":
			return m, func() tea.Msg {
				return scaleSelectedMsg{scale: m.scales[m.selected]}
			}

		case "1":
			// Quick select 1.00
			for i, scale := range m.scales {
				if scale == 1.00 {
					m.selected = i
					return m, func() tea.Msg {
						return scaleSelectedMsg{scale: scale}
					}
				}
			}

		case "2":
			// Quick select 2.00
			for i, scale := range m.scales {
				if scale == 2.00 {
					m.selected = i
					return m, func() tea.Msg {
						return scaleSelectedMsg{scale: scale}
					}
				}
			}
		}
	}

	return m, nil
}

func (m scalePickerModel) View() string {
	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	s.WriteString(titleStyle.Render(fmt.Sprintf("Select Scale for %s", m.monitor)))
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

	for i, scale := range m.scales {
		scaleStr := fmt.Sprintf("%.2fx", scale)
		
		// Add indicators for special scales
		indicator := ""
		if scale == 1.00 {
			indicator = " (native)"
		} else if scale == m.current {
			indicator = currentStyle.Render(" (current)")
		}
		
		// Add DPI information
		dpi := int(96 * scale)
		dpiInfo := fmt.Sprintf(" - %d DPI", dpi)
		
		// Add recommendations
		recommendation := ""
		if scale == 1.00 {
			recommendation = recommendedStyle.Render(" - No scaling")
		} else if scale == 1.25 {
			recommendation = recommendedStyle.Render(" - Good for 27\" 4K")
		} else if scale == 1.50 {
			recommendation = recommendedStyle.Render(" - Good for 24\" 4K")
		} else if scale == 2.00 {
			recommendation = recommendedStyle.Render(" - HiDPI/Retina")
		}

		line := fmt.Sprintf("%s%s%s%s", scaleStr, indicator, dpiInfo, recommendation)

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

	help := "↑/↓: Navigate  •  Enter: Select  •  1: 1.00x  •  2: 2.00x  •  Esc: Cancel"
	s.WriteString(helpStyle.Render(help))

	// Add preview of what the scale means
	s.WriteString("\n\n")
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)
	
	selectedScale := m.scales[m.selected]
	effectiveRes := fmt.Sprintf("Physical: %dx%d → Effective: %dx%d", 
		m.width, m.height,
		int(float32(m.width)/selectedScale), int(float32(m.height)/selectedScale))
	s.WriteString(previewStyle.Render(effectiveRes))

	return s.String()
}