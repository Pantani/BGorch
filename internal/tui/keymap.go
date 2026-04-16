package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Select     key.Binding
	Run        key.Binding
	Back       key.Binding
	NextFocus  key.Binding
	PrevFocus  key.Binding
	Toggle     key.Binding
	Help       key.Binding
	Quit       key.Binding
	Filter     key.Binding
	TableFocus key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Select:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run/select")),
		Run:        key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "run action")),
		Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/cancel")),
		NextFocus:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next focus")),
		PrevFocus:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev focus")),
		Toggle:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle option")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter actions")),
		TableFocus: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch summary/table")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Run, k.NextFocus, k.Back, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.Run},
		{k.NextFocus, k.PrevFocus, k.Toggle, k.Filter},
		{k.Back, k.Help, k.Quit},
	}
}
