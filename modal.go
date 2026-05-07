package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
)

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
