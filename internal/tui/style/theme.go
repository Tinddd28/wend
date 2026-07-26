package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	Accent = lipgloss.Color("212")
	Dim    = lipgloss.Color("240")
	Danger = lipgloss.Color("203")

	Border = lipgloss.RoundedBorder()

	PaneFocused = lipgloss.NewStyle().
			Border(Border).
			BorderForeground(Accent)

	PaneBlurred = lipgloss.NewStyle().
			Border(Border).
			BorderForeground(Dim)

	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(Accent).
		Padding(0, 1)

	Footer = lipgloss.NewStyle().
		Foreground(Dim).
		Padding(0, 1)

	Error = lipgloss.NewStyle().Foreground(Danger)

	Title = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)

	ColumnHeader = lipgloss.NewStyle().Foreground(Dim).Bold(true)

	Detail = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	Selected = lipgloss.NewStyle().
			Background(Accent).
			Foreground(lipgloss.Color("0")).
			Bold(true)

	// SelectedBlurred — курсор в неактивной панели: он должен оставаться
	// видимым (иначе после tab непонятно, куда вернёшься), но не спорить
	// за внимание с курсором в активной панели.
	SelectedBlurred = lipgloss.NewStyle().
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("252"))

	BarFilled = lipgloss.NewStyle().Foreground(Accent)
	BarEmpty  = lipgloss.NewStyle().Foreground(Dim)
)

// Cell приводит строку ровно к ширине w: обрезает по видимой ширине (с учётом
// ANSI-последовательностей и широких рун) и добивает пробелами.
//
// Это ключевая защита layout'а: lipgloss.Width() не режет, а *переносит*
// длинные строки, а Height() не обрезает лишние — панель с длинным именем
// файла молча становилась выше своего бокса и ломала JoinHorizontal.
func Cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	switch sw := ansi.StringWidth(s); {
	case sw > w:
		return ansi.Truncate(s, w, "…")
	case sw < w:
		return s + strings.Repeat(" ", w-sw)
	}
	return s
}

// Clip обрезает строку до ширины w, не добивая пробелами.
//
// Проверка ширины перед вызовом Truncate не оптимизация, а необходимость:
// Truncate пересобирает строку, заново выставляя стиль перед каждым
// графемным кластером, и на уже раскрашенном тексте вывод раздувался
// в десятки раз даже когда обрезать было нечего.
func Clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

// Highlight накладывает выделение курсора на готовую строку.
//
// Собственные цвета из строки снимаются: вложенный сброс (ESC[0m) от
// раскрашенного бара обрывал фон выделения на середине, и подсветка
// выглядела рваной.
func Highlight(s string, focused bool) string {
	s = ansi.Strip(s)
	if focused {
		return Selected.Render(s)
	}
	return SelectedBlurred.Render(s)
}

// Pane рендерит содержимое панели в рамке фиксированного размера.
// lines уже обязаны быть обрезаны по ширине w (см. Cell); лишние строки
// отбрасываются, недостающие добиваются самим lipgloss.
func Pane(lines []string, w, h int, focused bool) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	s := PaneBlurred
	if focused {
		s = PaneFocused
	}
	return s.Width(w).Height(h).MaxWidth(w + 2).MaxHeight(h + 2).
		Render(strings.Join(lines, "\n"))
}
