package main

import "github.com/charmbracelet/bubbles/key"

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
