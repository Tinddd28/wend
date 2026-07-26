// Package mainview реализует основную панель: список, гистограмму, кольцевую
// диаграмму или treemap содержимого выбранной в sidebar директории.
package mainview

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tinddd28/wend/internal/humanize"
	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/canvas"
	"github.com/Tinddd28/wend/internal/tui/chart"
	"github.com/Tinddd28/wend/internal/tui/style"
)

type ViewMode int

const (
	ModeList ViewMode = iota
	ModeBars
	ModeRings
	ModeTreemap
	modeCount
)

var modeNames = [modeCount]string{"список", "полосы", "кольца", "treemap"}

func (v ViewMode) String() string { return modeNames[v] }

// isChart — режимы, рисующие растровую диаграмму, а не список строк:
// у них нет курсора и прокрутки, зато они используют весь холст панели.
func (v ViewMode) isChart() bool { return v == ModeRings || v == ModeTreemap }

// OpenMsg — пользователь «провалился» в директорию из главной панели.
// Обрабатывается в RootModel, чтобы sidebar и main не разъезжались:
// раньше main вообще не умел навигацию и был read-only придатком дерева.
type OpenMsg struct{ Path string }

// sizeCol — ширина колонки размера ("1023.9 MiB" — максимум 10 символов).
const (
	sizeCol  = 10
	pctCol   = 5
	itemsCol = 7
)

type Model struct {
	tree      *scan.DirTree
	path      string
	highlight string
	mode      ViewMode
	cursor    int
	offset    int
	width     int
	height    int
	focused   bool
}

func New(tree *scan.DirTree) Model {
	return Model{tree: tree, path: tree.Root().Path}
}

// SetPath меняет отображаемую директорию (вызывается при перемещении курсора
// в sidebar). Смена пути сбрасывает курсор — иначе он «залипал» бы на индексе
// от предыдущей директории.
func (m *Model) SetPath(path string) {
	if path == "" || path == m.path {
		return
	}
	m.path = path
	m.cursor, m.offset = 0, 0
}

// SetHighlight сообщает диаграммам, какая ветка выбрана в sidebar: она
// рисуется в полном цвете, остальное приглушается.
func (m *Model) SetHighlight(path string) { m.highlight = path }

func (m Model) Path() string   { return m.path }
func (m Model) Mode() ViewMode { return m.mode }

func (m *Model) SetFocused(f bool) { m.focused = f }

func (m *Model) SetSize(w, h int) {
	m.width, m.height = max(w, 0), max(h, 0)
	m.clamp(len(m.tree.Children(m.path)))
}

// ToggleMode циклически переключает режимы. Вынесено в метод, т.к. RootModel
// обрабатывает эту клавишу глобально: подсказка в статусбаре обещает "v"
// всегда, а не только когда фокус стоит на главной панели.
func (m *Model) ToggleMode() { m.mode = (m.mode + 1) % modeCount }

// rowsHeight — сколько строк остаётся под список после заголовка панели.
func (m Model) rowsHeight() int { return max(m.height-1, 1) }

func (m *Model) clamp(n int) {
	if n == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	m.cursor = max(0, min(m.cursor, n-1))

	h := m.rowsHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, max(0, n-h)))
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	children := m.tree.Children(m.path)
	var cmd tea.Cmd

	switch keyMsg.String() {
	case "j", "down":
		m.cursor++
	case "k", "up":
		m.cursor--
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(children) - 1
	case "pgdown", "ctrl+f":
		m.cursor += max(m.rowsHeight()-1, 1)
	case "pgup", "ctrl+b":
		m.cursor -= max(m.rowsHeight()-1, 1)
	case "l", "right", "enter":
		if m.cursor < len(children) && children[m.cursor].Kind == scan.KindDir {
			path := children[m.cursor].Path
			cmd = func() tea.Msg { return OpenMsg{Path: path} }
		}
	case "h", "left", "backspace":
		if m.path != m.tree.Root().Path {
			parent := filepath.Dir(m.path)
			cmd = func() tea.Msg { return OpenMsg{Path: parent} }
		}
	}

	m.clamp(len(children))
	return m, cmd
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	children := m.tree.Children(m.path)
	// Стиль применяется после подгонки под ширину: Cell поверх раскрашенной
	// строки пришлось бы разбирать по ANSI-кластерам.
	lines := []string{style.Title.Render(style.Cell(m.title(len(children)), m.width))}

	switch {
	case len(children) == 0:
		lines = append(lines, style.Cell("(пусто)", m.width))
	case m.mode.isChart():
		lines = append(lines, m.renderChart()...)
	default:
		lines = append(lines, m.renderRows(children)...)
	}

	return style.Pane(lines, m.width, m.height, m.focused)
}

