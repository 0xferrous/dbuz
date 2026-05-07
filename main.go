package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/godbus/dbus/v5"
)

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	LogUp    key.Binding
	LogDown  key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Enter, k.Back, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.PageUp, k.PageDown}, {k.LogUp, k.LogDown, k.Enter, k.Back, k.Quit}}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "ctrl+u"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", "ctrl+d"),
		key.WithHelp("pgdn", "page down"),
	),
	LogUp: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "log up"),
	),
	LogDown: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "log down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("right", "l", "enter"),
		key.WithHelp("→/l", "dive"),
	),
	Back: key.NewBinding(
		key.WithKeys("left", "h", "backspace"),
		key.WithHelp("←/h", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

type entryKind int

const (
	entryBus entryKind = iota
	entryBusName
	entryObject
	entryInterface
	entryMethod
	entryProperty
	entrySignal
)

type entry struct {
	name        string
	detail      string
	path        string
	busName     string
	objectPath  string
	interface_  string
	busType     string
	kind        entryKind
	args        []introspectArg
	annotations []introspectAnnotation
}

type pane struct {
	title   string
	entries []entry
	cursor  int
	scroll  int
	err     error
}

type logEntry struct {
	time    time.Time
	message string
}

type model struct {
	help      help.Model
	width     int
	height    int
	conn      *dbus.Conn
	panes     []pane
	logs      []logEntry
	logScroll int
}

type introspectNode struct {
	XMLName    xml.Name              `xml:"node"`
	Name       string                `xml:"name,attr"`
	Nodes      []introspectChildNode `xml:"node"`
	Interfaces []introspectInterface `xml:"interface"`
}

type introspectChildNode struct {
	Name string `xml:"name,attr"`
}

type introspectInterface struct {
	Name        string                 `xml:"name,attr"`
	Methods     []introspectMember     `xml:"method"`
	Signals     []introspectMember     `xml:"signal"`
	Properties  []introspectProperty   `xml:"property"`
	Annotations []introspectAnnotation `xml:"annotation"`
}

type introspectMember struct {
	Name        string                 `xml:"name,attr"`
	Args        []introspectArg        `xml:"arg"`
	Annotations []introspectAnnotation `xml:"annotation"`
}

type introspectArg struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	Direction string `xml:"direction,attr"`
}

type introspectProperty struct {
	Name        string                 `xml:"name,attr"`
	Type        string                 `xml:"type,attr"`
	Access      string                 `xml:"access,attr"`
	Annotations []introspectAnnotation `xml:"annotation"`
}

type introspectAnnotation struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

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

func (m *model) connectBus(busType string) error {
	m.log("connect %s bus", busType)
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}

	var (
		conn *dbus.Conn
		err  error
	)
	if busType == "system" {
		conn, err = dbus.ConnectSystemBus()
	} else {
		conn, err = dbus.ConnectSessionBus()
	}
	if err != nil {
		m.log("connect %s bus error: %v", busType, err)
		return err
	}

	m.conn = conn
	m.log("connected %s bus", busType)
	return nil
}

func (m *model) readBusNamesPane(busType string) pane {
	p := pane{title: busType + " bus"}
	obj := m.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")

	m.log("call org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.ListNames")
	var names []string
	if err := obj.Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		m.log("error org.freedesktop.DBus.ListNames: %v", err)
		p.err = err
		return p
	}

	m.log("reply org.freedesktop.DBus.ListNames: %d names", len(names))
	sort.Strings(names)
	p.entries = make([]entry, 0, len(names))
	for _, name := range names {
		p.entries = append(p.entries, entry{
			name:       name,
			path:       name,
			busName:    name,
			objectPath: "/",
			kind:       entryBusName,
		})
	}
	return p
}

func (m *model) readObjectPane(busName, objectPath string) pane {
	p := pane{title: objectPath}
	node, err := m.introspect(busName, objectPath)
	if err != nil {
		p.err = err
		return p
	}

	for _, child := range node.Nodes {
		childPath := joinObjectPath(objectPath, child.Name)
		p.entries = append(p.entries, entry{
			name:       child.Name,
			path:       busName + childPath,
			busName:    busName,
			objectPath: childPath,
			kind:       entryObject,
		})
	}

	for _, iface := range node.Interfaces {
		p.entries = append(p.entries, entry{
			name:        iface.Name,
			detail:      fmt.Sprintf("%d methods, %d properties, %d signals", len(iface.Methods), len(iface.Properties), len(iface.Signals)),
			path:        busName + objectPath + "#" + iface.Name,
			busName:     busName,
			objectPath:  objectPath,
			interface_:  iface.Name,
			kind:        entryInterface,
			annotations: iface.Annotations,
		})
	}

	sortEntries(p.entries)
	return p
}

