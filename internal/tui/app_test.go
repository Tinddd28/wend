package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/mainview"
)

// renderAt наполняет модель деревом и рендерит её в терминале w×h.
func renderAt(t *testing.T, root string, entries []scan.Entry, w, h int) string {
	t.Helper()

	m := New(root)
	for _, e := range entries {
		m.tree.Upsert(e)
	}
	m.sidebar.Rebuild()

	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.View()
}

func assertFits(t *testing.T, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Errorf("высота вывода = %d строк, ожидалось %d", len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got > w {
			t.Errorf("строка %d шириной %d > %d: %q", i, got, w, ansi.Strip(l))
		}
	}
}

// TestView_FitsTerminal — регрессия на главный визуальный баг: длинные имена
// и глубокая вложенность переносились lipgloss'ом, панель становилась выше
// своего бокса, и весь layout разъезжался.
func TestView_FitsTerminal(t *testing.T) {
	root := "/repo"
	long := strings.Repeat("very-long-file-name-", 8) + ".bin"
	deep := filepath.Join(root, "a", "b", "c", "d", "e", "f", "g")

	entries := []scan.Entry{
		{Path: filepath.Join(root, long), Kind: scan.KindFile, Size: 5 << 30},
		{Path: filepath.Join(root, "sub"), Kind: scan.KindDir},
		{Path: filepath.Join(root, "sub", "x.bin"), Kind: scan.KindFile, Size: 1 << 20},
		{Path: filepath.Join(deep, long), Kind: scan.KindFile, Size: 42},
	}

	for _, tc := range []struct{ w, h int }{
		{80, 24}, {120, 40}, {40, 10}, {30, 8},
	} {
		view := renderAt(t, root, entries, tc.w, tc.h)
		assertFits(t, view, tc.w, tc.h)
	}
}

// TestView_AllModesFitTerminal — каждый режим рисует по-своему (строки,
// полосы, два растровых холста), и каждый обязан уложиться в свою панель.
// Полоса раньше занимала почти всю ширину, а метка дописывалась сверх неё.
func TestView_AllModesFitTerminal(t *testing.T) {
	root := "/repo"
	long := strings.Repeat("z", 90)
	entries := []scan.Entry{
		{Path: filepath.Join(root, long), Kind: scan.KindFile, Size: 1 << 40},
		{Path: filepath.Join(root, "sub"), Kind: scan.KindDir},
		{Path: filepath.Join(root, "sub", "a"), Kind: scan.KindFile, Size: 1 << 30},
		{Path: filepath.Join(root, "sub", "b"), Kind: scan.KindFile, Size: 1 << 20},
		{Path: filepath.Join(root, "small"), Kind: scan.KindFile, Size: 1},
	}

	for _, tc := range []struct{ w, h int }{{150, 40}, {100, 20}, {60, 12}, {34, 9}} {
		m := New(root)
		for _, e := range entries {
			m.tree.Upsert(e)
		}
		m.sidebar.Reveal(filepath.Join(root, "sub"))
		m.syncSelection()

		next, _ := m.Update(tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
		cur := next.(RootModel)

		for mode := range int(mainview.ModeTreemap) + 1 {
			if got := cur.main.Mode(); int(got) != mode {
				t.Fatalf("режим = %v, ожидался индекс %d", got, mode)
			}
			assertFits(t, cur.View(), tc.w, tc.h)
			n, _ := cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
			cur = n.(RootModel)
		}
	}
}

// TestSelectingFile_ShowsParentDirectory — на файле главная панель показывала
// «детей файла», т.е. пустоту.
func TestSelectingFile_ShowsParentDirectory(t *testing.T) {
	root := "/repo"
	m := New(root)
	m.tree.Upsert(scan.Entry{Path: filepath.Join(root, "sub"), Kind: scan.KindDir, Size: 0})
	m.tree.Upsert(scan.Entry{Path: filepath.Join(root, "sub", "x.bin"), Kind: scan.KindFile, Size: 100})
	m.sidebar.Rebuild()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cur := next.(RootModel)

	// root → (шаг внутрь) sub → (раскрыть) → x.bin
	for _, k := range []string{"l", "l", "j"} {
		next, _ = cur.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		cur = next.(RootModel)
	}

	if got := cur.sidebar.Selected(); got == nil || got.Name != "x.bin" {
		t.Fatalf("курсор sidebar = %v, ожидался x.bin", got)
	}
	if got, want := cur.main.Path(), filepath.Join(root, "sub"); got != want {
		t.Errorf("путь главной панели = %q, ожидался %q (родитель файла)", got, want)
	}
}
