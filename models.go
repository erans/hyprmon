package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Mode struct {
	W  uint32
	H  uint32
	Hz float32
}

type Monitor struct {
	Name     string
	PxW      uint32
	PxH      uint32
	Hz       float32
	Scale    float32
	X        int32
	Y        int32
	Active   bool
	EDIDName string
	Modes    []Mode

	// Advanced display settings
	BitDepth      uint8   // 8 or 10
	ColorMode     string  // "auto", "srgb", "wide", "edid", "hdr", "hdredid"
	SDRBrightness float32 // 1.0 default, typically 1.0-2.0
	SDRSaturation float32 // 1.0 default
	VRR           int     // 0=off, 1=on, 2=fullscreen-only
	Transform     int     // 0-7 for rotation/flip

	Dragging bool
	DragOffX int32
	DragOffY int32
}

type SnapMode int

const (
	SnapOff SnapMode = iota
	SnapEdges
	SnapCenters
	SnapBoth
)

type world struct {
	Width   int32
	Height  int32
	TermW   int
	TermH   int
	Scale   float32
	OffsetX int32
	OffsetY int32
}

type guide struct {
	Type  string
	Value int32
}

type model struct {
	Monitors    []Monitor
	Selected    int
	GridPx      int
	Snap        SnapMode
	SnapThresh  int
	World       world
	Guides      []guide
	ProfileName string
	Status      string
	MouseX      int
	MouseY      int
	LastMouseX  int
	LastMouseY  int

	// Sub-views
	ShowScalePicker      bool
	ScalePicker          scalePickerModel
	ShowProfileInput     bool
	ProfileInput         profileInputModel
	ShowHelp             bool
	HelpScrollOffset     int  // Scroll position for help screen
	OpenProfiles         bool // Flag to open profiles page
	ShowAdvancedSettings bool
	AdvancedSettings     advancedSettingsModel

	// Monitor tracking for workspace migration
	PreviousMonitorNames []string
}

type initMsg struct {
	monitors []Monitor
	err      error
}

type applyMsg struct {
	success bool
	err     error
}

type saveMsg struct {
	success bool
	err     error
}

type revertMsg struct {
	success bool
	err     error
}

func (m *model) updateWorld() {
	if len(m.Monitors) == 0 {
		m.World = world{
			Width:  3840,
			Height: 2160,
			Scale:  1.0,
		}
		return
	}

	var maxX, maxY int32
	for _, mon := range m.Monitors {
		// Use scaled dimensions for world bounds
		scaledWidth := int32(float32(mon.PxW) / mon.Scale)
		scaledHeight := int32(float32(mon.PxH) / mon.Scale)

		if mon.X+scaledWidth > maxX {
			maxX = mon.X + scaledWidth
		}
		if mon.Y+scaledHeight > maxY {
			maxY = mon.Y + scaledHeight
		}
	}

	m.World = world{
		Width:  maxX + 500,
		Height: maxY + 500,
		Scale:  1.0,
	}
}

func (m *model) worldToTerm(x, y int32) (int, int) {
	// Use desktop dimensions (accounting for borders and UI elements)
	desktopWidth := m.World.TermW - 3   // Border (2) + margin (1)
	desktopHeight := m.World.TermH - 10 // Updated for 3-line footer

	termX := int(float32(x-m.World.OffsetX) * float32(desktopWidth) / float32(m.World.Width))
	termY := int(float32(y-m.World.OffsetY) * float32(desktopHeight) / float32(m.World.Height))
	return termX, termY
}

func (m *model) termToWorld(x, y int) (int32, int32) {
	// Use desktop dimensions (accounting for borders and UI elements)
	desktopWidth := m.World.TermW - 3   // Border (2) + margin (1)
	desktopHeight := m.World.TermH - 10 // Updated for 3-line footer

	worldX := int32(float32(x)*float32(m.World.Width)/float32(desktopWidth)) + m.World.OffsetX
	worldY := int32(float32(y)*float32(m.World.Height)/float32(desktopHeight)) + m.World.OffsetY
	return worldX, worldY
}

func (m *model) hitTest(x, y int) int {
	wx, wy := m.termToWorld(x, y)
	for i, mon := range m.Monitors {
		// Use scaled dimensions for hit testing
		scaledWidth := int32(float32(mon.PxW) / mon.Scale)
		scaledHeight := int32(float32(mon.PxH) / mon.Scale)

		if wx >= mon.X && wx < mon.X+scaledWidth &&
			wy >= mon.Y && wy < mon.Y+scaledHeight {
			return i
		}
	}
	return -1
}

