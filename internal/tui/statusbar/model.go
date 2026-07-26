// Package statusbar рендерит нижние строки: детали выбранного объекта,
// подсказки по клавишам, прогресс скана и ошибки.
package statusbar

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tinddd28/wend/internal/humanize"
	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/style"
)

// Height — сколько строк занимает статусбар: строка деталей + строка подсказок.
const Height = 2

type Model struct {
	width    int
	help     string
	scanning bool
	files    int
	dirs     int
	dups     int
	err      error
	sel      *scan.DirNode
}

func New() Model { return Model{} }

func (m *Model) SetSize(w, _ int)            { m.width = max(w, 0) }
func (m *Model) SetHelp(s string)            { m.help = s }
func (m *Model) SetScanning(v bool)          { m.scanning = v }
func (m *Model) SetCounts(f, d, dup int)     { m.files, m.dirs, m.dups = f, d, dup }
func (m *Model) SetError(err error)          { m.err = err }
func (m *Model) SetSelected(n *scan.DirNode) { m.sel = n }

func (m Model) View() string {
	if m.width <= 0 {
		return ""
	}
	// Padding(0,1) съедает по символу с каждой стороны — обрезаем по остатку,
	// иначе длинная подсказка переносится на вторую строку и выдавливает
	// нижнюю границу панелей за пределы экрана.
	inner := max(m.width-2, 0)

	detail := style.Footer.Width(m.width).Render(
		style.Detail.Render(style.Clip(m.detailLine(), inner)))

	var status string
	switch {
	case m.err != nil:
		status = style.Error.Render(style.Clip("ошибка обхода: "+m.err.Error(), inner))
	default:
		status = style.Clip(m.statusLine()+"  ·  "+m.help, inner)
	}

	return detail + "\n" + style.Footer.Width(m.width).Render(status)
}

func (m Model) statusLine() string {
	prefix := ""
	if m.scanning {
		prefix = "сканирование… "
	}
	s := fmt.Sprintf("%s%s файлов / %s папок", prefix, humanize.Count(m.files), humanize.Count(m.dirs))
	if m.dups > 0 {
		// Пользователю важно знать, что суммы меньше «наивных»: иначе
		// расхождение с du/ls выглядит как баг.
		s += fmt.Sprintf(" / %s жёстких ссылок не учтено", humanize.Count(m.dups))
	}
	return s
}

// detailLine показывает то, что не помещается в таблицу: реальный расход
// диска, inode и число жёстких ссылок, точное время изменения.
func (m Model) detailLine() string {
	n := m.sel
	if n == nil {
		return ""
	}

	parts := []string{n.Path, humanize.Bytes(n.Size)}

	// Alloc отличается от Size из-за округления до блока и sparse-файлов;
	// показываем отдельно, только когда расхождение заметное.
	if n.Alloc > 0 && diffNoticeable(n.Size, n.Alloc) {
		parts = append(parts, "на диске "+humanize.Bytes(n.Alloc))
	}
	if n.Kind == scan.KindDir {
		parts = append(parts, humanize.Count(n.Items)+" элем.")
	}
	if n.Inode != 0 {
		parts = append(parts, fmt.Sprintf("inode %d", n.Inode))
	}
	if n.Nlink > 1 {
		parts = append(parts, fmt.Sprintf("ссылок %d", n.Nlink))
	}
	if n.Dup {
		parts = append(parts, "повтор жёсткой ссылки, в сумме не учтён")
	}
	if !n.ModTime.IsZero() {
		parts = append(parts, n.ModTime.Format(time.DateTime))
	}
	return strings.Join(parts, "  ·  ")
}

func diffNoticeable(size, alloc int64) bool {
	d := size - alloc
	if d < 0 {
		d = -d
	}
	return d*10 > size
}
