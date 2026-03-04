package main

import (
	"testing"
)

func TestBuildHardwareID(t *testing.T) {
	tests := []struct {
		name     string
		make_    string
		model    string
		serial   string
		expected string
	}{
		{
			name:     "Full EDID with make, model, and serial",
			make_:    "Dell Inc.",
			model:    "DELL U2723QE",
			serial:   "ABC123",
			expected: "Dell Inc./DELL U2723QE/ABC123",
		},
		{
			name:     "No serial",
			make_:    "Samsung",
			model:    "LC49G95T",
			serial:   "",
			expected: "Samsung/LC49G95T",
		},
		{
			name:     "Empty make and model returns empty",
			make_:    "",
			model:    "",
			serial:   "SERIAL",
			expected: "",
		},
		{
			name:     "All empty returns empty",
			make_:    "",
			model:    "",
			serial:   "",
			expected: "",
		},
		{
			name:     "Whitespace trimming on make",
			make_:    "  Dell Inc.  ",
			model:    "U2723QE",
			serial:   "ABC",
			expected: "Dell Inc./U2723QE/ABC",
		},
		{
			name:     "Whitespace trimming on model",
			make_:    "Dell",
			model:    "  U2723QE  ",
			serial:   "ABC",
			expected: "Dell/U2723QE/ABC",
		},
		{
			name:     "Whitespace trimming on serial",
			make_:    "Dell",
			model:    "U2723QE",
			serial:   "  ABC  ",
			expected: "Dell/U2723QE/ABC",
		},
		{
			name:     "Whitespace-only make and model returns empty",
			make_:    "   ",
			model:    "   ",
			serial:   "ABC",
			expected: "",
		},
		{
			name:     "Whitespace-only serial treated as empty",
			make_:    "Dell",
			model:    "U2723QE",
			serial:   "   ",
			expected: "Dell/U2723QE",
		},
		{
			name:     "Only make is set",
			make_:    "LG",
			model:    "",
			serial:   "",
			expected: "LG/",
		},
		{
			name:     "Only model is set",
			make_:    "",
			model:    "27GL850",
			serial:   "",
			expected: "/27GL850",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHardwareID(tt.make_, tt.model, tt.serial)
			if got != tt.expected {
				t.Errorf("buildHardwareID(%q, %q, %q) = %q, want %q",
					tt.make_, tt.model, tt.serial, got, tt.expected)
			}
		})
	}
}

