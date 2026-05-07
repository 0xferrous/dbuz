package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
		titleStyle.Render("dbuz")+" "+statusStyle.Render(breadcrumb),
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
