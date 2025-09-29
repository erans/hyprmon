package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// customConfigPath holds the custom configuration path set via -cfg flag
var customConfigPath string

type Profile struct {
	Name      string    `json:"name"`
	Monitors  []Monitor `json:"monitors"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func getProfilesDir() string {
	// Use custom config path if provided via -cfg flag
	if customConfigPath != "" {
		return filepath.Join(customConfigPath, "profiles")
	}

	// Otherwise use default path
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "hyprmon", "profiles")
}

func ensureProfilesDir() error {
	dir := getProfilesDir()
	if dir == "" {
		return fmt.Errorf("could not determine profiles directory")
	}
	return os.MkdirAll(dir, profileDirMode)
}

func saveProfile(name string, monitors []Monitor) error {
	if err := ensureProfilesDir(); err != nil {
		return err
	}

	profile := Profile{
		Name:      name,
		Monitors:  monitors,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	filename := filepath.Join(getProfilesDir(), fmt.Sprintf("%s.json", name))

	if _, err := os.Stat(filename); err == nil {
		existingProfile, err := loadProfile(name)
		if err == nil {
			profile.CreatedAt = existingProfile.CreatedAt
		}
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(filename, data, profileFileMode); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}
	return nil
}

func loadProfile(name string) (*Profile, error) {
	filename := filepath.Join(getProfilesDir(), fmt.Sprintf("%s.json", name))

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
	}

	return &profile, nil
}

func listProfiles() ([]string, error) {
	dir := getProfilesDir()
	if dir == "" {
		return nil, fmt.Errorf("could not determine profiles directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			profiles = append(profiles, name)
		}
	}

	return profiles, nil
}

func deleteProfile(name string) error {
	filename := filepath.Join(getProfilesDir(), fmt.Sprintf("%s.json", name))
	return os.Remove(filename)
}

func getProfileOrderFile() string {
	dir := getProfilesDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, ".profile_order")
}

func loadProfileOrder() ([]string, error) {
	filename := getProfileOrderFile()
	if filename == "" {
		return nil, fmt.Errorf("could not determine profile order file")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read profile order file: %w", err)
	}

	var order []string
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile order: %w", err)
	}

	return order, nil
}

func saveProfileOrder(order []string) error {
	filename := getProfileOrderFile()
	if filename == "" {
		return fmt.Errorf("could not determine profile order file")
	}

	if err := ensureProfilesDir(); err != nil {
		return fmt.Errorf("failed to ensure profiles directory: %w", err)
	}

	data, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile order: %w", err)
	}

	if err := os.WriteFile(filename, data, profileFileMode); err != nil {
		return fmt.Errorf("failed to write profile order: %w", err)
	}
	return nil
}

func renameProfile(oldName, newName string) error {
	if oldName == newName {
		return nil
	}

	// Check if new name already exists
	newFilename := filepath.Join(getProfilesDir(), fmt.Sprintf("%s.json", newName))
	if _, err := os.Stat(newFilename); err == nil {
		return fmt.Errorf("profile '%s' already exists", newName)
	}

	// Load the old profile
	profile, err := loadProfile(oldName)
	if err != nil {
		return fmt.Errorf("failed to load old profile: %w", err)
	}

	// Update the name
	profile.Name = newName

	// Save with new name
	if err := saveProfile(newName, profile.Monitors); err != nil {
		return fmt.Errorf("failed to save renamed profile: %w", err)
	}

	// Delete old file
	if err := deleteProfile(oldName); err != nil {
		return fmt.Errorf("failed to delete old profile: %w", err)
	}
	return nil
}

func applyProfile(name string) error {
	profile, err := loadProfile(name)
	if err != nil {
		return fmt.Errorf("failed to load profile %s: %w", name, err)
	}

	saveRollback(profile.Monitors)

	if err := applyMonitors(profile.Monitors); err != nil {
		return fmt.Errorf("failed to apply profile: %w", err)
	}

	return nil
}

// compareMonitorConfigurations compares two monitor configurations for equality
func compareMonitorConfigurations(current, saved []Monitor) bool {
	if len(current) != len(saved) {
		return false
	}

	// Create maps for easier lookup by monitor name
	currentMap := make(map[string]Monitor)
	savedMap := make(map[string]Monitor)

	for _, m := range current {
		currentMap[m.Name] = m
	}

	for _, m := range saved {
		savedMap[m.Name] = m
	}

	// Check if both configurations have the same monitors
	for name := range currentMap {
		if _, exists := savedMap[name]; !exists {
			return false
		}
	}

	for name := range savedMap {
		if _, exists := currentMap[name]; !exists {
			return false
		}
	}

	// Compare each monitor's configuration - ALL monitors must match
	for name, currentMonitor := range currentMap {
		savedMonitor := savedMap[name]

		// Check if all key properties match for this monitor
		if currentMonitor.PxW != savedMonitor.PxW ||
			currentMonitor.PxH != savedMonitor.PxH ||
			currentMonitor.X != savedMonitor.X ||
			currentMonitor.Y != savedMonitor.Y ||
			currentMonitor.Active != savedMonitor.Active {
			return false // If any monitor doesn't match, configurations are different
		}
	}

	return true // All monitors match
}

// getCurrentActiveProfile returns the name of the currently active profile by comparing
// the current monitor configuration with all saved profiles
func getCurrentActiveProfile() (string, error) {
	// Get current monitor configuration
	currentMonitors, err := readMonitors()
	if err != nil {
		return "", fmt.Errorf("failed to read current monitors: %w", err)
	}

	// Get list of all profiles
	profiles, err := listProfiles()
	if err != nil {
		return "", fmt.Errorf("failed to list profiles: %w", err)
	}

	// Compare current configuration with each profile
	for _, profileName := range profiles {
		profile, err := loadProfile(profileName)
		if err != nil {
			continue // Skip profiles that can't be loaded
		}

		if compareMonitorConfigurations(currentMonitors, profile.Monitors) {
			return profileName, nil
		}
	}

	return "", nil // No matching profile found
}

type profileMenuModel struct {
	profiles        []string
	selected        int
	err             error
	confirmDelete   bool
	deleteCandidate string
	renaming        bool
	renameCandidate string
	renameInput     string
	renameCursor    int
	profileOrder    []string // Keep track of custom order
	showHelp        bool
	launchFullUI    bool // Flag to indicate launching full UI
	termWidth       int  // Terminal width for responsive layout
	termHeight      int  // Terminal height
}

func initialProfileMenu() (profileMenuModel, error) {
	profiles, err := listProfiles()
	if err != nil {
		return profileMenuModel{err: err}, nil
	}

	// Load saved order
	savedOrder, _ := loadProfileOrder()

	// Apply saved order if it exists
	if len(savedOrder) > 0 {
		orderedProfiles := []string{}
		remainingProfiles := make(map[string]bool)

		// Mark all profiles as remaining
		for _, p := range profiles {
			remainingProfiles[p] = true
		}

		// Add profiles in saved order
		for _, name := range savedOrder {
			if remainingProfiles[name] {
				orderedProfiles = append(orderedProfiles, name)
				delete(remainingProfiles, name)
			}
		}

		// Add any new profiles not in saved order
		for _, p := range profiles {
			if remainingProfiles[p] {
				orderedProfiles = append(orderedProfiles, p)
			}
		}

		profiles = orderedProfiles
	}

	// Store the profile order (without UI elements)
	profileOrder := make([]string, len(profiles))
	copy(profileOrder, profiles)

	// Add options at the end
	profiles = append(profiles, "──────────────────")
	profiles = append(profiles, "[ Open Full UI ]")

	// Try to get terminal size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fall back to defaults if we can't get terminal size
		width = 80
		height = 24
	}

	return profileMenuModel{
		profiles:     profiles,
		selected:     0,
		profileOrder: profileOrder,
		termWidth:    width,
		termHeight:   height,
	}, nil
}

func (m profileMenuModel) Init() tea.Cmd {
	return tea.WindowSize()
}

func (m profileMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle help screen first
		if m.showHelp {
			// Any key closes help
			m.showHelp = false
			return m, nil
		}

		// Handle rename input
		if m.renaming {
			switch msg.String() {
			case "enter":
				// Apply rename
				newName := strings.TrimSpace(m.renameInput)
				if newName != "" && newName != m.renameCandidate {
					if err := renameProfile(m.renameCandidate, newName); err != nil {
						m.err = err
					} else {
						// Update profile order with new name
						for i, p := range m.profileOrder {
							if p == m.renameCandidate {
								m.profileOrder[i] = newName
								break
							}
						}
						_ = saveProfileOrder(m.profileOrder)

						// Rebuild profiles list maintaining order
						profiles := make([]string, len(m.profileOrder))
						copy(profiles, m.profileOrder)
						profiles = append(profiles, "──────────────────")
						profiles = append(profiles, "[ Open Full UI ]")
						m.profiles = profiles

						// Try to keep selection on renamed item
						for i, p := range m.profiles {
							if p == newName {
								m.selected = i
								break
							}
						}
					}
				}
				m.renaming = false
				m.renameCandidate = ""
				m.renameInput = ""
				m.renameCursor = 0
				return m, nil

			case "esc":
				// Cancel rename
				m.renaming = false
				m.renameCandidate = ""
				m.renameInput = ""
				m.renameCursor = 0
				return m, nil

			case "backspace":
				if m.renameCursor > 0 {
					m.renameInput = m.renameInput[:m.renameCursor-1] + m.renameInput[m.renameCursor:]
					m.renameCursor--
				}

			case "left":
				if m.renameCursor > 0 {
					m.renameCursor--
				}

			case "right":
				if m.renameCursor < len(m.renameInput) {
					m.renameCursor++
				}

			case "home":
				m.renameCursor = 0

			case "end":
				m.renameCursor = len(m.renameInput)

			default:
				// Add character at cursor position
				if len(msg.String()) == 1 && msg.String()[0] >= 32 && msg.String()[0] < 127 {
					m.renameInput = m.renameInput[:m.renameCursor] + msg.String() + m.renameInput[m.renameCursor:]
					m.renameCursor++
				}
			}
			return m, nil
		}

		// Handle delete confirmation
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				// Confirmed - delete the profile
				if err := deleteProfile(m.deleteCandidate); err != nil {
					m.err = err
				} else {
					// Remove from profile order
					newOrder := []string{}
					for _, p := range m.profileOrder {
						if p != m.deleteCandidate {
							newOrder = append(newOrder, p)
						}
					}
					m.profileOrder = newOrder
					_ = saveProfileOrder(m.profileOrder)

					// Rebuild profiles list maintaining order
					profiles := make([]string, len(m.profileOrder))
					copy(profiles, m.profileOrder)
					profiles = append(profiles, "──────────────────")
					profiles = append(profiles, "[ Open Full UI ]")
					m.profiles = profiles
					if m.selected >= len(m.profiles) {
						m.selected = len(m.profiles) - 1
					}
				}
				m.confirmDelete = false
				m.deleteCandidate = ""
				return m, nil

			case "n", "N", "esc":
				// Cancelled
				m.confirmDelete = false
				m.deleteCandidate = ""
				return m, nil

			default:
				// Ignore other keys during confirmation
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "shift+up":
			// Reorder profile up
			if m.selected > 0 && m.selected < len(m.profiles)-2 &&
				!strings.HasPrefix(m.profiles[m.selected], "─") {
				// Swap in display list
				m.profiles[m.selected], m.profiles[m.selected-1] =
					m.profiles[m.selected-1], m.profiles[m.selected]

				// Update profile order and save
				m.profileOrder = make([]string, 0)
				for _, p := range m.profiles {
					if !strings.HasPrefix(p, "─") && p != "[ Open Full UI ]" {
						m.profileOrder = append(m.profileOrder, p)
					}
				}

				// Save the new order
				if err := saveProfileOrder(m.profileOrder); err != nil {
					m.err = err
				}

				m.selected--
			}

		case "shift+down":
			// Reorder profile down
			if m.selected < len(m.profiles)-3 &&
				!strings.HasPrefix(m.profiles[m.selected], "─") &&
				!strings.HasPrefix(m.profiles[m.selected+1], "─") {
				// Swap in display list
				m.profiles[m.selected], m.profiles[m.selected+1] =
					m.profiles[m.selected+1], m.profiles[m.selected]

				// Update profile order and save
				m.profileOrder = make([]string, 0)
				for _, p := range m.profiles {
					if !strings.HasPrefix(p, "─") && p != "[ Open Full UI ]" {
						m.profileOrder = append(m.profileOrder, p)
					}
				}

				// Save the new order
				if err := saveProfileOrder(m.profileOrder); err != nil {
					m.err = err
				}

				m.selected++
			}

		case "up", "k":
			if m.selected > 0 {
				m.selected--
				// Skip separator
				if strings.HasPrefix(m.profiles[m.selected], "─") && m.selected > 0 {
					m.selected--
				}
			}

		case "down", "j":
			if m.selected < len(m.profiles)-1 {
				m.selected++
				// Skip separator
				if strings.HasPrefix(m.profiles[m.selected], "─") && m.selected < len(m.profiles)-1 {
					m.selected++
				}
			}

		case "enter":
			selectedProfile := m.profiles[m.selected]

			if selectedProfile == "[ Open Full UI ]" {
				m.launchFullUI = true
				return m, tea.Quit
			} else if strings.HasPrefix(selectedProfile, "─") {
				// Separator line, do nothing
				return m, nil
			} else {
				err := applyProfile(selectedProfile)
				if err != nil {
					m.err = err
					return m, nil
				}
				fmt.Printf("Applied profile: %s\n", selectedProfile)
				return m, tea.Quit
			}

		case "d", "D":
			// Don't allow deleting the separator or UI option
			if m.selected < len(m.profiles)-2 && !strings.HasPrefix(m.profiles[m.selected], "─") {
				m.deleteCandidate = m.profiles[m.selected]
				m.confirmDelete = true
			}

		case "r", "R":
			// Don't allow renaming the separator or UI option
			if m.selected < len(m.profiles)-2 && !strings.HasPrefix(m.profiles[m.selected], "─") {
				m.renameCandidate = m.profiles[m.selected]
				m.renameInput = m.renameCandidate
				m.renameCursor = len(m.renameInput)
				m.renaming = true
			}

		case "?":
			m.showHelp = true
			return m, nil
		}
	}

	return m, nil
}

func (m profileMenuModel) renderHelp() string {
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
	content.WriteString(titleStyle.Render(fmt.Sprintf("HyprMon Profiles %s", ShortVersion())))
	content.WriteString("\n")
	content.WriteString("Copyright © 2025 Eran Sandler\n\n")
	content.WriteString("Profile management for saved monitor configurations.\n")

	// Navigation
	content.WriteString(sectionStyle.Render("\nNavigation:"))
	content.WriteString("\n")

	navigation := []struct {
		key  string
		desc string
	}{
		{"↑/↓ or k/j", "Move selection up/down"},
		{"Shift+↑/↓", "Reorder profile position"},
		{"Enter", "Apply selected profile"},
		{"?", "Show this help"},
		{"Q / Esc / Ctrl+C", "Exit"},
	}

	for _, n := range navigation {
		content.WriteString(fmt.Sprintf("%s %s\n",
			keyStyle.Render(n.key), n.desc))
	}

	// Profile Management
	content.WriteString(sectionStyle.Render("\nProfile Management:"))
	content.WriteString("\n")

	management := []struct {
		key  string
		desc string
	}{
		{"R", "Rename selected profile"},
		{"D", "Delete selected profile (with confirmation)"},
	}

	for _, m := range management {
		content.WriteString(fmt.Sprintf("%s %s\n",
			keyStyle.Render(m.key), m.desc))
	}

	// About Profiles
	content.WriteString(sectionStyle.Render("\nAbout Profiles:"))
	content.WriteString("\n")
	content.WriteString("• Profiles save your complete monitor configuration\n")
	content.WriteString("• Includes position, resolution, refresh rate, and scale\n")
	content.WriteString("• Profiles are stored in ~/.config/hyprmon/profiles/\n")
	content.WriteString("• Custom ordering is preserved between sessions\n")
	content.WriteString("• Use 'hyprmon -profile NAME' to apply directly from CLI\n")

	// Menu Options
	content.WriteString(sectionStyle.Render("\nMenu Options:"))
	content.WriteString("\n")
	content.WriteString("• Profile names: Your saved monitor configurations\n")
	content.WriteString("• [Open Full UI]: Launch the main HyprMon interface\n")

	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press any key to close help"))

	return helpStyle.Render(content.String())
}

func (m profileMenuModel) View() string {
	// Show help if active
	if m.showHelp {
		return m.renderHelp()
	}

	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	s.WriteString(titleStyle.Render("HyprMon - Profile Selection"))
	s.WriteString("\n\n")

	if m.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))
		s.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n\n")
	}

	// Show rename input if active
	if m.renaming {
		renameStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(0, 1)

		// Build input with cursor
		inputDisplay := m.renameInput[:m.renameCursor] + "│" + m.renameInput[m.renameCursor:]

		renamePrompt := fmt.Sprintf("Rename profile '%s':\n\n%s\n\nPress Enter to save, Esc to cancel",
			m.renameCandidate, inputStyle.Render(inputDisplay))
		s.WriteString(renameStyle.Render(renamePrompt))
		s.WriteString("\n")
		return s.String()
	}

	// Show delete confirmation dialog if active
	if m.confirmDelete {
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("9")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

		confirmPrompt := fmt.Sprintf("Delete profile '%s'?\n\nPress Y to confirm, N to cancel", m.deleteCandidate)
		s.WriteString(confirmStyle.Render(confirmPrompt))
		s.WriteString("\n")
		return s.String()
	}

	// Get the currently active profile for display
	activeProfile, _ := getCurrentActiveProfile()

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(lipgloss.Color("214")).
		Bold(true)

	for i, profile := range m.profiles {
		if strings.HasPrefix(profile, "─") {
			// Render separator
			sepStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))
			s.WriteString(sepStyle.Render(profile))
		} else if i == m.selected {
			displayName := profile
			if profile == activeProfile && profile != "[ Open Full UI ]" {
				displayName = profile + " *"
			}
			s.WriteString(selectedStyle.Render("▶ " + displayName))
		} else {
			displayName := profile
			if profile == activeProfile && profile != "[ Open Full UI ]" {
				displayName = profile + " *"
			}
			s.WriteString(itemStyle.Render("  " + displayName))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")

	// Render responsive footer
	s.WriteString(m.renderFooter())

	return s.String()
}

// profileKeyCommand represents a keyboard command with different verbosity levels for profile menu
type profileKeyCommand struct {
	full     string
	medium   string
	short    string
	priority int // 1 = essential, 2 = important, 3 = nice to have
}

func (m profileMenuModel) renderFooter() string {
	commands := []profileKeyCommand{
		{"↑/↓ Navigate", "↑/↓ Nav", "↑↓", 1},
		{"Shift+↑/↓ Reorder", "S+↑/↓ Order", "S+↑↓", 2},
		{"Enter Select", "Enter Sel", "⏎", 1},
		{"R Rename", "R Rename", "R", 2},
		{"D Delete", "D Delete", "D", 2},
		{"? Help", "? Help", "?", 1},
		{"Q Quit", "Q Quit", "Q", 1},
	}

	// Determine format based on terminal width
	var keys []string
	separator := "  •  "

	width := m.termWidth
	if width <= 0 {
		width = 80 // Fallback
	}

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
		// Medium: all commands, medium form
		separator = " • "
		for _, cmd := range commands {
			keys = append(keys, cmd.medium)
		}
	} else {
		// Wide: all commands, full form
		for _, cmd := range commands {
			keys = append(keys, cmd.full)
		}
	}

	// Check if the footer would overflow and switch to multi-line if needed
	footerText := strings.Join(keys, separator)
	if len(footerText) > width-4 && width >= 60 {
		// Try multi-line layout for medium and wider terminals
		return m.renderMultiLineFooter(commands, width, keys, separator)
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	return helpStyle.Render(footerText)
}

func (m profileMenuModel) renderMultiLineFooter(commands []profileKeyCommand, width int, singleLineKeys []string, separator string) string {
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

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	return helpStyle.Render(strings.Join(lines, "\n"))
}