func (m *model) readInterfacePane(busName, objectPath, interfaceName string) pane {
	p := pane{title: interfaceName}
	node, err := m.introspect(busName, objectPath)
	if err != nil {
		p.err = err
		return p
	}

	var iface introspectInterface
	found := false
	for _, candidate := range node.Interfaces {
		if candidate.Name == interfaceName {
			iface = candidate
			found = true
			break
		}
	}
	if !found {
		p.err = fmt.Errorf("interface %q not found", interfaceName)
		return p
	}

	for _, method := range iface.Methods {
		p.entries = append(p.entries, entry{
			name:        method.Name + memberSignature(method.Args),
			detail:      argsDetail(method.Args),
			kind:        entryMethod,
			args:        method.Args,
			annotations: method.Annotations,
		})
	}
	for _, signal := range iface.Signals {
		p.entries = append(p.entries, entry{
			name:        signal.Name + signalSignature(signal.Args),
			detail:      argsDetail(signal.Args),
			kind:        entrySignal,
			args:        signal.Args,
			annotations: signal.Annotations,
		})
	}
	for _, prop := range iface.Properties {
		p.entries = append(p.entries, entry{
			name:        prop.Name,
			detail:      fmt.Sprintf("%s %s", prop.Type, prop.Access),
			kind:        entryProperty,
			annotations: prop.Annotations,
		})
	}

	sortEntries(p.entries)
	return p
}

func (m *model) introspect(busName, objectPath string) (introspectNode, error) {
	var xmlText string
	obj := m.conn.Object(busName, dbus.ObjectPath(objectPath))
	m.log("call %s %s org.freedesktop.DBus.Introspectable.Introspect", busName, objectPath)
	if err := obj.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xmlText); err != nil {
		m.log("error introspect %s %s: %v", busName, objectPath, err)
		return introspectNode{}, err
	}

	var node introspectNode
	if err := xml.Unmarshal([]byte(xmlText), &node); err != nil {
		m.log("error parse introspection %s %s: %v", busName, objectPath, err)
		return introspectNode{}, err
	}
	m.log("reply introspect %s %s: %d nodes, %d interfaces", busName, objectPath, len(node.Nodes), len(node.Interfaces))
	return node, nil
}

func sortEntries(entries []entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].kind != entries[j].kind {
			return entries[i].kind < entries[j].kind
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})
}

func memberSignature(args []introspectArg) string {
	in := make([]string, 0)
	out := make([]string, 0)
	for _, arg := range args {
		if arg.Direction == "out" {
			out = append(out, arg.Type)
			continue
		}
		in = append(in, arg.Type)
	}

	sig := "(" + strings.Join(in, ", ") + ")"
	if len(out) > 0 {
		sig += " → " + strings.Join(out, ", ")
	}
	return sig
}

func signalSignature(args []introspectArg) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, arg.Type)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func argsDetail(args []introspectArg) string {
	if len(args) == 0 {
		return ""
	}

	parts := make([]string, 0, len(args))
	for _, arg := range args {
		name := arg.Name
		if name == "" {
			name = "_"
		}
		direction := arg.Direction
		if direction == "" {
			direction = "in"
		}
		parts = append(parts, fmt.Sprintf("%s %s:%s", direction, name, arg.Type))
	}
	return strings.Join(parts, ", ")
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	// Pane body height minus header and separator, matching renderPane's viewport.
	return max(1, m.height-7)
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

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	logHeight := m.logViewportHeight()
	availableHeight := max(1, m.height-logHeight-5)
	visible := visiblePanes(m.panes, m.width)

	rendered := make([]string, 0, len(visible))
	for i, p := range visible {
		active := i == len(visible)-1
		rendered = append(rendered, renderPane(p, paneDisplayWidth(p, m.width), availableHeight, active))
	}

	tree := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	path := ""
	if p := m.activePane(); p != nil {
		path = p.title
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("dbus-debug")+" "+statusStyle.Render(path),
		tree,
		renderLogPane(m.logs, m.logScroll, m.width, logHeight),
		m.help.View(keys),
	)
}

