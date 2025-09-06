package main

import (
	"testing"
)

func TestGetEffectiveDimensions(t *testing.T) {
	m := model{}

	tests := []struct {
		name      string
		monitor   Monitor
		expectedW int32
		expectedH int32
	}{
		{
			name: "Normal orientation (Transform 0)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 0,
			},
			expectedW: 1920,
			expectedH: 1080,
		},
		{
			name: "90 degree rotation (Transform 1)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 1,
			},
			expectedW: 1080, // Swapped
			expectedH: 1920, // Swapped
		},
		{
			name: "180 degree rotation (Transform 2)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 2,
			},
			expectedW: 1920, // Not swapped
			expectedH: 1080, // Not swapped
		},
		{
			name: "270 degree rotation (Transform 3)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 3,
			},
			expectedW: 1080, // Swapped
			expectedH: 1920, // Swapped
		},
		{
			name: "Flipped (Transform 4)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 4,
			},
			expectedW: 1920, // Not swapped
			expectedH: 1080, // Not swapped
		},
		{
			name: "Flipped + 90 degree (Transform 5)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 5,
			},
			expectedW: 1080, // Swapped
			expectedH: 1920, // Swapped
		},
		{
			name: "Flipped + 180 degree (Transform 6)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 6,
			},
			expectedW: 1920, // Not swapped
			expectedH: 1080, // Not swapped
		},
		{
			name: "Flipped + 270 degree (Transform 7)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 7,
			},
			expectedW: 1080, // Swapped
			expectedH: 1920, // Swapped
		},
		{
			name: "With scaling - Normal (Transform 0)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     2.0,
				Transform: 0,
			},
			expectedW: 960, // 1920/2
			expectedH: 540, // 1080/2
		},
		{
			name: "With scaling - 90 degree (Transform 1)",
			monitor: Monitor{
				PxW:       1920,
				PxH:       1080,
				Scale:     2.0,
				Transform: 1,
			},
			expectedW: 540, // 1080/2 (swapped)
			expectedH: 960, // 1920/2 (swapped)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := m.getEffectiveDimensions(tt.monitor)
			if w != tt.expectedW {
				t.Errorf("getEffectiveDimensions() width = %d, expected %d", w, tt.expectedW)
			}
			if h != tt.expectedH {
				t.Errorf("getEffectiveDimensions() height = %d, expected %d", h, tt.expectedH)
			}
		})
	}
}

func TestHitTestWithRotation(t *testing.T) {
	// Create a test model with fixed world dimensions
	m := model{
		World: world{
			TermW:   80,
			TermH:   24,
			OffsetX: 0,
			OffsetY: 0,
			Width:   4000, // Bigger to accommodate both monitors
			Height:  2500,
		},
		Monitors: []Monitor{
			{
				Name:      "Normal",
				X:         0,
				Y:         0,
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 0,
			},
			{
				Name:      "Rotated90",
				X:         2000,
				Y:         0,
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 1, // 90 degree rotation - effective dims should be 1080x1920
			},
		},
	}

	tests := []struct {
		name        string
		termX       int
		termY       int
		expectedHit int // -1 for no hit, 0+ for monitor index
		description string
	}{
		{
			name:        "Hit normal monitor",
			termX:       5,
			termY:       5,
			expectedHit: 0,
			description: "Should hit the normal monitor",
		},
		{
			name:        "Hit rotated monitor center",
			termX:       49,
			termY:       5,
			expectedHit: 1,
			description: "Should hit the rotated monitor (accounting for swapped dimensions)",
		},
		{
			name:        "Miss both monitors",
			termX:       5,
			termY:       20,
			expectedHit: -1,
			description: "Should not hit any monitor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit := m.hitTest(tt.termX, tt.termY)
			if hit != tt.expectedHit {
				// Debug information
				wx, wy := m.termToWorld(tt.termX, tt.termY)
				t.Logf("Debug: term(%d,%d) -> world(%d,%d)", tt.termX, tt.termY, wx, wy)
				for i, mon := range m.Monitors {
					ew, eh := m.getEffectiveDimensions(mon)
					t.Logf("Monitor %d (%s): pos(%d,%d) effectiveDim(%dx%d) transform=%d",
						i, mon.Name, mon.X, mon.Y, ew, eh, mon.Transform)
				}
				t.Errorf("hitTest(%d, %d) = %d, expected %d (%s)",
					tt.termX, tt.termY, hit, tt.expectedHit, tt.description)
			}
		})
	}
}

