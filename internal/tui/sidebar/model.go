// Package sidebar реализует панель дерева директорий: vim-навигация,
// expand/collapse, рендер таблицей с долей, размером, числом элементов и
// временем изменения.
package sidebar

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tinddd28/wend/internal/humanize"
	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/style"
)

// row — одна видимая строка дерева: узел, его глубина для отступа и доля от
// размера родителя. Доля считается при пересборке, а не при рендере: иначе
// на каждую перерисовку пришлось бы заново суммировать соседей узла.
type row struct {
	node  *scan.DirNode
	depth int
	frac  float64
}

type Model struct {
	tree     *scan.DirTree
	expanded map[string]bool
	rows     []row
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
}

func New(tree *scan.DirTree) Model {
	m := Model{
		tree:     tree,
		expanded: map[string]bool{tree.Root().Path: true},
		focused:  true,
	}
	m.Rebuild()
	return m
}

// Rebuild пересобирает плоский список видимых строк из актуального состояния
// дерева. Вызывается извне после каждого мержа результатов скана.
//
// Курсор восстанавливается по пути, а не по индексу: во время скана строки
// постоянно переупорядочиваются по размеру, и удержание индекса выглядело бы
// как самопроизвольный переезд выделения на другой файл.
func (m *Model) Rebuild() {
	var prevPath string
	if n := m.Selected(); n != nil {
		prevPath = n.Path
	}

	m.rows = m.rows[:0]
	var walk func(n *scan.DirNode, depth int, frac float64)
	walk = func(n *scan.DirNode, depth int, frac float64) {
		m.rows = append(m.rows, row{node: n, depth: depth, frac: frac})
		if n.Kind != scan.KindDir || !m.expanded[n.Path] {
			return
		}
		kids := m.tree.Children(n.Path)
		var total int64
		for _, c := range kids {
			total += c.Size
		}
		for _, c := range kids {
			var f float64
			if total > 0 {
				f = float64(c.Size) / float64(total)
			}
			walk(c, depth+1, f)
		}
	}
	walk(m.tree.Root(), 0, 1)

	if prevPath != "" {
		if i := m.indexOf(prevPath); i >= 0 {
			m.cursor = i
		}
	}
	m.clamp()
}

func (m *Model) indexOf(path string) int {
	for i, r := range m.rows {
		if r.node.Path == path {
			return i
		}
	}
	return -1
}

// Reset сворачивает дерево до корня — нужен перед пересканированием.
func (m *Model) Reset() {
	m.expanded = map[string]bool{m.tree.Root().Path: true}
	m.cursor, m.offset = 0, 0
	m.Rebuild()
}

// Reveal раскрывает путь и ставит на него курсор — используется, когда
// пользователь «проваливается» в директорию из главной панели.
func (m *Model) Reveal(path string) {
	root := m.tree.Root().Path
	for p := filepath.Clean(path); strings.HasPrefix(p, root); p = filepath.Dir(p) {
		m.expanded[p] = true
		if p == root {
			break
		}
	}
	m.Rebuild()
	if i := m.indexOf(filepath.Clean(path)); i >= 0 {
		m.cursor = i
		m.clamp()
	}
}

// rowsHeight — сколько строк остаётся под дерево после шапки таблицы.
func (m Model) rowsHeight() int { return max(m.height-1, 1) }

// clamp удерживает курсор в границах списка, а окно прокрутки — вокруг курсора.
func (m *Model) clamp() {
	if len(m.rows) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	m.cursor = max(0, min(m.cursor, len(m.rows)-1))

	h := m.rowsHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, max(0, len(m.rows)-h)))
}

// Selected возвращает узел под курсором (nil, если дерево ещё пустое).
func (m Model) Selected() *scan.DirNode {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor].node
}

// SelectedDir возвращает директорию, содержимое которой уместно показать в
// главной панели: сам узел, если это директория, иначе — его родителя.
// Раньше на файле в main уезжал пустой список «детей файла».
func (m Model) SelectedDir() *scan.DirNode {
	n := m.Selected()
	if n == nil {
		return nil
	}
	if n.Kind == scan.KindDir {
		return n
	}
	if n.Parent != nil {
		return n.Parent
	}
	return m.tree.Root()
}

func (m *Model) SetFocused(f bool) { m.focused = f }

func (m *Model) SetSize(w, h int) {
	m.width, m.height = max(w, 0), max(h, 0)
	m.clamp()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "j", "down":
		m.cursor++
	case "k", "up":
		m.cursor--
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.rows) - 1
	case "pgdown", "ctrl+f":
		m.cursor += max(m.rowsHeight()-1, 1)
	case "pgup", "ctrl+b":
		m.cursor -= max(m.rowsHeight()-1, 1)
	case "l", "right", "enter":
		if n := m.Selected(); n != nil && n.Kind == scan.KindDir {
			if m.expanded[n.Path] {
				// Уже раскрыта — шагаем внутрь, к первому потомку.
				if m.cursor+1 < len(m.rows) && m.rows[m.cursor+1].node.Parent == n {
					m.cursor++
				}
			} else {
				m.expanded[n.Path] = true
				m.Rebuild()
			}
		}
	case "h", "left":
		if n := m.Selected(); n != nil {
			if n.Kind == scan.KindDir && m.expanded[n.Path] {
				m.expanded[n.Path] = false
				m.Rebuild()
			} else if n.Parent != nil {
				if i := m.indexOf(n.Parent.Path); i >= 0 {
					m.cursor = i
				}
			}
		}
	}
	m.clamp()
	return m, nil
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	cols := layoutColumns(m.width)
	lines := make([]string, 0, m.height)
	lines = append(lines, cols.header(m.width))

	end := min(m.offset+m.rowsHeight(), len(m.rows))
	for i := m.offset; i < end; i++ {
		line := style.Cell(m.renderRow(m.rows[i], cols), m.width)
		if i == m.cursor {
			line = style.Highlight(line, m.focused)
		}
		lines = append(lines, line)
	}

	return style.Pane(lines, m.width, m.height, m.focused)
}

func (m Model) renderRow(r row, cols columns) string {
	var b strings.Builder

	if cols.bar {
		b.WriteString(renderBar(r.frac) + " ")
	}
	if cols.pct {
		b.WriteString(renderPct(r.frac) + " ")
	}

	marker := "  "
	if r.node.Kind == scan.KindDir {
		if m.expanded[r.node.Path] {
			marker = "▾ "
		} else {
			marker = "▸ "
		}
	}
	name := r.node.Name
	if r.node.Dup {
		// Повторная жёсткая ссылка: размер показан, но в сумму не вошёл —
		// без пометки строка выглядела бы как потерянные байты.
		name += " ⇥"
	}
	b.WriteString(style.Cell(strings.Repeat("  ", r.depth)+marker+name, cols.name))

	b.WriteString(fmt.Sprintf(" %*s", colSize, humanize.Bytes(r.node.Size)))
	if cols.items {
		items := "—"
		if r.node.Kind == scan.KindDir {
			items = humanize.Count(r.node.Items)
		}
		b.WriteString(fmt.Sprintf(" %*s", colItems, items))
	}
	if cols.mod {
		b.WriteString(fmt.Sprintf(" %*s", colMod, humanize.Age(r.node.ModTime)))
	}
	return b.String()
}
