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
		tea.WindowSize(), // Request initial window size
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle help screen if it's shown
	if m.ShowHelp {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			viewportHeight := m.World.TermH - 6
			pageSize := viewportHeight - 7 // Account for header and footer

			switch msg.String() {
			case "esc", "q", "?":
				// Close help
				m.ShowHelp = false
				m.HelpScrollOffset = 0 // Reset scroll when closing
				return m, nil
			case "up", "k":
				// Scroll up one line
				if m.HelpScrollOffset > 0 {
					m.HelpScrollOffset--
				}
				return m, nil
			case "down", "j":
				// Scroll down one line
				m.HelpScrollOffset++
				return m, nil
			case "pgup":
				// Page up
				m.HelpScrollOffset -= pageSize
				if m.HelpScrollOffset < 0 {
					m.HelpScrollOffset = 0
				}
				return m, nil
			case "pgdown":
				// Page down
				m.HelpScrollOffset += pageSize
				return m, nil
			case "home":
				// Jump to top
				m.HelpScrollOffset = 0
				return m, nil
			case "end":
				// Jump to bottom (will be clamped in renderHelp)
				m.HelpScrollOffset = 9999
				return m, nil
			}
			return m, nil
		case tea.MouseMsg:
			if msg.Action == tea.MouseActionPress {
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					// Scroll up
					if m.HelpScrollOffset > 0 {
						m.HelpScrollOffset--
					}
					return m, nil
				case tea.MouseButtonWheelDown:
					// Scroll down
					m.HelpScrollOffset++
					return m, nil
				default:
					// Other mouse actions close help
					m.ShowHelp = false
					m.HelpScrollOffset = 0
					return m, nil
				}
			}
			return m, nil
		}
		return m, nil
	}

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

	// Handle mode picker if it's shown
	if m.ShowModePicker {
		switch msg := msg.(type) {
		case modeSelectedMsg:
			if m.Selected >= 0 && m.Selected < len(m.Monitors) {
				m.Monitors[m.Selected].PxW = msg.mode.Width
				m.Monitors[m.Selected].PxH = msg.mode.Height
				m.Monitors[m.Selected].Hz = msg.mode.RefreshRate
				m.Status = fmt.Sprintf("Mode set to %dx%d@%.2fHz", msg.mode.Width, msg.mode.Height, msg.mode.RefreshRate)
			}
			m.ShowModePicker = false
			return m, nil

		case modeCancelledMsg:
			m.ShowModePicker = false
			m.Status = "Mode selection cancelled"
			return m, nil

		case tea.KeyMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				// Allow quitting from mode picker
				return m, tea.Quit
			}
		}

		// Pass other messages to mode picker
		newPicker, cmd := m.ModePicker.Update(msg)
		m.ModePicker = newPicker.(modePickerModel)
		return m, cmd
	}

	// Handle mirror picker if it's shown
	if m.ShowMirrorPicker {
		switch msg := msg.(type) {
		case mirrorSelectedMsg:
			if m.Selected >= 0 && m.Selected < len(m.Monitors) {
				// Update mirror settings
				mon := &m.Monitors[m.Selected]

				// Clear previous mirror relationships
				if mon.IsMirrored && mon.MirrorSource != "" {
					// Remove this monitor from its source's targets
					for i := range m.Monitors {
						if m.Monitors[i].Name == mon.MirrorSource {
							targets := m.Monitors[i].MirrorTargets
							for j, target := range targets {
								if target == mon.Name {
									m.Monitors[i].MirrorTargets = append(targets[:j], targets[j+1:]...)
									break
								}
							}
							break
						}
					}
				}

				if msg.source == "" {
					// Disable mirroring
					mon.IsMirrored = false
					mon.MirrorSource = ""
					m.Status = fmt.Sprintf("Mirroring disabled for %s", mon.Name)
				} else {
					// Enable mirroring
					mon.IsMirrored = true
					mon.MirrorSource = msg.source
					// Add this monitor to source's targets
					for i := range m.Monitors {
						if m.Monitors[i].Name == msg.source {
							m.Monitors[i].MirrorTargets = append(m.Monitors[i].MirrorTargets, mon.Name)
							break
						}
					}
					m.Status = fmt.Sprintf("Mirroring %s to %s", mon.Name, msg.source)
				}

				// Check for configuration warnings
				warnings := validateMirrorConfiguration(m.Monitors)
				if len(warnings) > 0 {
					m.Status += " | Warnings: " + warnings[0] // Show first warning
				}
			}
			m.ShowMirrorPicker = false
			return m, nil

		case mirrorCancelledMsg:
			m.ShowMirrorPicker = false
			m.Status = "Mirror selection cancelled"
			return m, nil

		case tea.KeyMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				// Allow quitting from mirror picker
				return m, tea.Quit
			}
		}

		// Pass other messages to mirror picker
		newPicker, cmd := m.MirrorPicker.Update(msg)
		m.MirrorPicker = newPicker.(mirrorPickerModel)
		return m, cmd
	}

	// Handle advanced settings dialog if it's shown
	if m.ShowAdvancedSettings {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Apply settings and close dialog
				m.ShowAdvancedSettings = false
				m.Status = "Advanced settings applied"
				return m, nil
			case "esc":
				// Cancel and close dialog
				m.ShowAdvancedSettings = false
				m.Status = "Advanced settings cancelled"
				return m, nil
			case "ctrl+c":
				// Allow force quitting
				return m, tea.Quit
			}
		}

		// Pass other messages to advanced settings
		newSettings, cmd := m.AdvancedSettings.Update(msg)
		m.AdvancedSettings = newSettings
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.World.TermW = msg.Width
		m.World.TermH = msg.Height
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

			// Store current monitor names for tracking
			var names []string
			for _, mon := range m.Monitors {
				if mon.Active {
					names = append(names, mon.Name)
				}
			}
			m.PreviousMonitorNames = names
		}
		// Force a window size refresh after initial load
		return m, tea.WindowSize()

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
				if m.canDisableMonitor(hit) {
					m.Monitors[hit].Active = !m.Monitors[hit].Active
					m.Status = fmt.Sprintf("Monitor %s: %s",
						m.Monitors[hit].Name,
						map[bool]string{true: "Active", false: "Inactive"}[m.Monitors[hit].Active])
				} else {
					m.Status = "Cannot disable the last active monitor"
				}
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
	case "?":
		m.ShowHelp = true
		return m, nil

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

	case "r", "R":
		// Open scale picker for selected monitor
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			mon := m.Monitors[m.Selected]
			m.ScalePicker = newScalePicker(mon.Name, mon.Scale, mon.PxW, mon.PxH)
			m.ShowScalePicker = true
		}

	case "f", "F":
		// Open mode picker for selected monitor
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			mon := m.Monitors[m.Selected]
			modes, err := getAvailableModes(mon.Name)
			if err != nil {
				m.Status = fmt.Sprintf("Failed to get available modes: %v", err)
				return m, nil
			}
			m.ModePicker = newModePicker(mon.Name, mon.PxW, mon.PxH, mon.Hz, modes)
			m.ShowModePicker = true
		}

	case "m", "M":
		// Open mirror picker for selected monitor
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			mon := m.Monitors[m.Selected]
			m.MirrorPicker = newMirrorPicker(mon.Name, mon.MirrorSource, m.Monitors)
			m.ShowMirrorPicker = true
		}

	case "c", "C", "d", "D":
		// Open advanced settings dialog for selected monitor
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			// Initialize with defaults if values are not set
			mon := &m.Monitors[m.Selected]
			if mon.BitDepth == 0 {
				mon.BitDepth = 8
			}
			if mon.ColorMode == "" {
				mon.ColorMode = "srgb"
			}
			if mon.SDRBrightness == 0 {
				mon.SDRBrightness = 1.0
			}
			if mon.SDRSaturation == 0 {
				mon.SDRSaturation = 1.0
			}
			m.AdvancedSettings = newAdvancedSettingsModel(mon)
			m.ShowAdvancedSettings = true
		}

	case "a", "A":
		saveRollback(m.Monitors)
		return m, applyCmd(m.Monitors)

	case "s", "S":
		return m, saveCmd(m.Monitors)

	case "z", "Z":
		return m, revertCmd()

	case "o", "O":
		// Open profiles page
		m.OpenProfiles = true
		return m, tea.Quit

	case "p", "P":
		// Show profile input dialog
		m.ProfileInput = newProfileInput()
		m.ShowProfileInput = true

	case "enter", " ":
		if m.Selected >= 0 && m.Selected < len(m.Monitors) {
			if m.canDisableMonitor(m.Selected) {
				m.Monitors[m.Selected].Active = !m.Monitors[m.Selected].Active
				m.Status = fmt.Sprintf("Monitor %s: %s",
					m.Monitors[m.Selected].Name,
					map[bool]string{true: "Active", false: "Inactive"}[m.Monitors[m.Selected].Active])
			} else {
				m.Status = "Cannot disable the last active monitor"
			}
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
		// Get current monitor names before applying changes
		previousNames, _ := getCurrentMonitorNames()

		// Apply the monitor configuration
		err := applyMonitors(monitors)
		if err != nil {
			return applyMsg{success: false, err: err}
		}

		// Get monitor names after applying changes
		currentNames, _ := getCurrentMonitorNames()

		// Migrate orphaned workspaces if monitors were removed
		if err := migrateOrphanedWorkspaces(previousNames, currentNames); err != nil {
			// Log the error but don't fail the apply operation
			fmt.Printf("Warning: Failed to migrate workspaces: %v\n", err)
		}

		return applyMsg{success: true, err: nil}
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
