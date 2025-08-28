package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type advancedSettingsModel struct {
	monitor      *Monitor
	focusedField int
	width        int
	height       int
}

const (
	fieldBitDepth = iota
	fieldColorMode
	fieldSDRBrightness
	fieldSDRSaturation
	fieldVRR
	fieldTransform
	fieldCount
)

func newAdvancedSettingsModel(monitor *Monitor) advancedSettingsModel {
	return advancedSettingsModel{
		monitor:      monitor,
		focusedField: 0,
	}
}

func (m advancedSettingsModel) Init() tea.Cmd {
	return nil
}

func (m advancedSettingsModel) Update(msg tea.Msg) (advancedSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "shift+tab":
			m.navigateUp()

		case "down", "tab":
			m.navigateDown()

		case "left":
			m.adjustValue(-1)

		case "right":
			m.adjustValue(1)

		case " ", "space":
			m.toggleValue()
		}
	}

	return m, nil
}

func (m *advancedSettingsModel) navigateDown() {
	isHDR := strings.Contains(m.monitor.ColorMode, "hdr")

	m.focusedField++

	// Skip SDR fields if not in HDR mode
	if !isHDR && (m.focusedField == fieldSDRBrightness || m.focusedField == fieldSDRSaturation) {
		m.focusedField = fieldVRR
	}

	if m.focusedField >= fieldCount {
		m.focusedField = 0
	}
}

func (m *advancedSettingsModel) navigateUp() {
	isHDR := strings.Contains(m.monitor.ColorMode, "hdr")

	m.focusedField--

	// Skip SDR fields if not in HDR mode
	if !isHDR && (m.focusedField == fieldSDRSaturation || m.focusedField == fieldSDRBrightness) {
		m.focusedField = fieldColorMode
	}

	if m.focusedField < 0 {
		m.focusedField = fieldCount - 1
		// If we wrapped to the end and HDR is off, skip SDR fields
		if !isHDR && (m.focusedField == fieldSDRSaturation || m.focusedField == fieldSDRBrightness) {
			m.focusedField = fieldTransform
		}
	}
}

func (m *advancedSettingsModel) adjustValue(delta int) {
	switch m.focusedField {
	case fieldSDRBrightness:
		m.monitor.SDRBrightness += float32(delta) * 0.1
		if m.monitor.SDRBrightness < 0.5 {
			m.monitor.SDRBrightness = 0.5
		}
		if m.monitor.SDRBrightness > 2.0 {
			m.monitor.SDRBrightness = 2.0
		}

	case fieldSDRSaturation:
		m.monitor.SDRSaturation += float32(delta) * 0.1
		if m.monitor.SDRSaturation < 0.5 {
			m.monitor.SDRSaturation = 0.5
		}
		if m.monitor.SDRSaturation > 1.5 {
			m.monitor.SDRSaturation = 1.5
		}
	}
}

func (m *advancedSettingsModel) toggleValue() {
	switch m.focusedField {
	case fieldBitDepth:
		if m.monitor.BitDepth == 8 {
			m.monitor.BitDepth = 10
		} else {
			m.monitor.BitDepth = 8
		}

	case fieldColorMode:
		modes := []string{"auto", "srgb", "wide", "edid", "hdr", "hdredid"}
		currentIdx := 0
		for i, mode := range modes {
			if m.monitor.ColorMode == mode {
				currentIdx = i
				break
			}
		}
		currentIdx = (currentIdx + 1) % len(modes)
		m.monitor.ColorMode = modes[currentIdx]

		// If we switched away from HDR and were focused on SDR fields, move focus
		if !strings.Contains(m.monitor.ColorMode, "hdr") &&
			(m.focusedField == fieldSDRBrightness || m.focusedField == fieldSDRSaturation) {
			m.focusedField = fieldColorMode
		}

	case fieldVRR:
		m.monitor.VRR = (m.monitor.VRR + 1) % 3

	case fieldTransform:
		m.monitor.Transform = (m.monitor.Transform + 1) % 8
	}
}

