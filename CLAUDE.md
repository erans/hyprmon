# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HyprMon is a TUI (Terminal User Interface) tool for configuring monitors on Arch Linux running Wayland with Hyprland. It provides a visual "desk map" where users can arrange monitors using keyboard and mouse controls, with real-time application to Hyprland.

## Build and Development Commands

```bash
# Build the application
make build

# Run the application
make run

# Run tests
make test

# Format code
make fmt

# Run linting
make lint

# Check formatting without fixing
make fmt-check

# Install dependencies
make deps

# Development build with debug info
make dev

# Build for multiple platforms (Linux amd64 and arm64)
make build-all

# Install git hooks
make hooks

# Install to system (/usr/local/bin)
make install

# Run profile selection menu
make profiles

# Get currently focused monitor
./hyprmon --active-monitor

# List profiles with active profile marked
./hyprmon --list-profiles
```

## Architecture Overview

### Core Components

1. **Main Entry Point** (`main.go`):
   - Handles CLI flags: `--profile`, `--profiles`, `--list-profiles`, `--version`, `--cfg`
   - Orchestrates between profile menu and main UI
   - Uses Bubble Tea framework for TUI

2. **Monitor Management** (`hyprland.go`):
   - Interfaces with Hyprland via `hyprctl` commands
   - `readMonitors()`: Fetches current monitor configuration from Hyprland
   - `applyMonitor()`: Applies monitor settings live to Hyprland
   - `writeConfig()`: Persists monitor configuration to hyprland.conf
   - Handles workspace migration when monitors are added/removed

3. **Profile System** (`profiles.go`):
   - Stores monitor configurations as JSON in `~/.config/hyprmon/profiles/`
   - Profile menu with reordering, renaming, and deletion capabilities
   - `applyProfile()`: Loads and applies saved monitor configurations
   - Custom profile ordering preserved in `.profile_order` file

4. **UI Models** (`models.go`, `view.go`):
   - Main `model` struct: Manages monitor layout UI state
   - Visual monitor representation with proportional scaling
   - Grid-based movement and snapping system
   - Mouse and keyboard input handling

5. **Advanced Settings** (`advanced_settings.go`):
   - Dialog for HDR, color management, VRR, transform settings
   - `advancedSettingsModel`: Separate Bubble Tea model for settings dialog

6. **Mode Picker** (`mode_picker.go`):
   - Resolution and refresh rate selection UI
   - Fetches available modes from Hyprland

7. **Scale Picker** (`scale_picker.go`):
   - DPI scaling selection with preset values (0.5x to 3.0x)

## Key Technical Details

### Monitor Data Flow
1. Hyprland → `hyprctl monitors -j` → `hyprMonitor` struct → `Monitor` struct (with HardwareID from EDID)
2. User edits in UI → `Monitor` struct updates → `hyprctl keyword monitor` → Hyprland (uses connector Name)
3. Save action → `Monitor` struct → Update hyprland.conf monitor lines (uses connector Name)
4. Profile save/load → JSON keyed by HardwareID for stable matching across port changes

### Profile Storage Structure
```json
{
  "name": "profile_name",
  "monitors": [...],
  "created_at": "2025-01-21T...",
  "updated_at": "2025-01-21T..."
}
```

### Hyprland Integration
- Uses `hyprctl` for all monitor operations (no direct Wayland protocol)
- Monitor command format: `monitor=NAME,WIDTHxHEIGHT@REFRESH,XxY,SCALE[,options]`
- Advanced options: bitdepth, cm (color mode), sdrbrightness, sdrsaturation, vrr, transform

### Monitor Identity Fields
- `Name`: Connector name (e.g., `DP-3`) - ephemeral, used only for hyprctl commands
- `HardwareID`: Stable EDID fingerprint (`Make/Model/Serial`) - used for profile matching
- `Make`: EDID manufacturer (e.g., "Dell Inc.")
- `Model`: EDID model name (e.g., "DELL U3419W")
- `Serial`: EDID serial number (e.g., "5HJB6T2") - may be empty
- `Alias`: Optional user-friendly display name
- `EDIDName`: Full Hyprland description string (kept for backward compat)

### UI Framework
- Built with Bubble Tea (Model-Update-View pattern)
- Lipgloss for styling and layout
- Mouse support via SGR protocol
- Responsive terminal layouts

## Testing

Run the test suite with:
```bash
make test
# Or directly with go test
go test -v ./...
```

## Monitor Active State Detection

The codebase tracks active/focused monitors through the Hyprland JSON API:
- `hyprMonitor.Focused`: Boolean indicating if a monitor has focus
- `hyprMonitor.Disabled`: Boolean indicating if a monitor is disabled
- Used when displaying profiles or current monitor status

### New Features Added

#### Active Monitor Detection
- `--active-monitor` flag: Returns the name of the currently focused monitor
- `getFocusedMonitor()` function in hyprland.go queries Hyprland and returns focused monitor name
- Useful for scripting and determining which monitor has focus

#### Active Profile Detection
- Profiles menu now shows `*` after the currently active profile name
- `--list-profiles` flag shows `*` after the active profile
- `getCurrentActiveProfile()` function compares current monitor configuration with saved profiles
- `compareMonitorConfigurations()` helper function compares monitor arrays for profile matching
- Active profile detection works by comparing resolution, position, scale, refresh rate, and active state

### EDID-Based Monitor Identification
- Monitors are identified by `HardwareID` (composite of `Make/Model/Serial` from EDID data)
- `HardwareID` is stable across port changes and reboots, unlike connector names (e.g., DP-3)
- When serial is empty (common for laptop panels): format is `Make/Model`
- Duplicate monitors (same make+model, no serial) get `/#N` suffix sorted by connector name
- `DisplayLabel()` method returns: Alias > Model > Name (for UI display)
- Profiles match by `HardwareID` when available, falling back to connector `Name` for legacy profiles
- Legacy profiles are auto-migrated with `HardwareID` data when accessed
- `resolveProfileMonitors()` maps saved HardwareIDs to current connector names when applying profiles

## Important File Paths

- Configuration: `~/.config/hypr/hyprland.conf` (or `$HYPRLAND_CONFIG`)
- Profiles: `~/.config/hyprmon/profiles/` (or custom via `--cfg` flag)
- Profile order: `~/.config/hyprmon/profiles/.profile_order`
- Config backups: `hyprland.conf.bak.<timestamp>`