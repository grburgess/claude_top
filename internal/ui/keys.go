package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Quit    key.Binding
	Mode    key.Binding
	Refresh key.Binding
	Focus   key.Binding
}

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k")),
	Down:    key.NewBinding(key.WithKeys("down", "j")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c")),
	Mode:    key.NewBinding(key.WithKeys("h")),
	Refresh: key.NewBinding(key.WithKeys("r")),
	Focus:   key.NewBinding(key.WithKeys("enter")),
}