func (m advancedSettingsModel) View() string {
	if m.width == 0 || m.height == 0 {
		m.width = 60
		m.height = 20
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42")).
		Padding(1, 2).
		Width(56).
		Height(16)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Width(16).
		Foreground(lipgloss.Color("244"))

	focusedLabelStyle := labelStyle.
		Foreground(lipgloss.Color("214")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	focusedValueStyle := valueStyle.
		Foreground(lipgloss.Color("214")).
		Bold(true)

	var content strings.Builder

	title := fmt.Sprintf("Advanced Display Settings [%s]", m.monitor.Name)
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Bit Depth
	label := "Color Depth:"
	value := m.renderBitDepth()
	if m.focusedField == fieldBitDepth {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// Color Mode
	label = "Color Mode:"
	value = m.renderColorMode()
	if m.focusedField == fieldColorMode {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// SDR Brightness (only show if HDR mode)
	if strings.Contains(m.monitor.ColorMode, "hdr") {
		label = "SDR Brightness:"
		value = m.renderSDRBrightness()
		if m.focusedField == fieldSDRBrightness {
			content.WriteString(focusedLabelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(focusedValueStyle.Render(value))
		} else {
			content.WriteString(labelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(valueStyle.Render(value))
		}
		content.WriteString("\n")

		// SDR Saturation
		label = "SDR Saturation:"
		value = m.renderSDRSaturation()
		if m.focusedField == fieldSDRSaturation {
			content.WriteString(focusedLabelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(focusedValueStyle.Render(value))
		} else {
			content.WriteString(labelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(valueStyle.Render(value))
		}
		content.WriteString("\n")
	}

	// VRR
	label = "VRR Mode:"
	value = m.renderVRR()
	if m.focusedField == fieldVRR {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// Transform
	label = "Transform:"
	value = m.renderTransform()
	if m.focusedField == fieldTransform {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n\n")

	// Controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	controls := "[Tab/↑↓] Navigate  [Space] Toggle  [←→] Adjust\n[Enter] Apply  [Esc] Cancel"
	content.WriteString(controlsStyle.Render(controls))

	dialog := dialogStyle.Render(content.String())

	// Center the dialog
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (m advancedSettingsModel) renderBitDepth() string {
	if m.monitor.BitDepth == 10 {
		return "○ 8-bit  ● 10-bit"
	}
	return "● 8-bit  ○ 10-bit"
}

func (m advancedSettingsModel) renderColorMode() string {
	modes := map[string]string{
		"auto":    "Auto",
		"srgb":    "sRGB",
		"wide":    "Wide",
		"edid":    "EDID",
		"hdr":     "HDR",
		"hdredid": "HDR-EDID",
	}

	var parts []string
	for _, key := range []string{"auto", "srgb", "wide", "hdr"} {
		if m.monitor.ColorMode == key {
			parts = append(parts, "● "+modes[key])
		} else {
			parts = append(parts, "○ "+modes[key])
		}
	}
	return strings.Join(parts, "  ")
}

func (m advancedSettingsModel) renderSDRBrightness() string {
	// Create a slider visualization
	width := 20
	value := m.monitor.SDRBrightness
	if value == 0 {
		value = 1.0 // Default
	}
	pos := int((value - 0.5) / 1.5 * float32(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	slider := make([]rune, width)
	for i := range slider {
		if i == pos {
			slider[i] = '●'
		} else {
			slider[i] = '─'
		}
	}
	return fmt.Sprintf("[%s] %.1f", string(slider), value)
}

func (m advancedSettingsModel) renderSDRSaturation() string {
	// Create a slider visualization
	width := 20
	value := m.monitor.SDRSaturation
	if value == 0 {
		value = 1.0 // Default
	}
	pos := int((value - 0.5) / 1.0 * float32(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	slider := make([]rune, width)
	for i := range slider {
		if i == pos {
			slider[i] = '●'
		} else {
			slider[i] = '─'
		}
	}
	return fmt.Sprintf("[%s] %.1f", string(slider), value)
}

func (m advancedSettingsModel) renderVRR() string {
	switch m.monitor.VRR {
	case 1:
		return "○ Off  ● On  ○ Fullscreen"
	case 2:
		return "○ Off  ○ On  ● Fullscreen"
	default:
		return "● Off  ○ On  ○ Fullscreen"
	}
}

func (m advancedSettingsModel) renderTransform() string {
	transforms := []string{"Normal", "90°", "180°", "270°", "Flipped", "Flip+90°", "Flip+180°", "Flip+270°"}
	if m.monitor.Transform >= 0 && m.monitor.Transform < len(transforms) {
		// Show current and next option
		current := transforms[m.monitor.Transform]
		if m.monitor.Transform <= 3 {
			// Show rotation options
			var parts []string
			for i := 0; i <= 3; i++ {
				if i == m.monitor.Transform {
					parts = append(parts, "● "+transforms[i])
				} else {
					parts = append(parts, "○ "+transforms[i])
				}
			}
			return strings.Join(parts, "  ")
		} else {
			// Show flipped options
			return "Flipped: " + current
		}
	}
	return "Normal"
}
