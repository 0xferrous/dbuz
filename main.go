package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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
	value       string
	memberName  string
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

type callModal struct {
	method         entry
	inputs         []textinput.Model
	focus          int
	response       []string
	responseScroll int
	err            string
}

type model struct {
	help      help.Model
	width     int
	height    int
	conn      *dbus.Conn
	panes     []pane
	logs      []logEntry
	logScroll int
	modal     *callModal
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
	activatable := m.activatableNames()
	owners := m.busNameOwners(names)
	wellKnownByOwner := wellKnownNamesByOwner(owners)
	sort.Strings(names)
	p.entries = make([]entry, 0, len(names))
	for _, name := range names {
		displayName := busNameDisplayName(name, owners, wellKnownByOwner)
		p.entries = append(p.entries, entry{
			name:       displayName,
			detail:     m.busNameMetadata(name, activatable, owners, wellKnownByOwner),
			path:       name,
			busName:    name,
			objectPath: "/",
			kind:       entryBusName,
		})
	}
	return p
}

func (m *model) activatableNames() map[string]bool {
	obj := m.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	m.log("call org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.ListActivatableNames")
	var names []string
	if err := obj.Call("org.freedesktop.DBus.ListActivatableNames", 0).Store(&names); err != nil {
		m.log("error org.freedesktop.DBus.ListActivatableNames: %v", err)
		return nil
	}
	m.log("reply org.freedesktop.DBus.ListActivatableNames: %d names", len(names))
	activatable := make(map[string]bool, len(names))
	for _, name := range names {
		activatable[name] = true
	}
	return activatable
}

func (m *model) busNameOwners(names []string) map[string]string {
	owners := make(map[string]string, len(names))
	obj := m.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	for _, name := range names {
		if strings.HasPrefix(name, ":") {
			owners[name] = name
			continue
		}
		m.log("call org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.GetNameOwner %s", name)
		var owner string
		if err := obj.Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner); err != nil {
			m.log("error org.freedesktop.DBus.GetNameOwner %s: %v", name, err)
			continue
		}
		owners[name] = owner
		m.log("reply org.freedesktop.DBus.GetNameOwner %s: %s", name, owner)
	}
	return owners
}

func wellKnownNamesByOwner(owners map[string]string) map[string][]string {
	byOwner := make(map[string][]string)
	for name, owner := range owners {
		if strings.HasPrefix(name, ":") || owner == "" {
			continue
		}
		byOwner[owner] = append(byOwner[owner], name)
	}
	for owner := range byOwner {
		sort.Strings(byOwner[owner])
	}
	return byOwner
}

func busNameDisplayName(name string, owners map[string]string, wellKnownByOwner map[string][]string) string {
	if strings.HasPrefix(name, ":") {
		wellKnown := wellKnownByOwner[name]
		if len(wellKnown) == 0 {
			return name
		}
		return fmt.Sprintf("%s [%s]", name, truncate(strings.Join(wellKnown, ", "), 32))
	}
	if owner := owners[name]; owner != "" {
		return fmt.Sprintf("%s [%s]", name, owner)
	}
	return name
}

func (m *model) busNameMetadata(name string, activatable map[string]bool, owners map[string]string, wellKnownByOwner map[string][]string) string {
	lines := []string{"kind: " + busNameKind(name)}
	if activatable[name] {
		lines = append(lines, "activatable: yes")
	}

	obj := m.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	if strings.HasPrefix(name, ":") {
		wellKnown := wellKnownByOwner[name]
		if len(wellKnown) > 0 {
			lines = append(lines, "well-known names:")
			for _, ownedName := range wellKnown {
				lines = append(lines, "  "+ownedName)
			}
		}
	} else {
		if owner := owners[name]; owner != "" {
			lines = append(lines, "owner: "+owner)
		} else {
			lines = append(lines, "owner: <none>")
		}
	}

	var pid uint32
	m.log("call org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.GetConnectionUnixProcessID %s", name)
	if err := obj.Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name).Store(&pid); err != nil {
		m.log("error org.freedesktop.DBus.GetConnectionUnixProcessID %s: %v", name, err)
	} else {
		lines = append(lines, "pid: "+formatPID(pid))
		m.log("reply org.freedesktop.DBus.GetConnectionUnixProcessID %s: %d", name, pid)
	}

	var uid uint32
	m.log("call org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.GetConnectionUnixUser %s", name)
	if err := obj.Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, name).Store(&uid); err != nil {
		m.log("error org.freedesktop.DBus.GetConnectionUnixUser %s: %v", name, err)
	} else {
		lines = append(lines, "uid: "+formatUID(uid))
		m.log("reply org.freedesktop.DBus.GetConnectionUnixUser %s: %d", name, uid)
	}

	return strings.Join(lines, "\n")
}

