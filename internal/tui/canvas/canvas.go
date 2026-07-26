// Package canvas — растровый холст из символьных ячеек с цветом фона и текста.
// Диаграммы рисуют в него по координатам, а он один раз собирает строки,
// склеивая соседние ячейки с одинаковым стилем в общий ANSI-сегмент
// (иначе escape-последовательности на каждый символ раздували бы вывод в
// десятки раз и заметно тормозили перерисовку).
package canvas

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Cell — одна ячейка. Пустые FG/BG означают «цвет терминала по умолчанию».
type Cell struct {
	R  rune
	FG string
	BG string
}

type Canvas struct {
	w, h  int
	cells []Cell
}

func New(w, h int) *Canvas {
	w, h = max(w, 0), max(h, 0)
	return &Canvas{w: w, h: h, cells: make([]Cell, w*h)}
}

func (c *Canvas) Width() int  { return c.w }
func (c *Canvas) Height() int { return c.h }

func (c *Canvas) inside(x, y int) bool { return x >= 0 && y >= 0 && x < c.w && y < c.h }

func (c *Canvas) Set(x, y int, cell Cell) {
	if c.inside(x, y) {
		c.cells[y*c.w+x] = cell
	}
}

// Fill закрашивает прямоугольник фоном (координаты клиппятся по холсту).
func (c *Canvas) Fill(x, y, w, h int, bg string) {
	for j := max(y, 0); j < min(y+h, c.h); j++ {
		for i := max(x, 0); i < min(x+w, c.w); i++ {
			c.cells[j*c.w+i] = Cell{R: ' ', BG: bg}
		}
	}
}

// Text пишет строку с позиции (x,y), сохраняя уже имеющийся фон ячеек,
// если bg пустой — так подпись ложится поверх заливки диаграммы.
func (c *Canvas) Text(x, y int, s, fg, bg string) {
	for _, r := range s {
		if !c.inside(x, y) {
			x++
			continue
		}
		cell := &c.cells[y*c.w+x]
		cell.R, cell.FG = r, fg
		if bg != "" {
			cell.BG = bg
		}
		x++
	}
}

// TextCentered пишет строку, центрируя её по x в пределах ширины w.
func (c *Canvas) TextCentered(x, y, w int, s, fg, bg string) {
	c.Text(x+max(0, (w-len([]rune(s)))/2), y, s, fg, bg)
}

// Lines собирает холст в строки, готовые к вставке в панель.
func (c *Canvas) Lines() []string {
	lines := make([]string, c.h)
	var b, seg strings.Builder

	for y := range c.h {
		b.Reset()
		row := c.cells[y*c.w : (y+1)*c.w]
		for i := 0; i < len(row); {
			j := i
			for j < len(row) && row[j].FG == row[i].FG && row[j].BG == row[i].BG {
				j++
			}

			seg.Reset()
			for _, cell := range row[i:j] {
				if cell.R == 0 {
					seg.WriteRune(' ')
				} else {
					seg.WriteRune(cell.R)
				}
			}

			if row[i].FG == "" && row[i].BG == "" {
				b.WriteString(seg.String())
			} else {
				st := lipgloss.NewStyle()
				if row[i].FG != "" {
					st = st.Foreground(lipgloss.Color(row[i].FG))
				}
				if row[i].BG != "" {
					st = st.Background(lipgloss.Color(row[i].BG))
				}
				b.WriteString(st.Render(seg.String()))
			}
			i = j
		}
		lines[y] = b.String()
	}
	return lines
}
