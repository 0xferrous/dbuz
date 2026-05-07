package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func initialModel() model {
	m := model{
		help: help.New(),
		panes: []pane{{
			title: "D-Bus",
			entries: []entry{
				{name: "session bus", busType: "session", kind: entryBus},
				{name: "system bus", busType: "system", kind: entryBus},
			},
		}},
	}
	m.log("initialized")
	return m
}

func (m *model) log(format string, args ...any) {
	m.logs = append(m.logs, logEntry{time: time.Now(), message: fmt.Sprintf(format, args...)})
	m.logScroll = max(0, len(m.logs)-m.logViewportHeight())
}

func (m model) logViewportHeight() int {
	return 6
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modal != nil {
		return m.updateModal(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			if m.conn != nil {
				m.conn.Close()
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			m.moveCursor(-1)
		case key.Matches(msg, keys.Down):
			m.moveCursor(1)
		case key.Matches(msg, keys.PageUp):
			m.pageMove(-1)
		case key.Matches(msg, keys.PageDown):
			m.pageMove(1)
		case key.Matches(msg, keys.LogUp):
			m.scrollLog(-1)
		case key.Matches(msg, keys.LogDown):
			m.scrollLog(1)
		case key.Matches(msg, keys.Enter):
			m.dive()
		case key.Matches(msg, keys.Back):
			m.back()
		}
	}

	return m, nil
}

func (m *model) activePane() *pane {
	if len(m.panes) == 0 {
		return nil
	}
	return &m.panes[len(m.panes)-1]
}

func (m *model) moveCursor(delta int) {
	p := m.activePane()
	if p == nil || len(p.entries) == 0 {
		return
	}

	p.cursor = clamp(p.cursor+delta, 0, len(p.entries)-1)
	p.ensureCursorVisible(m.pageSize())
}

func (m *model) scrollLog(delta int) {
	m.logScroll = clamp(m.logScroll+delta, 0, max(0, len(m.logs)-m.logViewportHeight()))
}

func (m *model) pageMove(direction int) {
	p := m.activePane()
	if p == nil || len(p.entries) == 0 {
		return
	}

	viewport := m.pageSize()
	cursorOffset := clamp(p.cursor-p.scroll, 0, viewport-1)
	maxScroll := max(0, len(p.entries)-viewport)
	p.scroll = clamp(p.scroll+(direction*viewport), 0, maxScroll)
	p.cursor = clamp(p.scroll+cursorOffset, 0, len(p.entries)-1)
}

func (p *pane) ensureCursorVisible(viewport int) {
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	}
	if p.cursor >= p.scroll+viewport {
		p.scroll = p.cursor - viewport + 1
	}
	p.scroll = clamp(p.scroll, 0, max(0, len(p.entries)-viewport))
}

func (m model) pageSize() int {
	paneHeight := m.paneContentHeight()
	metaHeight := 0
	if p := m.activePane(); p != nil {
		metaHeight = metadataBlockHeight(*p, paneHeight)
	}
	return max(1, paneHeight-3-metaHeight)
}

func (m model) paneContentHeight() int {
	// Rendered layout is: breadcrumb + pane borders + log borders + help.
	// lipgloss Height() is content height; borders add two rows each.
	helpHeight := lipgloss.Height(m.help.View(keys))
	return max(1, m.height-1-2-m.logViewportHeight()-2-helpHeight)
}

func (m *model) dive() {
	p := m.activePane()
	if p == nil || len(p.entries) == 0 {
		return
	}

	selected := p.entries[p.cursor]
	switch selected.kind {
	case entryBus:
		if err := m.connectBus(selected.busType); err != nil {
			m.panes = append(m.panes, pane{title: selected.name, err: err})
			return
		}
		m.panes = append(m.panes, m.readBusNamesPane(selected.busType))
	case entryBusName, entryObject:
		if m.conn == nil {
			return
		}
		m.panes = append(m.panes, m.readObjectPane(selected.busName, selected.objectPath))
	case entryInterface:
		if m.conn == nil {
			return
		}
		m.panes = append(m.panes, m.readInterfacePane(selected.busName, selected.objectPath, selected.interface_))
	case entryMethod:
		m.openCallModal(selected)
	case entryProperty:
		if m.conn == nil {
			return
		}
		m.readSelectedProperty()
	}
}

func (m *model) back() {
	if len(m.panes) > 1 {
		m.panes = m.panes[:len(m.panes)-1]
	}
	if len(m.panes) == 1 && m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
}
