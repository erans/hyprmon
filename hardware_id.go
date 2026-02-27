package main

import (
	"fmt"
	"sort"
	"strings"
)

// buildHardwareID creates a stable hardware identifier from EDID data.
// Format: "make/model/serial" or "make/model" when serial is empty.
// Returns "" when both make and model are empty.
func buildHardwareID(make_, model, serial string) string {
	make_ = strings.TrimSpace(make_)
	model = strings.TrimSpace(model)
	serial = strings.TrimSpace(serial)

	if make_ == "" && model == "" {
		return ""
	}

	if serial == "" {
		return fmt.Sprintf("%s/%s", make_, model)
	}
	return fmt.Sprintf("%s/%s/%s", make_, model, serial)
}

// disambiguateHardwareIDs appends /#N suffixes to monitors that share
// the same HardwareID. Numbering is deterministic: sorted by connector Name.
func disambiguateHardwareIDs(monitors []Monitor) {
	counts := make(map[string]int)
	for _, m := range monitors {
		if m.HardwareID != "" {
			counts[m.HardwareID]++
		}
	}

	groups := make(map[string][]int)
	for i, m := range monitors {
		if m.HardwareID != "" && counts[m.HardwareID] > 1 {
			groups[m.HardwareID] = append(groups[m.HardwareID], i)
		}
	}

	for _, indices := range groups {
		sort.Slice(indices, func(a, b int) bool {
			return monitors[indices[a]].Name < monitors[indices[b]].Name
		})
		for n, idx := range indices {
			monitors[idx].HardwareID = fmt.Sprintf("%s/#%d", monitors[idx].HardwareID, n+1)
		}
	}
}

// resolveProfileMonitors takes saved profile monitors and maps them to
// currently connected monitors by HardwareID. Returns only monitors that
// are currently connected, with their connector Name updated to the current value.
func resolveProfileMonitors(saved, current []Monitor) []Monitor {
	currentByHWID := make(map[string]Monitor)
	currentByName := make(map[string]Monitor)
	for _, m := range current {
		if m.HardwareID != "" {
			currentByHWID[m.HardwareID] = m
		}
		currentByName[m.Name] = m
	}

	var resolved []Monitor
	for _, savedMon := range saved {
		var currentMon Monitor
		var found bool

		if savedMon.HardwareID != "" {
			currentMon, found = currentByHWID[savedMon.HardwareID]
		} else {
			// Fallback to Name only for legacy profiles (no HardwareID)
			currentMon, found = currentByName[savedMon.Name]
		}

		if !found {
			continue
		}

		resolvedMon := savedMon
		resolvedMon.Name = currentMon.Name
		if resolvedMon.HardwareID == "" {
			resolvedMon.HardwareID = currentMon.HardwareID
		}
		resolved = append(resolved, resolvedMon)
	}

	return resolved
}

// DisplayLabel returns the best human-readable label for a monitor.
// Priority: Alias > Model > Name.
func (m Monitor) DisplayLabel() string {
	if m.Alias != "" {
		return m.Alias
	}
	if m.Model != "" {
		return m.Model
	}
	return m.Name
}
