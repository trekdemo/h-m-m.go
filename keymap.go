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
	ToggleHelp  key.Binding
	Save        key.Binding
}

var normalModeKeys = keyMap{
	Quit:        key.NewBinding(key.WithKeys("ctrl+c", "q", "esc"), key.WithHelp("q", "quit")),
	EnterEdit:   key.NewBinding(key.WithKeys("i", "a"), key.WithHelp("i/a", "edit")),
	AddSibling:  key.NewBinding(key.WithKeys("o", "enter"), key.WithHelp("o/enter", "add sibling")),
	AddChild:    key.NewBinding(key.WithKeys("O", "tab"), key.WithHelp("O/tab", "add child")),
	Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	MoveSibUp:   key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "move up among siblings")),
	MoveSibDown: key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "move down among siblings")),
	ToggleHelp:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	Save:        key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
}

// ShortHelp returns the bindings shown in the collapsed, single-line help
// view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.EnterEdit, k.AddSibling, k.AddChild, k.Save, k.Quit, k.ToggleHelp}
}

// FullHelp returns the bindings shown in the expanded, multi-column help
// view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.EnterEdit, k.AddSibling, k.AddChild},
		{k.MoveSibUp, k.MoveSibDown},
		{k.Save, k.Quit, k.ToggleHelp},
	}
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
	Exit:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "exit edit mode")),
	Backspace: key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "delete before cursor")),
	Delete:    key.NewBinding(key.WithKeys("delete"), key.WithHelp("delete", "delete after cursor")),
	Left:      key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "cursor left")),
	Right:     key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "cursor right")),
	Home:      key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "cursor to start")),
	End:       key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "cursor to end")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "newline")),
}

// ShortHelp returns the bindings shown in the collapsed, single-line help
// view.
func (k editModeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Left, k.Right, k.Home, k.End, k.Backspace, k.Delete, k.Enter, k.Exit}
}

// FullHelp returns the bindings shown in the expanded, multi-column help
// view.
func (k editModeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Left, k.Right, k.Home, k.End},
		{k.Backspace, k.Delete, k.Enter},
		{k.Exit},
	}
}