func (m *model) beginDrag(msg tea.MouseMsg) {
	if m.Selected < 0 || m.Selected >= len(m.Monitors) {
		return
	}

	mon := &m.Monitors[m.Selected]
	wx, wy := m.termToWorld(msg.X, msg.Y)
	mon.Dragging = true
	mon.DragOffX = wx - mon.X
	mon.DragOffY = wy - mon.Y
}

func (m *model) dragMove(msg tea.MouseMsg) {
	if m.Selected < 0 || m.Selected >= len(m.Monitors) {
		return
	}

	mon := &m.Monitors[m.Selected]
	if !mon.Dragging {
		return
	}

	wx, wy := m.termToWorld(msg.X, msg.Y)
	newX := wx - mon.DragOffX
	newY := wy - mon.DragOffY

	if m.GridPx > 1 {
		newX = (newX / int32(m.GridPx)) * int32(m.GridPx)
		newY = (newY / int32(m.GridPx)) * int32(m.GridPx)
	}

	if m.Snap != SnapOff {
		newX, newY, m.Guides = m.snapPosition(mon, newX, newY)
	}

	mon.X = newX
	mon.Y = newY
}

func (m *model) endDrag() {
	if m.Selected < 0 || m.Selected >= len(m.Monitors) {
		return
	}

	mon := &m.Monitors[m.Selected]
	mon.Dragging = false
	m.Guides = nil
}

func (m *model) snapPosition(mon *Monitor, x, y int32) (int32, int32, []guide) {
	guides := []guide{}
	newX, newY := x, y
	thresh := int32(m.SnapThresh)

	for i, other := range m.Monitors {
		if i == m.Selected || !other.Active {
			continue
		}

		if m.Snap == SnapEdges || m.Snap == SnapBoth {
			// Use scaled dimensions for snapping
			monScaledWidth := int32(float32(mon.PxW) / mon.Scale)
			monScaledHeight := int32(float32(mon.PxH) / mon.Scale)
			otherScaledWidth := int32(float32(other.PxW) / other.Scale)
			otherScaledHeight := int32(float32(other.PxH) / other.Scale)

			if abs(x-other.X-otherScaledWidth) < thresh {
				newX = other.X + otherScaledWidth
				guides = append(guides, guide{Type: "vertical", Value: newX})
			} else if abs(x+monScaledWidth-other.X) < thresh {
				newX = other.X - monScaledWidth
				guides = append(guides, guide{Type: "vertical", Value: other.X})
			} else if abs(x-other.X) < thresh {
				newX = other.X
				guides = append(guides, guide{Type: "vertical", Value: newX})
			}

			if abs(y-other.Y-otherScaledHeight) < thresh {
				newY = other.Y + otherScaledHeight
				guides = append(guides, guide{Type: "horizontal", Value: newY})
			} else if abs(y+monScaledHeight-other.Y) < thresh {
				newY = other.Y - monScaledHeight
				guides = append(guides, guide{Type: "horizontal", Value: other.Y})
			} else if abs(y-other.Y) < thresh {
				newY = other.Y
				guides = append(guides, guide{Type: "horizontal", Value: newY})
			}
		}

		if m.Snap == SnapCenters || m.Snap == SnapBoth {
			// Use scaled dimensions for center snapping
			monScaledWidth := int32(float32(mon.PxW) / mon.Scale)
			monScaledHeight := int32(float32(mon.PxH) / mon.Scale)
			otherScaledWidth := int32(float32(other.PxW) / other.Scale)
			otherScaledHeight := int32(float32(other.PxH) / other.Scale)

			monCenterX := x + monScaledWidth/2
			monCenterY := y + monScaledHeight/2
			otherCenterX := other.X + otherScaledWidth/2
			otherCenterY := other.Y + otherScaledHeight/2

			if abs(monCenterX-otherCenterX) < thresh {
				newX = otherCenterX - monScaledWidth/2
				guides = append(guides, guide{Type: "vertical", Value: otherCenterX})
			}

			if abs(monCenterY-otherCenterY) < thresh {
				newY = otherCenterY - monScaledHeight/2
				guides = append(guides, guide{Type: "horizontal", Value: otherCenterY})
			}
		}
	}

	if abs(x) < thresh {
		newX = 0
		guides = append(guides, guide{Type: "vertical", Value: 0})
	}
	if abs(y) < thresh {
		newY = 0
		guides = append(guides, guide{Type: "horizontal", Value: 0})
	}

	return newX, newY, guides
}

func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func (m *model) countActiveMonitors() int {
	count := 0
	for _, mon := range m.Monitors {
		if mon.Active {
			count++
		}
	}
	return count
}

func (m *model) canDisableMonitor(index int) bool {
	if index < 0 || index >= len(m.Monitors) {
		return false
	}

	if !m.Monitors[index].Active {
		return true
	}

	return m.countActiveMonitors() > 1
}
