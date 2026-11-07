package main

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

type Mode struct {
	W  uint32
	H  uint32
	Hz float32
}

type Monitor struct {
	Name          string `json:"name"`
	HardwareID    string `json:"hardware_id,omitempty"`
	Alias         string `json:"alias,omitempty"`
	Make          string `json:"make,omitempty"`
	Model         string `json:"model,omitempty"`
	Serial        string `json:"serial,omitempty"`
	UseDescFormat bool   `json:"use_desc_format,omitempty"`
	PxW           uint32
	PxH           uint32
	Hz            float32
	Scale         float32
	X             int32
	Y             int32
	Active        bool
	EDIDName      string
	Modes         []Mode

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
	termW := m.World.TermW
	termH := m.World.TermH

	if len(m.Monitors) == 0 {
		m.World = world{
			Width:  defaultWorldWidth,
			Height: defaultWorldHeight,
			TermW:  termW,
			TermH:  termH,
			Scale:  defaultWorldScale,
		}
		return
	}

	firstWidth, firstHeight := m.getEffectiveDimensions(m.Monitors[0])
	minX := m.Monitors[0].X
	minY := m.Monitors[0].Y
	maxX := m.Monitors[0].X + firstWidth
	maxY := m.Monitors[0].Y + firstHeight

	for _, mon := range m.Monitors[1:] {
		width, height := m.getEffectiveDimensions(mon)

		if mon.X < minX {
			minX = mon.X
		}
		if mon.Y < minY {
			minY = mon.Y
		}
		if mon.X+width > maxX {
			maxX = mon.X + width
		}
		if mon.Y+height > maxY {
			maxY = mon.Y + height
		}
	}

	m.World = world{
		Width:   maxX - minX + worldPaddingPx*2,
		Height:  maxY - minY + worldPaddingPx*2,
		TermW:   termW,
		TermH:   termH,
		Scale:   defaultWorldScale,
		OffsetX: minX - worldPaddingPx,
		OffsetY: minY - worldPaddingPx,
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
	m.updateWorld()
}

func (m *model) moveSelected(dx, dy int32) {
	if m.Selected < 0 || m.Selected >= len(m.Monitors) {
		return
	}

	mon := &m.Monitors[m.Selected]
	mon.X += dx
	mon.Y += dy
	if m.Snap != SnapOff {
		mon.X, mon.Y, m.Guides = m.snapPosition(mon, mon.X, mon.Y)
	} else {
		m.Guides = nil
	}
	m.updateWorld()
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

// toggleOrientation switches the active monitors between a side-by-side
// (horizontal) row and a stacked (vertical) column. Monitor order is kept
// (left→right becomes top→bottom and vice versa), each monitor is centered on
// the cross axis, and the arrangement stays anchored at the original top-left
// corner. Inactive and mirrored monitors are left untouched. It returns the
// new orientation ("vertical" or "horizontal"), or "" if fewer than two
// monitors take part.
func (m *model) toggleOrientation() string {
	var idx []int
	for i, mon := range m.Monitors {
		if mon.Active && !mon.IsMirrored {
			idx = append(idx, i)
		}
	}
	if len(idx) < 2 {
		return ""
	}

	// Measure the spread of monitor centers on each axis to decide which way
	// they are laid out now.
	var minCX, maxCX, minCY, maxCY, minX, minY int32
	for n, i := range idx {
		mon := m.Monitors[i]
		w, h := m.getEffectiveDimensions(mon)
		cx, cy := mon.X+w/2, mon.Y+h/2
		if n == 0 {
			minCX, maxCX, minCY, maxCY = cx, cx, cy, cy
			minX, minY = mon.X, mon.Y
			continue
		}
		minCX, maxCX = min32(minCX, cx), max(maxCX, cx)
		minCY, maxCY = min32(minCY, cy), max(maxCY, cy)
		minX, minY = min32(minX, mon.X), min32(minY, mon.Y)
	}
	toVertical := maxCX-minCX >= maxCY-minCY

	// Keep the existing order along the current main axis.
	sort.SliceStable(idx, func(a, b int) bool {
		ma, mb := m.Monitors[idx[a]], m.Monitors[idx[b]]
		if toVertical {
			return ma.X < mb.X
		}
		return ma.Y < mb.Y
	})

	// The widest (for a column) or tallest (for a row) monitor sets the
	// cross-axis span that the others are centered within.
	var span int32
	for _, i := range idx {
		w, h := m.getEffectiveDimensions(m.Monitors[i])
		if toVertical {
			span = max(span, w)
		} else {
			span = max(span, h)
		}
	}

	var offset int32
	for _, i := range idx {
		mon := &m.Monitors[i]
		w, h := m.getEffectiveDimensions(*mon)
		if toVertical {
			mon.X = minX + (span-w)/2
			mon.Y = minY + offset
			offset += h
		} else {
			mon.X = minX + offset
			mon.Y = minY + (span-h)/2
			offset += w
		}
	}

	m.Guides = nil
	m.updateWorld()
	if toVertical {
		return "vertical"
	}
	return "horizontal"
}

// min32 exists because view.go declares an int-only min that shadows the builtin.
func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
