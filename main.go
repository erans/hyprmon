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
	var listProfiles bool
	var showVersion bool

	flag.StringVar(&profileName, "profile", "", "Apply a specific profile")
	flag.BoolVar(&listProfiles, "profiles", false, "Show profile selection menu")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (short)")
	flag.Parse()

	// Handle version flag
	if showVersion {
		fmt.Println(VersionInfo())
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "profiles" {
		listProfiles = true
	}

	if profileName != "" {
		if err := applyProfile(profileName); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profile '%s' applied successfully\n", profileName)
		return
	}

	if listProfiles {
		m, err := initialProfileMenu()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading profiles: %v\n", err)
			os.Exit(1)
		}

		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		return
	}

	m := initialModel()
	p := tea.NewProgram(m, tea.WithMouseCellMotion(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func initialModel() model {
	m := model{
		GridPx:     32,
		Snap:       SnapOff,
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
