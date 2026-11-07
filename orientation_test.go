package main

import "testing"

func TestToggleOrientationRoundTrip(t *testing.T) {
	m := model{
		Monitors: []Monitor{
			// Listed out of order on purpose: the laptop is on the right.
			{Name: "eDP-1", X: 2560, Y: 360, PxW: 2880, PxH: 1800, Scale: 2, Active: true},
			{Name: "DP-1", X: 0, Y: 0, PxW: 2560, PxH: 1440, Scale: 1, Active: true},
			{Name: "HDMI-1", X: 9000, Y: 9000, PxW: 1920, PxH: 1080, Scale: 1, Active: false},
		},
	}

	if got := m.toggleOrientation(); got != "vertical" {
		t.Fatalf("first toggle = %q, want vertical", got)
	}
	dp, edp := m.Monitors[1], m.Monitors[0]
	// DP-1 was on the left, so it should now be on top, centered over the laptop.
	if dp.X != 0 || dp.Y != 0 {
		t.Fatalf("DP-1 at %d,%d, want 0,0", dp.X, dp.Y)
	}
	if edp.X != (2560-1440)/2 || edp.Y != 1440 {
		t.Fatalf("eDP-1 at %d,%d, want %d,1440", edp.X, edp.Y, (2560-1440)/2)
	}
	if m.Monitors[2].X != 9000 || m.Monitors[2].Y != 9000 {
		t.Fatalf("inactive monitor moved")
	}

	if got := m.toggleOrientation(); got != "horizontal" {
		t.Fatalf("second toggle = %q, want horizontal", got)
	}
	dp, edp = m.Monitors[1], m.Monitors[0]
	if dp.X != 0 || dp.Y != 0 {
		t.Fatalf("DP-1 at %d,%d, want 0,0", dp.X, dp.Y)
	}
	if edp.X != 2560 || edp.Y != (1440-900)/2 {
		t.Fatalf("eDP-1 at %d,%d, want 2560,%d", edp.X, edp.Y, (1440-900)/2)
	}
}

func TestToggleOrientationRespectsRotation(t *testing.T) {
	m := model{
		Monitors: []Monitor{
			{Name: "A", X: 0, Y: 0, PxW: 1920, PxH: 1080, Scale: 1, Active: true},
			{Name: "B", X: 1920, Y: 0, PxW: 1920, PxH: 1080, Scale: 1, Active: true, Transform: 1},
		},
	}
	m.toggleOrientation()
	// B is rotated (1080 wide, 1920 tall) and sits below A, centered.
	if b := m.Monitors[1]; b.X != (1920-1080)/2 || b.Y != 1080 {
		t.Fatalf("B at %d,%d, want %d,1080", b.X, b.Y, (1920-1080)/2)
	}
}

func TestToggleOrientationNeedsTwoMonitors(t *testing.T) {
	m := model{Monitors: []Monitor{{Name: "A", PxW: 1920, PxH: 1080, Scale: 1, Active: true}}}
	if got := m.toggleOrientation(); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
