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
)

type Profile struct {
	Name      string    `json:"name"`
	Monitors  []Monitor `json:"monitors"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func getProfilesDir() string {
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
	return os.MkdirAll(dir, 0755)
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
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func loadProfile(name string) (*Profile, error) {
	filename := filepath.Join(getProfilesDir(), fmt.Sprintf("%s.json", name))

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
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
		return nil, err
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

type profileMenuModel struct {
	profiles []string
	selected int
	err      error
}

func initialProfileMenu() (profileMenuModel, error) {
	profiles, err := listProfiles()
	if err != nil {
		return profileMenuModel{err: err}, nil
	}

	// Add options at the end
	profiles = append(profiles, "──────────────────")
	profiles = append(profiles, "[ Open Full UI ]")

	return profileMenuModel{
		profiles: profiles,
		selected: 0,
	}, nil
}

func (m profileMenuModel) Init() tea.Cmd {
	return nil
}

func (m profileMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

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
				fmt.Println("Launching full UI...")
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

		case "d":
			// Don't allow deleting the separator or UI option
			if m.selected < len(m.profiles)-2 && !strings.HasPrefix(m.profiles[m.selected], "─") {
				profileName := m.profiles[m.selected]
				if err := deleteProfile(profileName); err != nil {
					m.err = err
				} else {
					profiles, _ := listProfiles()
					profiles = append(profiles, "──────────────────")
					profiles = append(profiles, "[ Open Full UI ]")
					m.profiles = profiles
					if m.selected >= len(m.profiles) {
						m.selected = len(m.profiles) - 1
					}
				}
			}
		}
	}

	return m, nil
}

func (m profileMenuModel) View() string {
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
			s.WriteString(selectedStyle.Render("▶ " + profile))
		} else {
			s.WriteString(itemStyle.Render("  " + profile))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	help := "↑/↓: Navigate  •  Enter: Select  •  D: Delete  •  Q: Quit"
	s.WriteString(helpStyle.Render(help))

	return s.String()
}