func formatPID(pid uint32) string {
	pidText := fmt.Sprint(pid)
	exe, err := os.Readlink(filepath.Join("/proc", pidText, "exe"))
	if err != nil || exe == "" {
		return pidText
	}
	return fmt.Sprintf("%s (%s %s)", pidText, filepath.Base(exe), shortenMiddle(exe, 48))
}

func shortenMiddle(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return "..."
	}
	prefix := (width - 3) / 2
	suffix := width - 3 - prefix
	return string(runes[:prefix]) + "..." + string(runes[len(runes)-suffix:])
}

func formatUID(uid uint32) string {
	uidText := fmt.Sprint(uid)
	userInfo, err := user.LookupId(uidText)
	if err != nil || userInfo.Username == "" {
		return uidText
	}
	return fmt.Sprintf("%s (%s)", uidText, userInfo.Username)
}

func busNameKind(name string) string {
	if strings.HasPrefix(name, ":") {
		return "unique"
	}
	return "well-known"
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
			memberName:  method.Name,
			detail:      argsDetail(method.Args),
			busName:     busName,
			objectPath:  objectPath,
			interface_:  interfaceName,
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
			busName:     busName,
			objectPath:  objectPath,
			interface_:  interfaceName,
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

func (m model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	modal := m.modal
	if modal == nil {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.modal = nil
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab", "up", "down":
			if len(modal.inputs) > 0 {
				if keyMsg.String() == "up" || keyMsg.String() == "shift+tab" {
					modal.focus = clamp(modal.focus-1, 0, len(modal.inputs)-1)
				} else {
					modal.focus = clamp(modal.focus+1, 0, len(modal.inputs)-1)
				}
				for i := range modal.inputs {
					if i == modal.focus {
						modal.inputs[i].Focus()
					} else {
						modal.inputs[i].Blur()
					}
				}
			}
			return m, nil
		case "enter":
			m.callModalMethod()
			return m, nil
		case "pgup":
			modal.responseScroll = clamp(modal.responseScroll-8, 0, max(0, len(modal.response)-1))
			return m, nil
		case "pgdown":
			modal.responseScroll = clamp(modal.responseScroll+8, 0, max(0, len(modal.response)-1))
			return m, nil
		}
	}

	if len(modal.inputs) > 0 {
		var cmd tea.Cmd
		modal.inputs[modal.focus], cmd = modal.inputs[modal.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) openCallModal(method entry) {
	inputs := make([]textinput.Model, 0)
	for _, arg := range method.args {
		if methodArgIsOut(method, arg) {
			continue
		}
		input := textinput.New()
		input.Placeholder = arg.Type + " " + typeDescription(arg.Type)
		input.Prompt = argLabel(arg) + ": "
		if len(inputs) == 0 {
			input.Focus()
		}
		inputs = append(inputs, input)
	}
	m.modal = &callModal{method: method, inputs: inputs}
}

func argLabel(arg introspectArg) string {
	name := arg.Name
	if name == "" {
		name = "arg"
	}
	return fmt.Sprintf("%s (%s)", name, arg.Type)
}

func (m *model) callModalMethod() {
	if m.modal == nil || m.conn == nil {
		return
	}
	method := m.modal.method
	args := make([]any, 0, len(m.modal.inputs))
	inputIndex := 0
	for _, arg := range method.args {
		if methodArgIsOut(method, arg) {
			continue
		}
		value, err := parseDBusInput(arg.Type, m.modal.inputs[inputIndex].Value())
		if err != nil {
			m.modal.err = err.Error()
			return
		}
		args = append(args, value)
		inputIndex++
	}

	member := method.interface_ + "." + method.memberName
	m.log("call %s %s %s sig=%s args=%v", method.busName, method.objectPath, member, dbusInputSignature(method), args)
	call := m.conn.Object(method.busName, dbus.ObjectPath(method.objectPath)).Call(member, 0, args...)
	if call.Err != nil {
		m.modal.err = call.Err.Error()
		m.modal.response = nil
		m.log("error method %s: %v", member, call.Err)
		return
	}
	m.modal.err = ""
	m.modal.response = formatResponseLines(call.Body)
	m.modal.responseScroll = 0
	m.log("reply method %s: %v", member, call.Body)
}

func methodArgIsOut(method entry, arg introspectArg) bool {
	if strings.TrimSpace(arg.Direction) == "out" {
		return true
	}

	// xdg-desktop-portal backend introspection can be incomplete/misleading in
	// practice. The published API marks these as OUT for impl FileChooser calls:
	// https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.impl.portal.FileChooser.html
	if method.interface_ == "org.freedesktop.impl.portal.FileChooser" {
		switch arg.Name {
		case "response", "results":
			return true
		}
	}

	return false
}

func isOutArg(arg introspectArg) bool {
	return strings.TrimSpace(arg.Direction) == "out"
}

func parseDBusInput(signature, raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	switch signature {
	case "s":
		return raw, nil
	case "o":
		return dbus.ObjectPath(raw), nil
	case "g":
		sig, err := dbus.ParseSignature(raw)
		if err != nil {
			return nil, err
		}
		return sig, nil
	case "b":
		if raw == "true" {
			return true, nil
		}
		if raw == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("%s expects true or false", signature)
	case "y":
		var v uint8
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "n":
		var v int16
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "q":
		var v uint16
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "i":
		var v int32
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "u":
		var v uint32
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "x":
		var v int64
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "t":
		var v uint64
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "d":
		var v float64
		_, err := fmt.Sscan(raw, &v)
		return v, err
	case "as":
		return parseStringArray(raw)
	case "a{sv}":
		return parseVariantMap(raw)
	default:
		return nil, fmt.Errorf("input parsing for %q not supported yet", signature)
	}
}

func dbusInputSignature(method entry) string {
	var b strings.Builder
	for _, arg := range method.args {
		if !methodArgIsOut(method, arg) {
			b.WriteString(arg.Type)
		}
	}
	return b.String()
}

func parseStringArray(raw string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}
	if strings.HasPrefix(raw, "[") {
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, err
		}
		return values, nil
	}
	parts := strings.Split(raw, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, nil
}

func parseVariantMap(raw string) (map[string]dbus.Variant, error) {
	if raw == "" {
		raw = "{}"
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}

	variants := make(map[string]dbus.Variant, len(values))
	for key, value := range values {
		variants[key] = dbus.MakeVariant(jsonValueToDBus(value))
	}
	return variants, nil
}

func jsonValueToDBus(value any) any {
	switch value := value.(type) {
	case map[string]any:
		mapped := make(map[string]dbus.Variant, len(value))
		for key, nested := range value {
			mapped[key] = dbus.MakeVariant(jsonValueToDBus(nested))
		}
		return mapped
	case []any:
		items := make([]any, len(value))
		for i, nested := range value {
			items[i] = jsonValueToDBus(nested)
		}
		return items
	case float64:
		if value == float64(int32(value)) {
			return int32(value)
		}
		return value
	default:
		return value
	}
}

func formatResponseLines(values []any) []string {
	if len(values) == 0 {
		return []string{"<empty reply>"}
	}
	lines := make([]string, 0, len(values))
	for i, value := range values {
		lines = append(lines, fmt.Sprintf("[%d] %#v", i, value))
	}
	return lines
}

func (m *model) readSelectedProperty() {
	p := m.activePane()
	if p == nil || len(p.entries) == 0 || p.cursor < 0 || p.cursor >= len(p.entries) {
		return
	}

	selected := &p.entries[p.cursor]
	m.log("call %s %s org.freedesktop.DBus.Properties.Get %s %s", selected.busName, selected.objectPath, selected.interface_, selected.name)
	obj := m.conn.Object(selected.busName, dbus.ObjectPath(selected.objectPath))

	var value dbus.Variant
	if err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, selected.interface_, selected.name).Store(&value); err != nil {
		m.log("error property get %s.%s: %v", selected.interface_, selected.name, err)
		selected.value = "error: " + err.Error()
		return
	}

	selected.value = formatDBusValue(value.Value())
	m.log("reply property get %s.%s: %s", selected.interface_, selected.name, selected.value)
}