// renderChart рисует растровую диаграмму на холсте размером с панель.
func (m Model) renderChart() []string {
	node := m.tree.Node(m.path)
	if node == nil {
		return nil
	}

	cv := canvas.New(m.width, m.rowsHeight())
	opts := chart.Opts{Tree: m.tree, Root: node, Highlight: m.highlight}
	if m.mode == ModeRings {
		chart.Rings(cv, opts)
	} else {
		chart.Treemap(cv, opts)
	}

	lines := cv.Lines()
	for i, l := range lines {
		lines[i] = style.Cell(l, m.width)
	}
	return lines
}

func (m Model) renderRows(children []*scan.DirNode) []string {
	// maxSize задаёт масштаб полосы (так разница между элементами читается
	// лучше всего), total — знаменатель для доли: процент от объёма
	// родителя информативнее доли от самого крупного соседа.
	var maxSize, total int64
	for _, c := range children {
		maxSize = max(maxSize, c.Size)
		total += c.Size
	}

	// Окно прокрутки считается по offset, а не обрезанием «первых N»:
	// раньше строки ниже экрана были просто недостижимы, хотя панель
	// принимала фокус по tab.
	end := min(m.offset+m.rowsHeight(), len(children))
	lines := make([]string, 0, end-m.offset)

	for i := m.offset; i < end; i++ {
		c := children[i]
		var line string
		if m.mode == ModeBars {
			line = m.renderBarRow(c, maxSize, total)
		} else {
			line = m.renderListRow(c, total)
		}
		line = style.Cell(line, m.width)

		if i == m.cursor {
			line = style.Highlight(line, m.focused)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) title(n int) string {
	rel, err := filepath.Rel(m.tree.Root().Path, m.path)
	if err != nil || rel == "." {
		rel = filepath.Base(m.tree.Root().Path)
	}
	node := m.tree.Node(m.path)
	items := ""
	if node != nil {
		items = fmt.Sprintf(" · %s всего", humanize.Count(node.Items))
	}
	return fmt.Sprintf("%s · %d в этой папке%s · [%s]", rel, n, items, m.mode)
}

func (m Model) renderListRow(c *scan.DirNode, total int64) string {
	items := "—"
	if c.Kind == scan.KindDir {
		items = humanize.Count(c.Items)
	}
	return fmt.Sprintf("%*s %*s %*s  %s",
		sizeCol, humanize.Bytes(c.Size),
		pctCol, percent(c.Size, total),
		itemsCol, items,
		displayName(c))
}

func (m Model) renderBarRow(c *scan.DirNode, maxSize, total int64) string {
	// Полоса считается от остатка ширины после колонок размера/доли/имени —
	// раньше она брала почти всю ширину панели, а метка дописывалась *сверх*
	// неё, и строка гарантированно переносилась.
	nameW := max(m.width/4, 8)
	barW := m.width - sizeCol - pctCol - nameW - 4
	if barW < 4 {
		barW = max(m.width-sizeCol-pctCol-4, 1)
		nameW = 0
	}

	filled := 0
	if maxSize > 0 && c.Size > 0 {
		filled = max(1, int(float64(c.Size)/float64(maxSize)*float64(barW)))
	}
	filled = min(filled, barW)

	bar := style.BarFilled.Render(strings.Repeat("█", filled)) +
		style.BarEmpty.Render(strings.Repeat("░", barW-filled))

	row := fmt.Sprintf("%*s %*s %s", sizeCol, humanize.Bytes(c.Size), pctCol, percent(c.Size, total), bar)
	if nameW > 0 {
		row += " " + style.Clip(displayName(c), nameW)
	}
	return row
}

func displayName(c *scan.DirNode) string {
	name := c.Name
	if c.Kind == scan.KindDir {
		name += "/"
	}
	if c.Dup {
		name += " ⇥"
	}
	return name
}

func percent(size, total int64) string {
	if total <= 0 {
		return "   0%"
	}
	return fmt.Sprintf("%4.0f%%", float64(size)/float64(total)*100)
}
