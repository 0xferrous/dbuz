package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/godbus/dbus/v5"
)

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
	lines := make([]string, 0, 8)
	obj := m.conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")

	if !strings.HasPrefix(name, ":") {
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

	lines = append(lines, "kind: "+busNameKind(name))
	if activatable[name] {
		lines = append(lines, "activatable: yes")
	}

	if strings.HasPrefix(name, ":") {
		wellKnown := wellKnownByOwner[name]
		if len(wellKnown) > 0 {
			lines = append(lines, "well-known names:")
			for _, ownedName := range wellKnown {
				lines = append(lines, "  "+ownedName)
			}
		}
	}

	var uid uint32
	m.log("call org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.GetConnectionUnixUser %s", name)
	if err := obj.Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, name).Store(&uid); err != nil {
		m.log("error org.freedesktop.DBus.GetConnectionUnixUser %s: %v", name, err)
	} else {
		lines = append(lines, "uid: "+formatUID(uid))
		m.log("reply org.freedesktop.DBus.GetConnectionUnixUser %s: %d", name, uid)
	}

	return strings.Join(lines, ", ")
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
