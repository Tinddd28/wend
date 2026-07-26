package sidebar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Tinddd28/wend/internal/tui/style"
)

// Ширины колонок. Имя занимает весь остаток, поэтому фиксированы только
// числовые колонки — их ширина подобрана под максимальные значения
// ("1023.9 GiB", "548.3k", "12 мес.").
const (
	colBar   = 5
	colPct   = 6
	colSize  = 10
	colItems = 7
	colMod   = 8
	gap      = 1
)

// columns описывает, какие колонки помещаются в текущую ширину панели.
// В узком терминале колонки отключаются от наименее важной к самой важной,
// а не сжимаются: сжатая до трёх символов дата бесполезна, а обрезанный
// размер ещё и вводит в заблуждение.
type columns struct {
	bar   bool
	pct   bool
	size  bool
	items bool
	mod   bool
	name  int // остаток ширины под дерево имён
}

func layoutColumns(w int) columns {
	c := columns{size: true}
	rest := w - colSize - gap

	if rest > 34 {
		c.bar, c.pct = true, true
		rest -= colBar + colPct + 2*gap
	} else if rest > 18 {
		c.pct = true
		rest -= colPct + gap
	}
	if rest > 26 {
		c.items = true
		rest -= colItems + gap
	}
	if rest > 24 {
		c.mod = true
		rest -= colMod + gap
	}

	c.name = max(rest, 4)
	return c
}

// eighths — частичные блоки для «полутоновой» ширины бара: без них короткий
// бар в 5 ячеек различал бы всего 5 градаций и большинство строк выглядело
// бы одинаково пустыми.
var eighths = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

var (
	barFill  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e2455a"))
	barTrack = lipgloss.NewStyle().Foreground(lipgloss.Color("#3a3a3a"))
)

// renderBar рисует долю frac (0..1) полосой шириной colBar.
func renderBar(frac float64) string {
	frac = min(max(frac, 0), 1)
	units := int(frac*float64(colBar)*8 + 0.5)
	full, rem := units/8, units%8

	var b strings.Builder
	b.WriteString(strings.Repeat("█", full))
	if rem > 0 && full < colBar {
		b.WriteRune(eighths[rem-1])
		full++
	}
	return barFill.Render(b.String()) + barTrack.Render(strings.Repeat("╌", colBar-full))
}

func renderPct(frac float64) string {
	p := frac * 100
	if p >= 99.95 {
		return fmt.Sprintf("%*s", colPct, "100%")
	}
	return fmt.Sprintf("%*.1f%%", colPct-1, p)
}

// header рисует шапку таблицы теми же колонками, что и строки.
func (c columns) header(w int) string {
	var b strings.Builder
	if c.bar {
		b.WriteString(strings.Repeat(" ", colBar+gap))
	}
	if c.pct {
		b.WriteString(fmt.Sprintf("%*s ", colPct, "доля"))
	}
	b.WriteString(style.Cell("объект", c.name))
	b.WriteString(fmt.Sprintf(" %*s", colSize, "размер"))
	if c.items {
		b.WriteString(fmt.Sprintf(" %*s", colItems, "элем."))
	}
	if c.mod {
		b.WriteString(fmt.Sprintf(" %*s", colMod, "изменён"))
	}
	return style.ColumnHeader.Render(style.Cell(b.String(), w))
}
