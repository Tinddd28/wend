package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Collapse key.Binding
	Expand   key.Binding
	Toggle   key.Binding
	NextPane key.Binding
	Rescan   key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("j/k", "навигация"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	Collapse: key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/l", "свернуть/развернуть"),
	),
	Expand: key.NewBinding(
		key.WithKeys("l", "right", "enter"),
		key.WithHelp("l/→/enter", "expand/into"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "вид"),
	),
	NextPane: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "панель"),
	),
	Rescan: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "пересканировать"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "выход"),
	),
}

// helpBindings — то, что показывается в статусбаре. Up/Collapse описывают
// сразу обе клавиши пары, поэтому Down/Expand в подсказку не попадают.
var helpBindings = []key.Binding{
	keys.Up, keys.Collapse, keys.Toggle, keys.NextPane, keys.Rescan, keys.Quit,
}

// HelpText собирает строку подсказок из самих биндингов. Раньше она была
// продублирована литералом в statusbar и успела разойтись с поведением.
func (k keyMap) HelpText() string {
	parts := make([]string, 0, len(helpBindings))
	for _, b := range helpBindings {
		h := b.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return strings.Join(parts, "  ")
}
