package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type profileInputModel struct {
	input           string
	cursor          int
	confirmOverride bool
	existingName    string
	error           string
}

func newProfileInput() profileInputModel {
	return profileInputModel{
		input:  "",
		cursor: 0,
	}
}

type profileSaveMsg struct {
	name string
}

type profileInputCancelMsg struct{}

func (m profileInputModel) Init() tea.Cmd {
	return nil
}

func (m profileInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle override confirmation
		if m.confirmOverride {
			switch msg.String() {
			case "y", "Y":
				return m, func() tea.Msg {
					return profileSaveMsg{name: m.existingName}
				}
			case "n", "N", "esc":
				m.confirmOverride = false
				m.error = ""
				return m, nil
			}
			return m, nil
		}

		// Normal input handling
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, func() tea.Msg { return profileInputCancelMsg{} }

		case "enter":
			if m.input == "" {
				m.error = "Profile name cannot be empty"
				return m, nil
			}

			// Sanitize the name
			name := strings.TrimSpace(m.input)
			name = strings.ReplaceAll(name, "/", "-")
			name = strings.ReplaceAll(name, "\\", "-")
			name = strings.ReplaceAll(name, "..", "")

			if name == "" {
				m.error = "Invalid profile name"
				return m, nil
			}

			// Check if profile exists
			existing, _ := listProfiles()
			for _, profile := range existing {
				if profile == name {
					m.confirmOverride = true
					m.existingName = name
					return m, nil
				}
			}

			return m, func() tea.Msg {
				return profileSaveMsg{name: name}
			}

		case "backspace", "ctrl+h":
			if m.cursor > 0 {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
				m.error = ""
			}

		case "delete":
			if m.cursor < len(m.input) {
				m.input = m.input[:m.cursor] + m.input[m.cursor+1:]
				m.error = ""
			}

		case "left", "ctrl+b":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right", "ctrl+f":
			if m.cursor < len(m.input) {
				m.cursor++
			}

		case "home", "ctrl+a":
			m.cursor = 0

		case "end", "ctrl+e":
			m.cursor = len(m.input)

		case "ctrl+k":
			m.input = m.input[:m.cursor]
			m.error = ""

		case "ctrl+u":
			m.input = m.input[m.cursor:]
			m.cursor = 0
			m.error = ""

		default:
			// Add character if it's printable
			if len(msg.String()) == 1 && msg.String()[0] >= 32 && msg.String()[0] < 127 {
				m.input = m.input[:m.cursor] + msg.String() + m.input[m.cursor:]
				m.cursor++
				m.error = ""
			}
		}
	}

	return m, nil
}

func (m profileInputModel) View() string {
	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	if m.confirmOverride {
		s.WriteString(titleStyle.Render("Profile Already Exists"))
		s.WriteString("\n\n")

		warningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

		s.WriteString(warningStyle.Render(fmt.Sprintf("Profile '%s' already exists.", m.existingName)))
		s.WriteString("\n")
		s.WriteString("Do you want to override it? (y/n)")

		return s.String()
	}

	s.WriteString(titleStyle.Render("Save Profile"))
	s.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	s.WriteString(labelStyle.Render("Enter profile name:"))
	s.WriteString("\n\n")

	// Input field
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(40)

	// Show cursor
	display := m.input
	if m.cursor < len(display) {
		display = display[:m.cursor] + "│" + display[m.cursor:]
	} else {
		display = display + "│"
	}

	s.WriteString(inputStyle.Render(display))
	s.WriteString("\n\n")

	// Show error if any
	if m.error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))
		s.WriteString(errorStyle.Render("⚠ " + m.error))
		s.WriteString("\n\n")
	}

	// Show existing profiles
	existingProfiles, _ := listProfiles()
	if len(existingProfiles) > 0 {
		s.WriteString(labelStyle.Render("Existing profiles:"))
		s.WriteString("\n")

		listStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			PaddingLeft(2)

		for _, profile := range existingProfiles {
			s.WriteString(listStyle.Render("• " + profile))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	help := "Enter: Save  •  Esc: Cancel  •  Ctrl+U/K: Clear"
	s.WriteString(helpStyle.Render(help))

	// Suggestions
	s.WriteString("\n\n")
	suggestionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")).
		Italic(true)

	s.WriteString(suggestionStyle.Render("Suggestions: home, work, laptop, presentation, gaming"))

	return s.String()
}