func TestDisambiguateHardwareIDs(t *testing.T) {
	t.Run("No duplicates - IDs unchanged", func(t *testing.T) {
		monitors := []Monitor{
			{Name: "DP-1", HardwareID: "Dell/U2723QE/AAA"},
			{Name: "DP-2", HardwareID: "Samsung/LC49G95T/BBB"},
			{Name: "HDMI-A-1", HardwareID: "LG/27GL850/CCC"},
		}

		disambiguateHardwareIDs(monitors)

		expected := []string{"Dell/U2723QE/AAA", "Samsung/LC49G95T/BBB", "LG/27GL850/CCC"}
		for i, mon := range monitors {
			if mon.HardwareID != expected[i] {
				t.Errorf("monitors[%d].HardwareID = %q, want %q", i, mon.HardwareID, expected[i])
			}
		}
	})

	t.Run("Two duplicates get #1 and #2 sorted by Name", func(t *testing.T) {
		monitors := []Monitor{
			{Name: "DP-3", HardwareID: "Dell/U2723QE"},
			{Name: "DP-1", HardwareID: "Dell/U2723QE"},
		}

		disambiguateHardwareIDs(monitors)

		// DP-1 sorts before DP-3, so DP-1 gets #1 and DP-3 gets #2
		if monitors[0].HardwareID != "Dell/U2723QE/#2" {
			t.Errorf("monitors[0] (DP-3) HardwareID = %q, want %q", monitors[0].HardwareID, "Dell/U2723QE/#2")
		}
		if monitors[1].HardwareID != "Dell/U2723QE/#1" {
			t.Errorf("monitors[1] (DP-1) HardwareID = %q, want %q", monitors[1].HardwareID, "Dell/U2723QE/#1")
		}
	})

	t.Run("Three duplicates get #1 #2 #3 sorted by Name", func(t *testing.T) {
		monitors := []Monitor{
			{Name: "HDMI-A-2", HardwareID: "LG/27GL850"},
			{Name: "DP-1", HardwareID: "LG/27GL850"},
			{Name: "HDMI-A-1", HardwareID: "LG/27GL850"},
		}

		disambiguateHardwareIDs(monitors)

		// Sorted by Name: DP-1 (#1), HDMI-A-1 (#2), HDMI-A-2 (#3)
		if monitors[0].HardwareID != "LG/27GL850/#3" {
			t.Errorf("monitors[0] (HDMI-A-2) HardwareID = %q, want %q", monitors[0].HardwareID, "LG/27GL850/#3")
		}
		if monitors[1].HardwareID != "LG/27GL850/#1" {
			t.Errorf("monitors[1] (DP-1) HardwareID = %q, want %q", monitors[1].HardwareID, "LG/27GL850/#1")
		}
		if monitors[2].HardwareID != "LG/27GL850/#2" {
			t.Errorf("monitors[2] (HDMI-A-1) HardwareID = %q, want %q", monitors[2].HardwareID, "LG/27GL850/#2")
		}
	})

	t.Run("Mix of unique and duplicate IDs", func(t *testing.T) {
		monitors := []Monitor{
			{Name: "DP-2", HardwareID: "Dell/U2723QE"},
			{Name: "HDMI-A-1", HardwareID: "Samsung/LC49G95T/UNIQUE"},
			{Name: "DP-1", HardwareID: "Dell/U2723QE"},
			{Name: "eDP-1", HardwareID: "BOE/NV156FHM/LAPTOP"},
		}

		disambiguateHardwareIDs(monitors)

		// Dell duplicates: DP-1 (#1), DP-2 (#2)
		if monitors[0].HardwareID != "Dell/U2723QE/#2" {
			t.Errorf("monitors[0] (DP-2) HardwareID = %q, want %q", monitors[0].HardwareID, "Dell/U2723QE/#2")
		}
		if monitors[2].HardwareID != "Dell/U2723QE/#1" {
			t.Errorf("monitors[2] (DP-1) HardwareID = %q, want %q", monitors[2].HardwareID, "Dell/U2723QE/#1")
		}

		// Unique IDs should be unchanged
		if monitors[1].HardwareID != "Samsung/LC49G95T/UNIQUE" {
			t.Errorf("monitors[1] (HDMI-A-1) HardwareID = %q, want %q", monitors[1].HardwareID, "Samsung/LC49G95T/UNIQUE")
		}
		if monitors[3].HardwareID != "BOE/NV156FHM/LAPTOP" {
			t.Errorf("monitors[3] (eDP-1) HardwareID = %q, want %q", monitors[3].HardwareID, "BOE/NV156FHM/LAPTOP")
		}
	})

	t.Run("Empty HardwareID monitors are ignored", func(t *testing.T) {
		monitors := []Monitor{
			{Name: "DP-1", HardwareID: "Dell/U2723QE"},
			{Name: "DP-2", HardwareID: ""},
			{Name: "DP-3", HardwareID: "Dell/U2723QE"},
		}

		disambiguateHardwareIDs(monitors)

		// DP-1 (#1), DP-3 (#2)
		if monitors[0].HardwareID != "Dell/U2723QE/#1" {
			t.Errorf("monitors[0] (DP-1) HardwareID = %q, want %q", monitors[0].HardwareID, "Dell/U2723QE/#1")
		}
		if monitors[1].HardwareID != "" {
			t.Errorf("monitors[1] (DP-2) HardwareID = %q, want %q", monitors[1].HardwareID, "")
		}
		if monitors[2].HardwareID != "Dell/U2723QE/#2" {
			t.Errorf("monitors[2] (DP-3) HardwareID = %q, want %q", monitors[2].HardwareID, "Dell/U2723QE/#2")
		}
	})
}