func TestSnapPositionWithRotation(t *testing.T) {
	// Create test model with two monitors - one normal, one rotated
	m := model{
		Snap:       SnapEdges,
		SnapThresh: 50,
		Selected:   0,
		Monitors: []Monitor{
			{
				Name:      "Moving",
				X:         0,
				Y:         0,
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 0,
				Active:    true,
			},
			{
				Name:      "Fixed-Rotated",
				X:         2000,
				Y:         100,
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 1, // 90 degree - effective dims are 1080x1920
				Active:    true,
			},
		},
	}

	tests := []struct {
		name      string
		x         int32
		y         int32
		expectedX int32
		expectedY int32
		snapType  string
	}{
		{
			name:      "Snap to right edge of rotated monitor",
			x:         3090, // Close to 2000 + 1080 = 3080 (rotated monitor's right edge)
			y:         200,
			expectedX: 3080, // Should snap to 2000 + 1080
			expectedY: 200,
			snapType:  "edge",
		},
		{
			name:      "Snap to left edge of rotated monitor",
			x:         70, // Close to 2000 - 1920 = 80 (moving monitor would be at 80)
			y:         200,
			expectedX: 80, // Should snap to 2000 - 1920 (moving monitor width)
			expectedY: 200,
			snapType:  "edge",
		},
		{
			name:      "No snap - too far",
			x:         500,
			y:         500,
			expectedX: 500, // Should not change
			expectedY: 500,
			snapType:  "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newX, newY, guides := m.snapPosition(&m.Monitors[0], tt.x, tt.y)

			if newX != tt.expectedX || newY != tt.expectedY {
				// Debug information
				t.Logf("Debug: input pos(%d,%d) -> result pos(%d,%d)", tt.x, tt.y, newX, newY)
				movingW, movingH := m.getEffectiveDimensions(m.Monitors[0])
				otherW, otherH := m.getEffectiveDimensions(m.Monitors[1])
				t.Logf("Moving monitor dims: %dx%d", movingW, movingH)
				t.Logf("Other monitor pos(%d,%d) dims: %dx%d", m.Monitors[1].X, m.Monitors[1].Y, otherW, otherH)
				t.Logf("Guides created: %d", len(guides))
			}

			if newX != tt.expectedX {
				t.Errorf("snapPosition() X = %d, expected %d", newX, tt.expectedX)
			}
			if newY != tt.expectedY {
				t.Errorf("snapPosition() Y = %d, expected %d", newY, tt.expectedY)
			}

			// Check if guides were created when expected
			if tt.snapType == "edge" && len(guides) == 0 {
				t.Errorf("Expected snap guides but none were created")
			} else if tt.snapType == "none" && len(guides) > 0 {
				t.Errorf("Expected no snap guides but %d were created", len(guides))
			}
		})
	}
}

func TestSnapPositionWithCenterAlignment(t *testing.T) {
	// Test center snapping with rotation
	m := model{
		Snap:       SnapCenters,
		SnapThresh: 50,
		Selected:   0,
		Monitors: []Monitor{
			{
				Name:      "Moving",
				X:         0,
				Y:         0,
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 0,
				Active:    true,
			},
			{
				Name:      "Fixed-Rotated",
				X:         2000,
				Y:         0,
				PxW:       1920,
				PxH:       1080,
				Scale:     1.0,
				Transform: 1, // Effective dims: 1080x1920
				Active:    true,
			},
		},
	}

	// Fixed rotated monitor center: X = 2000 + 1080/2 = 2540, Y = 100 + 1920/2 = 1060
	// Moving monitor center should align to this
	tests := []struct {
		name      string
		x         int32
		y         int32
		expectedX int32
		expectedY int32
	}{
		{
			name:      "Center align X with rotated monitor",
			x:         1590, // Close to center alignment: 2540 - 1920/2 = 1580
			y:         100,
			expectedX: 1580, // 2540 - 1920/2 (moving monitor center X alignment)
			expectedY: 100,
		},
		{
			name:      "Center align Y with rotated monitor",
			x:         100,
			y:         430, // Close to 960 - 1080/2 = 420
			expectedY: 420, // 960 - 1080/2 (moving monitor center Y alignment)
			expectedX: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newX, newY, guides := m.snapPosition(&m.Monitors[0], tt.x, tt.y)

			if newX != tt.expectedX || newY != tt.expectedY || len(guides) == 0 {
				// Debug information
				movingW, movingH := m.getEffectiveDimensions(m.Monitors[0])
				otherW, otherH := m.getEffectiveDimensions(m.Monitors[1])
				movingCenterX := tt.x + movingW/2
				movingCenterY := tt.y + movingH/2
				otherCenterX := m.Monitors[1].X + otherW/2
				otherCenterY := m.Monitors[1].Y + otherH/2

				t.Logf("Debug: input pos(%d,%d) -> result pos(%d,%d)", tt.x, tt.y, newX, newY)
				t.Logf("Moving monitor dims: %dx%d, center: (%d,%d)", movingW, movingH, movingCenterX, movingCenterY)
				t.Logf("Other monitor pos(%d,%d) dims: %dx%d, center: (%d,%d)",
					m.Monitors[1].X, m.Monitors[1].Y, otherW, otherH, otherCenterX, otherCenterY)
				t.Logf("Guides created: %d", len(guides))
			}

			if newX != tt.expectedX {
				t.Errorf("snapPosition() center align X = %d, expected %d", newX, tt.expectedX)
			}
			if newY != tt.expectedY {
				t.Errorf("snapPosition() center align Y = %d, expected %d", newY, tt.expectedY)
			}

			// Should have guides for center alignment
			if len(guides) == 0 {
				t.Errorf("Expected center alignment guides but none were created")
			}
		})
	}
}