func renderLogPane(logs []logEntry, scroll, width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A")).
		Width(max(24, width-2)).
		Height(height)

	viewport := max(1, height-2)
	start := clamp(scroll, 0, max(0, len(logs)-viewport))
	end := min(len(logs), start+viewport)
	lines := []string{lipgloss.NewStyle().Bold(true).Render("log")}
	for _, entry := range logs[start:end] {
		line := fmt.Sprintf("%s %s", entry.time.Format("15:04:05.000"), entry.message)
		lines = append(lines, truncate(line, max(1, width-4)))
	}
	return style.Render(strings.Join(lines, "\n"))
}

func renderPane(p pane, width, height int, active bool) string {
	borderColor := lipgloss.Color("#444444")
	if active {
		borderColor = lipgloss.Color("#7D56F4")
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width).
		Height(height)

	header := lipgloss.NewStyle().Bold(true).Render(truncate(p.title, width-4))
	lines := []string{header, strings.Repeat("─", max(0, width-4))}

	if p.err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(p.err.Error()))
	} else if len(p.entries) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render("empty"))
	} else {
		viewport := max(1, height-3)
		start := clamp(p.scroll, 0, max(0, len(p.entries)-viewport))
		end := min(len(p.entries), start+viewport)
		for i := start; i < end; i++ {
			lines = append(lines, renderEntry(p.entries[i], i == p.cursor && active, width-4))
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

func renderEntry(e entry, selected bool, width int) string {
	prefix := kindIcon(e.kind) + e.name
	if e.detail == "" {
		line := truncate(prefix, width)
		if selected {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(lipgloss.Color("#CBA6F7")).Width(width).Render(line)
		}
		return lipgloss.NewStyle().Foreground(kindColor(e.kind)).Render(line)
	}

	separator := "  "
	prefixWidth := textWidth(prefix)
	separatorWidth := textWidth(separator)
	if prefixWidth+separatorWidth >= width {
		line := truncate(prefix, width)
		if selected {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(lipgloss.Color("#CBA6F7")).Width(width).Render(line)
		}
		return lipgloss.NewStyle().Foreground(kindColor(e.kind)).Render(line)
	}

	detail := truncate(e.detail, width-prefixWidth-separatorWidth)
	if selected {
		line := prefix + separator + detail
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(lipgloss.Color("#CBA6F7")).Width(width).Render(line)
	}

	return lipgloss.NewStyle().Foreground(kindColor(e.kind)).Render(prefix) + separator + lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Render(detail)
}

func kindIcon(kind entryKind) string {
	switch kind {
	case entryBus:
		return "◉ "
	case entryBusName:
		return "● "
	case entryObject:
		return "▸ "
	case entryInterface:
		return "◇ "
	case entryMethod:
		return "ƒ "
	case entryProperty:
		return "= "
	case entrySignal:
		return "↯ "
	default:
		return "  "
	}
}

func kindColor(kind entryKind) lipgloss.Color {
	switch kind {
	case entryBus:
		return lipgloss.Color("#F5C2E7")
	case entryBusName:
		return lipgloss.Color("#A6E3A1")
	case entryObject:
		return lipgloss.Color("#89B4FA")
	case entryInterface:
		return lipgloss.Color("#F9E2AF")
	case entryMethod:
		return lipgloss.Color("#CBA6F7")
	case entryProperty:
		return lipgloss.Color("#94E2D5")
	case entrySignal:
		return lipgloss.Color("#FAB387")
	default:
		return lipgloss.Color("#CDD6F4")
	}
}

func joinObjectPath(parent, child string) string {
	if parent == "/" {
		return "/" + child
	}
	return parent + "/" + child
}

func paneDisplayWidth(p pane, terminalWidth int) int {
	contentWidth := textWidth(p.title)
	for _, entry := range p.entries {
		line := kindIcon(entry.kind) + entry.name
		if entry.detail != "" {
			line += "  " + entry.detail
		}
		contentWidth = max(contentWidth, textWidth(line))
	}

	// Add room for borders/padding, then clamp so very long D-Bus names do not
	// monopolize the screen. This keeps panes compact by default while still
	// fitting the largest visible item when practical.
	return clamp(contentWidth+4, 24, max(24, min(56, terminalWidth-2)))
}

func visiblePanes(panes []pane, width int) []pane {
	if len(panes) == 0 {
		return panes
	}

	used := 0
	start := len(panes) - 1
	for ; start >= 0; start-- {
		paneWidth := paneDisplayWidth(panes[start], width)
		if used > 0 && used+paneWidth > width-2 {
			break
		}
		used += paneWidth
	}

	return panes[start+1:]
}

func truncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func textWidth(s string) int {
	return len([]rune(s))
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	if _, err := tea.NewProgram(initialModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
