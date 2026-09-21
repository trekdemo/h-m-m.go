package main

import "charm.land/bubbles/v2/key"

// keyMap defines the keybindings used in normalMode. Bindings are grouped
// into embeddable sub-maps so the navigator and edit-mode bindings can each
// be defined near the code that uses them.
type keyMap struct {
	Quit        key.Binding
	EnterEdit   key.Binding
	AddSibling  key.Binding
	AddChild    key.Binding
	Left        key.Binding
	Right       key.Binding
	Up          key.Binding
	Down        key.Binding
	MoveSibUp   key.Binding
	MoveSibDown key.Binding
}

var normalModeKeys = keyMap{
	Quit:        key.NewBinding(key.WithKeys("ctrl+c", "q", "esc")),
	EnterEdit:   key.NewBinding(key.WithKeys("i", "a")),
	AddSibling:  key.NewBinding(key.WithKeys("o", "enter")),
	AddChild:    key.NewBinding(key.WithKeys("O", "tab")),
	Left:        key.NewBinding(key.WithKeys("left", "h")),
	Right:       key.NewBinding(key.WithKeys("right", "l")),
	Up:          key.NewBinding(key.WithKeys("up", "k")),
	Down:        key.NewBinding(key.WithKeys("down", "j")),
	MoveSibUp:   key.NewBinding(key.WithKeys("K")),
	MoveSibDown: key.NewBinding(key.WithKeys("J")),
}

// editModeKeyMap defines the keybindings used while editMode is active.
type editModeKeyMap struct {
	Exit      key.Binding
	Backspace key.Binding
	Delete    key.Binding
	Left      key.Binding
	Right     key.Binding
	Home      key.Binding
	End       key.Binding
	Enter     key.Binding
}

var editModeKeys = editModeKeyMap{
	Exit:      key.NewBinding(key.WithKeys("esc")),
	Backspace: key.NewBinding(key.WithKeys("backspace")),
	Delete:    key.NewBinding(key.WithKeys("delete")),
	Left:      key.NewBinding(key.WithKeys("left")),
	Right:     key.NewBinding(key.WithKeys("right")),
	Home:      key.NewBinding(key.WithKeys("home")),
	End:       key.NewBinding(key.WithKeys("end")),
	Enter:     key.NewBinding(key.WithKeys("enter")),
}
