package main

import (
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadMonitorsCmd(),
		tea.EnterAltScreen,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle profile input if it's shown
	if m.ShowProfileInput {
		switch msg := msg.(type) {
		case profileSaveMsg:
			if err := saveProfile(msg.name, m.Monitors); err != nil {
				m.Status = fmt.Sprintf("Failed to save profile: %v", err)
			} else {
				m.Status = fmt.Sprintf("Profile '%s' saved", msg.name)
				m.ProfileName = msg.name
			}
			m.ShowProfileInput = false
			return m, nil

		case profileInputCancelMsg:
			m.ShowProfileInput = false
			m.Status = "Profile save cancelled"
			return m, nil

		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				// Allow force quitting from profile input
				return m, tea.Quit
			}
		}

		// Pass other messages to profile input
		newInput, cmd := m.ProfileInput.Update(msg)
		m.ProfileInput = newInput.(profileInputModel)
		return m, cmd
	}

	// Handle scale picker if it's shown
	if m.ShowScalePicker {
		switch msg := msg.(type) {
		case scaleSelectedMsg:
			if m.Selected >= 0 && m.Selected < len(m.Monitors) {
				m.Monitors[m.Selected].Scale = msg.scale
				m.Status = fmt.Sprintf("Scale set to %.2fx", msg.scale)
			}
			m.ShowScalePicker = false
			return m, nil

		case scaleCancelledMsg:
			m.ShowScalePicker = false
			m.Status = "Scale selection cancelled"
			return m, nil

		case tea.KeyMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				// Allow quitting from scale picker
				return m, tea.Quit
			}
		}

		// Pass other messages to scale picker
		newPicker, cmd := m.ScalePicker.Update(msg)
		m.ScalePicker = newPicker.(scalePickerModel)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.World.TermW = msg.Width - 2
		m.World.TermH = msg.Height - 6
		return m, nil

	case initMsg:
		if msg.err != nil {
			m.Status = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.Monitors = msg.monitors
			if len(m.Monitors) > 0 {
				m.Selected = 0
			}
			m.Status = fmt.Sprintf("Loaded %d monitors", len(m.Monitors))
			m.updateWorld()
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case applyMsg:
		if msg.success {
			m.Status = "Changes applied successfully"
		} else {
			m.Status = fmt.Sprintf("Failed to apply: %v", msg.err)
		}
		return m, nil

	case saveMsg:
		if msg.success {
			m.Status = "Configuration saved"
		} else {
			m.Status = fmt.Sprintf("Failed to save: %v", msg.err)
		}
		return m, nil

	case revertMsg:
		if msg.success {
			m.Status = "Reverted to previous configuration"
		} else {
			m.Status = fmt.Sprintf("Failed to revert: %v", msg.err)
		}
		return m, reloadMonitorsCmd()
	}

	return m, nil
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.LastMouseX = m.MouseX
	m.LastMouseY = m.MouseY
	m.MouseX = msg.X
	m.MouseY = msg.Y

	switch msg.Action {
	case tea.MouseActionPress:
		switch msg.Button {
		case tea.MouseButtonLeft:
			hit := m.hitTest(msg.X, msg.Y-2)
			if hit >= 0 {
				m.Selected = hit
				m.beginDrag(msg)
			}
		case tea.MouseButtonRight:
			hit := m.hitTest(msg.X, msg.Y-2)
			if hit >= 0 {
				m.Monitors[hit].Active = !m.Monitors[hit].Active
				m.Status = fmt.Sprintf("Monitor %s: %s",
					m.Monitors[hit].Name,
					map[bool]string{true: "Active", false: "Inactive"}[m.Monitors[hit].Active])
			}
		case tea.MouseButtonWheelUp:
			if m.Selected >= 0 && m.Selected < len(m.Monitors) {
				mon := &m.Monitors[m.Selected]
				delta := float32(0.05)
				mon.Scale = clamp(mon.Scale+delta, 0.5, 3.0)
				m.Status = fmt.Sprintf("Scale: %.2f", mon.Scale)
			}
		case tea.MouseButtonWheelDown:
			if m.Selected >= 0 && m.Selected < len(m.Monitors) {
				mon := &m.Monitors[m.Selected]
				delta := float32(0.05)
				mon.Scale = clamp(mon.Scale-delta, 0.5, 3.0)
				m.Status = fmt.Sprintf("Scale: %.2f", mon.Scale)
			}
		}
	case tea.MouseActionRelease:
		if msg.Button == tea.MouseButtonLeft {
			if m.Selected >= 0 && m.Selected < len(m.Monitors) {
				m.endDrag()
			}
		}
	case tea.MouseActionMotion:
		m.dragMove(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		if len(m.Monitors) > 0 {
			m.Selected = (m.Selected + 1) % len(m.Monitors)
		}

	case "shift+tab":
		if len(m.Monitors) > 0 {
			m.Selected = (m.Selected - 1 + len(m.Monitors)) % len(m.Monitors)
		}

	case "up", "k":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx)
			mon := &m.Monitors[m.Selected]
			mon.Y -= step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "shift+up", "K":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx) * 10
			mon := &m.Monitors[m.Selected]
			mon.Y -= step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "down", "j":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx)
			mon := &m.Monitors[m.Selected]
			mon.Y += step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "shift+down", "J":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx) * 10
			mon := &m.Monitors[m.Selected]
			mon.Y += step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "left", "h":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx)
			mon := &m.Monitors[m.Selected]
			mon.X -= step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "shift+left", "H":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx) * 10
			mon := &m.Monitors[m.Selected]
			mon.X -= step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "right", "l":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx)
			mon := &m.Monitors[m.Selected]
			mon.X += step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "shift+right":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			step := int32(m.GridPx) * 10
			mon := &m.Monitors[m.Selected]
			mon.X += step
			if m.Snap != SnapOff {
				mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
			} else {
				m.Guides = nil
			}
		}

	case "g", "G":
		grids := []int{1, 8, 16, 32, 64}
		currentIdx := 0
		for i, g := range grids {
			if g == m.GridPx {
				currentIdx = i
				break
			}
		}
		m.GridPx = grids[(currentIdx+1)%len(grids)]
		m.Status = fmt.Sprintf("Grid: %d px", m.GridPx)

	case "L":
		m.Snap = SnapMode((int(m.Snap) + 1) % 4)
		snapNames := []string{"Off", "Edges", "Centers", "Both"}
		m.Status = fmt.Sprintf("Snap: %s", snapNames[m.Snap])

	case "]":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			mon := &m.Monitors[m.Selected]
			mon.Scale = clamp(mon.Scale+0.05, 0.5, 3.0)
			m.Status = fmt.Sprintf("Scale: %.2f", mon.Scale)
		}

	case "[":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			mon := &m.Monitors[m.Selected]
			mon.Scale = clamp(mon.Scale-0.05, 0.5, 3.0)
			m.Status = fmt.Sprintf("Scale: %.2f", mon.Scale)
		}

	case "r", "R":
		// Open scale picker for selected monitor
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			mon := m.Monitors[m.Selected]
			m.ScalePicker = newScalePicker(mon.Name, mon.Scale, mon.PxW, mon.PxH)
			m.ShowScalePicker = true
		}

	case "a", "A":
		saveRollback(m.Monitors)
		return m, applyCmd(m.Monitors)

	case "s", "S":
		return m, saveCmd(m.Monitors)

	case "z", "Z":
		return m, revertCmd()

	case "p", "P":
		// Show profile input dialog
		m.ProfileInput = newProfileInput()
		m.ShowProfileInput = true

	case "enter", " ":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			m.Monitors[m.Selected].Active = !m.Monitors[m.Selected].Active
		}
	}

	return m, nil
}

func clamp(v, min, max float32) float32 {
	return float32(math.Max(float64(min), math.Min(float64(max), float64(v))))
}

func loadMonitorsCmd() tea.Cmd {
	return func() tea.Msg {
		monitors, err := readMonitors()
		return initMsg{monitors: monitors, err: err}
	}
}

func reloadMonitorsCmd() tea.Cmd {
	return func() tea.Msg {
		monitors, err := readMonitors()
		return initMsg{monitors: monitors, err: err}
	}
}

func applyCmd(monitors []Monitor) tea.Cmd {
	return func() tea.Msg {
		err := applyMonitors(monitors)
		return applyMsg{success: err == nil, err: err}
	}
}

func saveCmd(monitors []Monitor) tea.Cmd {
	return func() tea.Msg {
		err := writeConfig(monitors)
		if err == nil {
			err = reloadConfig()
		}
		return saveMsg{success: err == nil, err: err}
	}
}

func revertCmd() tea.Cmd {
	return func() tea.Msg {
		err := rollback()
		return revertMsg{success: err == nil, err: err}
	}
}
