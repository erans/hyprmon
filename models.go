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
	Width     int32
	Height    int32
	TermW     int
	TermH     int
	Scale     float32
	OffsetX   int32
	OffsetY   int32
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
	ShowScalePicker bool
	ScalePicker     scalePickerModel
	ShowProfileInput bool
	ProfileInput    profileInputModel
}

type initMsg struct {
	monitors []Monitor
	err      error
}

type pollMsg struct{}

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
		if mon.X+int32(mon.PxW) > maxX {
			maxX = mon.X + int32(mon.PxW)
		}
		if mon.Y+int32(mon.PxH) > maxY {
			maxY = mon.Y + int32(mon.PxH)
		}
	}

	m.World = world{
		Width:  maxX + 500,
		Height: maxY + 500,
		Scale:  1.0,
	}
}

func (m *model) worldToTerm(x, y int32) (int, int) {
	termX := int(float32(x-m.World.OffsetX) * float32(m.World.TermW) / float32(m.World.Width))
	termY := int(float32(y-m.World.OffsetY) * float32(m.World.TermH) / float32(m.World.Height))
	return termX, termY
}

func (m *model) termToWorld(x, y int) (int32, int32) {
	worldX := int32(float32(x)*float32(m.World.Width)/float32(m.World.TermW)) + m.World.OffsetX
	worldY := int32(float32(y)*float32(m.World.Height)/float32(m.World.TermH)) + m.World.OffsetY
	return worldX, worldY
}

func (m *model) hitTest(x, y int) int {
	wx, wy := m.termToWorld(x, y)
	for i, mon := range m.Monitors {
		if wx >= mon.X && wx < mon.X+int32(mon.PxW) &&
			wy >= mon.Y && wy < mon.Y+int32(mon.PxH) {
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
			if abs(x-other.X-int32(other.PxW)) < thresh {
				newX = other.X + int32(other.PxW)
				guides = append(guides, guide{Type: "vertical", Value: newX})
			} else if abs(x+int32(mon.PxW)-other.X) < thresh {
				newX = other.X - int32(mon.PxW)
				guides = append(guides, guide{Type: "vertical", Value: other.X})
			} else if abs(x-other.X) < thresh {
				newX = other.X
				guides = append(guides, guide{Type: "vertical", Value: newX})
			}
			
			if abs(y-other.Y-int32(other.PxH)) < thresh {
				newY = other.Y + int32(other.PxH)
				guides = append(guides, guide{Type: "horizontal", Value: newY})
			} else if abs(y+int32(mon.PxH)-other.Y) < thresh {
				newY = other.Y - int32(mon.PxH)
				guides = append(guides, guide{Type: "horizontal", Value: other.Y})
			} else if abs(y-other.Y) < thresh {
				newY = other.Y
				guides = append(guides, guide{Type: "horizontal", Value: newY})
			}
		}
		
		if m.Snap == SnapCenters || m.Snap == SnapBoth {
			monCenterX := x + int32(mon.PxW)/2
			monCenterY := y + int32(mon.PxH)/2
			otherCenterX := other.X + int32(other.PxW)/2
			otherCenterY := other.Y + int32(other.PxH)/2
			
			if abs(monCenterX-otherCenterX) < thresh {
				newX = otherCenterX - int32(mon.PxW)/2
				guides = append(guides, guide{Type: "vertical", Value: otherCenterX})
			}
			
			if abs(monCenterY-otherCenterY) < thresh {
				newY = otherCenterY - int32(mon.PxH)/2
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