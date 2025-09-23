package main

import (
	"testing"
)

func TestWouldCreateCircularMirror(t *testing.T) {
	tests := []struct {
		name             string
		currentMonitor   string
		sourceMonitor    string
		allMonitors      []Monitor
		expectedCircular bool
	}{
		{
			name:           "No circular mirror - simple case",
			currentMonitor: "HDMI-A-1",
			sourceMonitor:  "eDP-1",
			allMonitors: []Monitor{
				{Name: "HDMI-A-1", Active: true},
				{Name: "eDP-1", Active: true},
			},
			expectedCircular: false,
		},
		{
			name:           "Direct circular mirror",
			currentMonitor: "HDMI-A-1",
			sourceMonitor:  "eDP-1",
			allMonitors: []Monitor{
				{Name: "HDMI-A-1", Active: true},
				{Name: "eDP-1", Active: true, IsMirrored: true, MirrorSource: "HDMI-A-1"},
			},
			expectedCircular: true,
		},
		{
			name:           "Indirect circular mirror (3 monitors)",
			currentMonitor: "HDMI-A-1",
			sourceMonitor:  "eDP-1",
			allMonitors: []Monitor{
				{Name: "HDMI-A-1", Active: true},
				{Name: "eDP-1", Active: true, IsMirrored: true, MirrorSource: "DP-1"},
				{Name: "DP-1", Active: true, IsMirrored: true, MirrorSource: "HDMI-A-1"},
			},
			expectedCircular: true,
		},
		{
			name:           "No circular mirror - chain but not circular",
			currentMonitor: "HDMI-A-1",
			sourceMonitor:  "eDP-1",
			allMonitors: []Monitor{
				{Name: "HDMI-A-1", Active: true},
				{Name: "eDP-1", Active: true, IsMirrored: true, MirrorSource: "DP-1"},
				{Name: "DP-1", Active: true},
			},
			expectedCircular: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wouldCreateCircularMirror(tt.currentMonitor, tt.sourceMonitor, tt.allMonitors)
			if result != tt.expectedCircular {
				t.Errorf("wouldCreateCircularMirror() = %v, want %v", result, tt.expectedCircular)
			}
		})
	}
}

func TestValidateMirrorConfiguration(t *testing.T) {
	tests := []struct {
		name             string
		monitors         []Monitor
		expectedWarnings int
	}{
		{
			name: "No warnings - valid configuration",
			monitors: []Monitor{
				{Name: "HDMI-A-1", Active: true, PxW: 1920, PxH: 1080},
				{Name: "eDP-1", Active: true, PxW: 1920, PxH: 1080, IsMirrored: true, MirrorSource: "HDMI-A-1"},
			},
			expectedWarnings: 0,
		},
		{
			name: "Resolution mismatch warning",
			monitors: []Monitor{
				{Name: "HDMI-A-1", Active: true, PxW: 1920, PxH: 1080},
				{Name: "eDP-1", Active: true, PxW: 3840, PxH: 2160, IsMirrored: true, MirrorSource: "HDMI-A-1"},
			},
			expectedWarnings: 1,
		},
		{
			name: "Disabled source monitor warning",
			monitors: []Monitor{
				{Name: "HDMI-A-1", Active: false, PxW: 1920, PxH: 1080},
				{Name: "eDP-1", Active: true, PxW: 1920, PxH: 1080, IsMirrored: true, MirrorSource: "HDMI-A-1"},
			},
			expectedWarnings: 1,
		},
		{
			name: "Multiple warnings",
			monitors: []Monitor{
				{Name: "HDMI-A-1", Active: false, PxW: 1920, PxH: 1080},
				{Name: "eDP-1", Active: true, PxW: 3840, PxH: 2160, IsMirrored: true, MirrorSource: "HDMI-A-1"},
			},
			expectedWarnings: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := validateMirrorConfiguration(tt.monitors)
			if len(warnings) != tt.expectedWarnings {
				t.Errorf("validateMirrorConfiguration() returned %d warnings, want %d. Warnings: %v",
					len(warnings), tt.expectedWarnings, warnings)
			}
		})
	}
}
