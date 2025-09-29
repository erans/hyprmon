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

	// Mirror settings
	IsMirrored    bool     // Whether this monitor is mirroring another
	MirrorSource  string   // Name of monitor being mirrored (empty if not mirroring)
	MirrorTargets []string // Names of monitors mirroring this one

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
	ShowModePicker       bool
	ModePicker           modePickerModel
	ShowMirrorPicker     bool
	MirrorPicker         mirrorPickerModel
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
			Width:  defaultWorldWidth,
			Height: defaultWorldHeight,
			Scale:  defaultWorldScale,
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
		Width:  maxX + worldPaddingPx,
		Height: maxY + worldPaddingPx,
		Scale:  defaultWorldScale,
	}
}

// getEffectiveDimensions returns the effective width and height considering transform rotation
func (m *model) getEffectiveDimensions(mon Monitor) (int32, int32) {
	scaledWidth := int32(float32(mon.PxW) / mon.Scale)
	scaledHeight := int32(float32(mon.PxH) / mon.Scale)

	// For 90° and 270° rotations, swap width and height
	if mon.Transform == 1 || mon.Transform == 3 || mon.Transform == 5 || mon.Transform == 7 {
		return scaledHeight, scaledWidth
	}

	return scaledWidth, scaledHeight
}

func (m *model) worldToTerm(x, y int32) (int, int) {
	// Use desktop dimensions (accounting for borders and UI elements)
	desktopWidth := m.World.TermW - desktopBorderMargin
	desktopHeight := m.World.TermH - desktopFooterHeight

	termX := int(float32(x-m.World.OffsetX) * float32(desktopWidth) / float32(m.World.Width))
	termY := int(float32(y-m.World.OffsetY) * float32(desktopHeight) / float32(m.World.Height))
	return termX, termY
}

func (m *model) termToWorld(x, y int) (int32, int32) {
	// Use desktop dimensions (accounting for borders and UI elements)
	desktopWidth := m.World.TermW - desktopBorderMargin
	desktopHeight := m.World.TermH - desktopFooterHeight

	worldX := int32(float32(x)*float32(m.World.Width)/float32(desktopWidth)) + m.World.OffsetX
	worldY := int32(float32(y)*float32(m.World.Height)/float32(desktopHeight)) + m.World.OffsetY
	return worldX, worldY
}

func (m *model) hitTest(x, y int) int {
	wx, wy := m.termToWorld(x, y)
	for i, mon := range m.Monitors {
		// Use effective dimensions considering transform rotation
		effectiveWidth, effectiveHeight := m.getEffectiveDimensions(mon)

		if wx >= mon.X && wx < mon.X+effectiveWidth &&
			wy >= mon.Y && wy < mon.Y+effectiveHeight {
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
			// Use effective dimensions considering transform rotation
			monEffectiveWidth, monEffectiveHeight := m.getEffectiveDimensions(*mon)
			otherEffectiveWidth, otherEffectiveHeight := m.getEffectiveDimensions(other)

			if abs(x-other.X-otherEffectiveWidth) < thresh {
				newX = other.X + otherEffectiveWidth
				guides = append(guides, guide{Type: "vertical", Value: newX})
			} else if abs(x+monEffectiveWidth-other.X) < thresh {
				newX = other.X - monEffectiveWidth
				guides = append(guides, guide{Type: "vertical", Value: other.X})
			} else if abs(x-other.X) < thresh {
				newX = other.X
				guides = append(guides, guide{Type: "vertical", Value: newX})
			}

			if abs(y-other.Y-otherEffectiveHeight) < thresh {
				newY = other.Y + otherEffectiveHeight
				guides = append(guides, guide{Type: "horizontal", Value: newY})
			} else if abs(y+monEffectiveHeight-other.Y) < thresh {
				newY = other.Y - monEffectiveHeight
				guides = append(guides, guide{Type: "horizontal", Value: other.Y})
			} else if abs(y-other.Y) < thresh {
				newY = other.Y
				guides = append(guides, guide{Type: "horizontal", Value: newY})
			}
		}

		if m.Snap == SnapCenters || m.Snap == SnapBoth {
			// Use effective dimensions considering transform rotation for center snapping
			monEffectiveWidth, monEffectiveHeight := m.getEffectiveDimensions(*mon)
			otherEffectiveWidth, otherEffectiveHeight := m.getEffectiveDimensions(other)

			monCenterX := x + monEffectiveWidth/2
			monCenterY := y + monEffectiveHeight/2
			otherCenterX := other.X + otherEffectiveWidth/2
			otherCenterY := other.Y + otherEffectiveHeight/2

			if abs(monCenterX-otherCenterX) < thresh {
				newX = otherCenterX - monEffectiveWidth/2
				guides = append(guides, guide{Type: "vertical", Value: otherCenterX})
			}

			if abs(monCenterY-otherCenterY) < thresh {
				newY = otherCenterY - monEffectiveHeight/2
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