func formatDBusValue(value any) string {
	return fmt.Sprintf("%#v", value)
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
	availableHeight := m.paneContentHeight()
	visible := visiblePanes(m.panes, m.width)

	rendered := make([]string, 0, len(visible))
	for i, p := range visible {
		active := i == len(visible)-1
		rendered = append(rendered, renderPane(p, paneDisplayWidth(p, m.width), availableHeight, active))
	}

	tree := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	breadcrumb := m.breadcrumb()

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("dbus-debug")+" "+statusStyle.Render(breadcrumb),
		tree,
		renderLogPane(m.logs, m.logScroll, m.width, logHeight),
		m.help.View(keys),
	)
	if m.modal != nil {
		return overlay(view, renderCallModal(*m.modal, m.width, m.height), m.width, m.height)
	}
	return view
}

func renderCallModal(modal callModal, terminalWidth, terminalHeight int) string {
	width := clamp(terminalWidth*4/5, 70, max(70, terminalWidth-4))
	height := clamp(terminalHeight*3/5, 12, max(12, terminalHeight-4))
	innerWidth := width - 4
	lines := []string{lipgloss.NewStyle().Bold(true).Render("Call method")}
	lines = append(lines, wrapLine(modal.method.busName+" "+modal.method.objectPath, innerWidth)...)
	lines = append(lines, wrapLine(modal.method.interface_+"."+modal.method.memberName, innerWidth)...)
	lines = append(lines, strings.Repeat("─", innerWidth))

	if len(modal.inputs) == 0 {
		lines = append(lines, "no input arguments")
	} else {
		for _, input := range modal.inputs {
			lines = append(lines, truncate(input.View(), innerWidth))
		}
	}

	lines = append(lines, "", "[esc] cancel    [enter] call")
	if modal.err != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render("error: "+truncate(modal.err, innerWidth-7)))
	}
	if len(modal.response) > 0 {
		lines = append(lines, strings.Repeat("─", innerWidth), "response:")
		used := len(lines)
		viewport := max(1, height-used-2)
		start := clamp(modal.responseScroll, 0, max(0, len(modal.response)-viewport))
		end := min(len(modal.response), start+viewport)
		for _, line := range modal.response[start:end] {
			lines = append(lines, truncate(line, innerWidth))
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#CBA6F7")).
		Padding(1, 2).
		Width(width).
		Height(height).
		Render(strings.Join(lines, "\n"))
}

func overlay(base, modal string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	modalLines := strings.Split(modal, "\n")
	x := max(0, (width-lipgloss.Width(modal))/2)
	y := max(0, (height-lipgloss.Height(modal))/2)

	out := make([]string, max(len(baseLines), y+len(modalLines)))
	copy(out, baseLines)
	for i, modalLine := range modalLines {
		lineIndex := y + i
		// Bubble Tea renders strings line-by-line; embedded carriage returns are not
		// a reliable way to composite an overlay. Replace the affected rows with a
		// centered modal row so the modal is always visible.
		out[lineIndex] = strings.Repeat(" ", x) + modalLine
	}
	return strings.Join(out, "\n")
}

func wrapLine(line string, width int) []string {
	if textWidth(line) <= width {
		return []string{line}
	}
	runes := []rune(line)
	lines := make([]string, 0, (len(runes)/width)+1)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func (m model) breadcrumb() string {
	parts := make([]string, 0, len(m.panes)+1)
	for i, p := range m.panes {
		if i == 0 {
			if selected, ok := selectedEntry(p); ok && selected.kind == entryBus {
				parts = append(parts, selected.name)
			}
			continue
		}

		switch {
		case strings.HasSuffix(p.title, " bus"):
			if selected, ok := selectedEntry(p); ok {
				parts = append(parts, selected.busName)
			}
		case strings.HasPrefix(p.title, "/"):
			parts = append(parts, p.title)
			if selected, ok := selectedEntry(p); ok && selected.kind == entryInterface {
				parts = append(parts, selected.interface_)
			}
		default:
			parts = append(parts, p.title)
			if selected, ok := selectedEntry(p); ok && (selected.kind == entryMethod || selected.kind == entryProperty || selected.kind == entrySignal) {
				parts = append(parts, selected.name)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " › ")
}

func selectedEntry(p pane) (entry, bool) {
	if len(p.entries) == 0 || p.cursor < 0 || p.cursor >= len(p.entries) {
		return entry{}, false
	}
	return p.entries[p.cursor], true
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

	innerWidth := width - 4
	metadata := selectedMetadataLines(p)
	metaHeight := metadataBlockHeight(p, height)

	header := lipgloss.NewStyle().Bold(true).Render(truncate(p.title, innerWidth))
	lines := []string{header, strings.Repeat("─", max(0, innerWidth))}

	if p.err != nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(p.err.Error()))
	} else if len(p.entries) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render("empty"))
	} else {
		viewport := max(1, height-3-metaHeight)
		start := clamp(p.scroll, 0, max(0, len(p.entries)-viewport))
		end := min(len(p.entries), start+viewport)
		for i := start; i < end; i++ {
			lines = append(lines, renderEntry(p.entries[i], i == p.cursor && active, innerWidth))
		}
	}

	if len(metadata) > 0 && metaHeight > 0 {
		lines = append(lines, strings.Repeat("─", max(0, innerWidth)))
		for _, line := range metadata[:min(len(metadata), metaHeight-1)] {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#A6ADC8")).Render(truncate(line, innerWidth)))
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

func renderEntry(e entry, selected bool, width int) string {
	line := truncate(kindIcon(e.kind)+e.name, width)
	if selected {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#11111B")).Background(lipgloss.Color("#CBA6F7")).Width(width).Render(line)
	}
	return lipgloss.NewStyle().Foreground(kindColor(e.kind)).Render(line)
}

func metadataBlockHeight(p pane, paneHeight int) int {
	entry, ok := selectedEntry(p)
	if !ok {
		return 0
	}
	lines := entryMetadataLines(entry)
	if len(lines) == 0 {
		return 0
	}

	// Bus-name metadata is bounded and predictable:
	// separator + kind/activatable/owner/pid/uid.
	if entry.kind == entryBusName {
		return min(6, max(0, paneHeight-6))
	}

	// Other metadata is content-sized as before, but capped so large annotations
	// or arg lists cannot push the pane/breadcrumb outside the terminal.
	maxMetadata := max(0, min(paneHeight/2, paneHeight-6))
	return min(len(lines)+1, maxMetadata)
}

func selectedMetadataLines(p pane) []string {
	if len(p.entries) == 0 || p.cursor < 0 || p.cursor >= len(p.entries) {
		return nil
	}
	return entryMetadataLines(p.entries[p.cursor])
}

func entryMetadataLines(e entry) []string {
	lines := make([]string, 0, 8)
	switch e.kind {
	case entryInterface:
		lines = append(lines, splitDetail(e.detail)...)
	case entryMethod, entrySignal:
		lines = append(lines, "args:")
		if len(e.args) == 0 {
			lines = append(lines, "  none")
		}
		for _, arg := range e.args {
			name := arg.Name
			if name == "" {
				name = "_"
			}
			direction := arg.Direction
			if direction == "" {
				direction = "in"
			}
			lines = append(lines, fmt.Sprintf("  %s %s: %s", direction, name, arg.Type))
			lines = append(lines, "    "+typeDescription(arg.Type))
		}
	case entryProperty:
		fields := strings.Fields(e.detail)
		if len(fields) > 0 {
			lines = append(lines, "type: "+fields[0])
			lines = append(lines, "  "+typeDescription(fields[0]))
		}
		if len(fields) > 1 {
			lines = append(lines, "access: "+fields[1])
		}
		if e.value != "" {
			lines = append(lines, "value:")
			lines = append(lines, "  "+e.value)
		} else if len(fields) > 1 && (fields[1] == "read" || fields[1] == "readwrite") {
			lines = append(lines, "enter: read value")
		}
	default:
		lines = append(lines, splitDetail(e.detail)...)
	}

	if len(e.annotations) > 0 {
		lines = append(lines, "annotations:")
		for _, annotation := range e.annotations {
			lines = append(lines, fmt.Sprintf("  %s: %s", annotation.Name, annotation.Value))
		}
	}
	return lines
}

func splitDetail(detail string) []string {
	if detail == "" {
		return nil
	}
	parts := strings.Split(detail, ",")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			lines = append(lines, part)
		}
	}
	return lines
}

func typeDescription(signature string) string {
	if signature == "" {
		return "unknown"
	}
	parser := signatureParser{input: []rune(signature)}
	parts := make([]string, 0)
	for parser.pos < len(parser.input) {
		parts = append(parts, parser.parseType())
	}
	return strings.Join(parts, ", ")
}

type signatureParser struct {
	input []rune
	pos   int
}

func (p *signatureParser) parseType() string {
	if p.pos >= len(p.input) {
		return "unknown"
	}
	ch := p.input[p.pos]
	p.pos++

	switch ch {
	case 'y':
		return "byte"
	case 'b':
		return "boolean"
	case 'n':
		return "int16"
	case 'q':
		return "uint16"
	case 'i':
		return "int32"
	case 'u':
		return "uint32"
	case 'x':
		return "int64"
	case 't':
		return "uint64"
	case 'd':
		return "double"
	case 'h':
		return "unix fd"
	case 's':
		return "string"
	case 'o':
		return "object path"
	case 'g':
		return "signature"
	case 'v':
		return "variant"
	case 'a':
		if p.pos < len(p.input) && p.input[p.pos] == '{' {
			p.pos++
			key := p.parseType()
			value := p.parseType()
			if p.pos < len(p.input) && p.input[p.pos] == '}' {
				p.pos++
			}
			return fmt.Sprintf("dictionary<%s, %s>", key, value)
		}
		return "array<" + p.parseType() + ">"
	case '(':
		items := make([]string, 0)
		for p.pos < len(p.input) && p.input[p.pos] != ')' {
			items = append(items, p.parseType())
		}
		if p.pos < len(p.input) && p.input[p.pos] == ')' {
			p.pos++
		}
		return "struct<" + strings.Join(items, ", ") + ">"
	case '{':
		key := p.parseType()
		value := p.parseType()
		if p.pos < len(p.input) && p.input[p.pos] == '}' {
			p.pos++
		}
		return fmt.Sprintf("dict entry<%s, %s>", key, value)
	default:
		return fmt.Sprintf("unknown(%c)", ch)
	}
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
		contentWidth = max(contentWidth, textWidth(kindIcon(entry.kind)+entry.name))
	}

	// Do not include metadata width here. Metadata is rendered inside whatever
	// width the list/title needs and may be truncated. Letting metadata determine
	// pane width causes long owner lists, annotations, or /nix/store paths to make
	// one pane consume the whole terminal.
	return clamp(contentWidth+4, 24, max(24, terminalWidth-2))
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