func TestCompareMonitorConfigurationsUsesHardwareID(t *testing.T) {
	tests := []struct {
		name     string
		current  []Monitor
		saved    []Monitor
		expected bool
	}{
		{
			name: "Match by HardwareID even with different connector names",
			current: []Monitor{
				{Name: "DP-4", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			expected: true,
		},
		{
			name: "No match when HardwareID differs",
			current: []Monitor{
				{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "LG/27UK850/ABC123", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			expected: false,
		},
		{
			name: "Fallback to Name when saved HardwareID is empty (legacy)",
			current: []Monitor{
				{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			expected: true,
		},
		{
			name: "Different resolution - no match",
			current: []Monitor{
				{Name: "DP-3", HardwareID: "Dell/U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "Dell/U3419W/5HJB6T2", PxW: 1920, PxH: 1080, X: 0, Y: 0, Active: true},
			},
			expected: false,
		},
		{
			name: "Different count - no match",
			current: []Monitor{
				{Name: "DP-3", HardwareID: "Dell/U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
				{Name: "eDP-1", HardwareID: "Samsung/ATNA33AA08-0", PxW: 2880, PxH: 1800, X: 3440, Y: 0, Active: true},
			},
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "Dell/U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0, Active: true},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareMonitorConfigurations(tt.current, tt.saved)
			if result != tt.expected {
				t.Errorf("compareMonitorConfigurations() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResolveProfileMonitors(t *testing.T) {
	current := []Monitor{
		{Name: "DP-4", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440},
		{Name: "eDP-1", HardwareID: "Samsung Display Corp./ATNA33AA08-0", PxW: 2880, PxH: 1800},
	}

	tests := []struct {
		name          string
		saved         []Monitor
		expectedCount int
		expectedNames map[string]string // HardwareID -> expected Name
	}{
		{
			name: "Remap connector names via HardwareID",
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440, X: 0, Y: 0},
				{Name: "eDP-1", HardwareID: "Samsung Display Corp./ATNA33AA08-0", PxW: 2880, PxH: 1800, X: 3440, Y: 0},
			},
			expectedCount: 2,
			expectedNames: map[string]string{
				"Dell Inc./DELL U3419W/5HJB6T2":      "DP-4",
				"Samsung Display Corp./ATNA33AA08-0": "eDP-1",
			},
		},
		{
			name: "Skip disconnected monitors",
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440},
				{Name: "HDMI-1", HardwareID: "LG/27UK850/XYZ", PxW: 3840, PxH: 2160},
			},
			expectedCount: 1,
			expectedNames: map[string]string{
				"Dell Inc./DELL U3419W/5HJB6T2": "DP-4",
			},
		},
		{
			name: "Legacy profile uses Name fallback",
			saved: []Monitor{
				{Name: "eDP-1", PxW: 2880, PxH: 1800, X: 0, Y: 0},
			},
			expectedCount: 1,
			expectedNames: map[string]string{
				"Samsung Display Corp./ATNA33AA08-0": "eDP-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := resolveProfileMonitors(tt.saved, current)
			if len(resolved) != tt.expectedCount {
				t.Errorf("got %d monitors, want %d", len(resolved), tt.expectedCount)
				return
			}
			for _, m := range resolved {
				expectedName, ok := tt.expectedNames[m.HardwareID]
				if !ok {
					t.Errorf("unexpected HardwareID %q in resolved monitors", m.HardwareID)
					continue
				}
				if m.Name != expectedName {
					t.Errorf("HardwareID %q has Name=%q, want %q", m.HardwareID, m.Name, expectedName)
				}
			}
		})
	}
}

func TestMigrateProfileMonitors(t *testing.T) {
	current := []Monitor{
		{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", Make: "Dell Inc.", Model: "DELL U3419W", Serial: "5HJB6T2"},
		{Name: "eDP-1", HardwareID: "Samsung Display Corp./ATNA33AA08-0", Make: "Samsung Display Corp.", Model: "ATNA33AA08-0"},
	}

	tests := []struct {
		name        string
		saved       []Monitor
		expectHWIDs map[string]string // Name -> expected HardwareID after migration
	}{
		{
			name: "Legacy monitors get HardwareID from current",
			saved: []Monitor{
				{Name: "DP-3", PxW: 3440, PxH: 1440},
				{Name: "eDP-1", PxW: 2880, PxH: 1800},
			},
			expectHWIDs: map[string]string{
				"DP-3":  "Dell Inc./DELL U3419W/5HJB6T2",
				"eDP-1": "Samsung Display Corp./ATNA33AA08-0",
			},
		},
		{
			name: "Already migrated monitors unchanged",
			saved: []Monitor{
				{Name: "DP-3", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2", PxW: 3440, PxH: 1440},
			},
			expectHWIDs: map[string]string{
				"DP-3": "Dell Inc./DELL U3419W/5HJB6T2",
			},
		},
		{
			name: "Disconnected monitor keeps empty HardwareID",
			saved: []Monitor{
				{Name: "HDMI-1", PxW: 1920, PxH: 1080},
			},
			expectHWIDs: map[string]string{
				"HDMI-1": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			migrated := migrateProfileMonitors(tt.saved, current)
			for _, m := range migrated {
				expected, ok := tt.expectHWIDs[m.Name]
				if !ok {
					t.Errorf("unexpected monitor Name %q", m.Name)
					continue
				}
				if m.HardwareID != expected {
					t.Errorf("monitor %q HardwareID = %q, want %q", m.Name, m.HardwareID, expected)
				}
			}
		})
	}
}

func TestNeedsMigration(t *testing.T) {
	tests := []struct {
		name     string
		monitors []Monitor
		expected bool
	}{
		{
			name:     "All have HardwareID",
			monitors: []Monitor{{HardwareID: "Dell/U3419W/5HJB6T2"}, {HardwareID: "Samsung/ATNA33AA08-0"}},
			expected: false,
		},
		{
			name:     "One missing HardwareID",
			monitors: []Monitor{{HardwareID: "Dell/U3419W/5HJB6T2"}, {HardwareID: ""}},
			expected: true,
		},
		{
			name:     "Empty list",
			monitors: []Monitor{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsMigration(tt.monitors)
			if got != tt.expected {
				t.Errorf("needsMigration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMonitorDisplayLabel(t *testing.T) {
	tests := []struct {
		name     string
		monitor  Monitor
		expected string
	}{
		{
			name: "Alias takes highest priority",
			monitor: Monitor{
				Name:  "DP-1",
				Model: "U2723QE",
				Alias: "Left Monitor",
			},
			expected: "Left Monitor",
		},
		{
			name: "Model is used when no alias",
			monitor: Monitor{
				Name:  "DP-1",
				Model: "U2723QE",
				Alias: "",
			},
			expected: "U2723QE",
		},
		{
			name: "Name is last resort",
			monitor: Monitor{
				Name:  "DP-1",
				Model: "",
				Alias: "",
			},
			expected: "DP-1",
		},
		{
			name: "All fields set - alias wins",
			monitor: Monitor{
				Name:  "HDMI-A-1",
				Model: "LC49G95T",
				Alias: "Ultrawide",
			},
			expected: "Ultrawide",
		},
		{
			name: "Only Name set",
			monitor: Monitor{
				Name: "eDP-1",
			},
			expected: "eDP-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.monitor.DisplayLabel()
			if got != tt.expected {
				t.Errorf("DisplayLabel() = %q, want %q", got, tt.expected)
			}
		})
	}
}
