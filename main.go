package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var profileName string
	var showProfileMenu bool
	var listProfilesNames bool
	var showVersion bool

	flag.StringVar(&profileName, "profile", "", "Apply a specific profile")
	flag.BoolVar(&showProfileMenu, "profiles", false, "Show profile selection menu")
	flag.BoolVar(&listProfilesNames, "list-profiles", false, "List available profile names")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (short)")
	flag.Parse()

	// Handle version flag
	if showVersion {
		fmt.Println(VersionInfo())
		return
	}

	// Handle list-profiles flag
	if listProfilesNames {
		profiles, err := listProfiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing profiles: %v\n", err)
			os.Exit(1)
		}

		// Load saved order if exists
		savedOrder, _ := loadProfileOrder()
		if len(savedOrder) > 0 {
			// Apply saved order
			orderedProfiles := []string{}
			remainingProfiles := make(map[string]bool)

			for _, p := range profiles {
				remainingProfiles[p] = true
			}

			for _, name := range savedOrder {
				if remainingProfiles[name] {
					orderedProfiles = append(orderedProfiles, name)
					delete(remainingProfiles, name)
				}
			}

			for _, p := range profiles {
				if remainingProfiles[p] {
					orderedProfiles = append(orderedProfiles, p)
				}
			}

			profiles = orderedProfiles
		}

		// Print one profile name per line for easy scripting
		for _, profile := range profiles {
			fmt.Println(profile)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "profiles" {
		showProfileMenu = true
	}

	if profileName != "" {
		if err := applyProfile(profileName); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profile '%s' applied successfully\n", profileName)
		return
	}

	if showProfileMenu {
		m, err := initialProfileMenu()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading profiles: %v\n", err)
			os.Exit(1)
		}

		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		// Check if we should launch the full UI
		if profileModel, ok := finalModel.(profileMenuModel); ok && profileModel.launchFullUI {
			// Continue to launch full UI below
		} else {
			return
		}
	}

	// Main UI loop - may need to restart if switching between views
	for {
		m := initialModel()
		p := tea.NewProgram(m, tea.WithMouseCellMotion(), tea.WithAltScreen())

		finalModel, err := p.Run()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		// Check if we should open profiles page
		if mainModel, ok := finalModel.(model); ok && mainModel.OpenProfiles {
			// Launch profiles page
			profileModel, err := initialProfileMenu()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading profiles: %v\n", err)
				os.Exit(1)
			}

			profileProg := tea.NewProgram(profileModel, tea.WithAltScreen())
			finalProfileModel, err := profileProg.Run()
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			// Check if we should return to main UI
			if pm, ok := finalProfileModel.(profileMenuModel); ok && pm.launchFullUI {
				continue // Go back to main UI
			}
			break // Exit completely
		}
		break // Normal exit
	}
}

func initialModel() model {
	m := model{
		GridPx:     32,
		Snap:       SnapEdges,
		SnapThresh: 10,
		Status:     "Loading monitors...",
	}

	// Set initial terminal size to prevent "Initializing..." stuck state
	m.World.TermW = 80
	m.World.TermH = 24

	// Don't load monitors here - let the Init command do it
	// This ensures proper async loading
	m.updateWorld()
	return m
}
