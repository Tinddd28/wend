package chart

import (
	"fmt"
	"math"

	"github.com/Tinddd28/wend/internal/humanize"
	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/canvas"
)

const (
	treemapMaxDepth = 3
	// Минимальный размер плитки, при котором есть смысл раскладывать внутри
	// неё потомков: меньше — получится каша из однопиксельных полосок.
	nestMinW, nestMinH = 10, 5
	// Минимальный размер плитки для подписи.
	labelMinW, labelMinH = 7, 1
)

type rect struct{ x, y, w, h float64 }

// Treemap рисует squarified treemap: площадь плитки пропорциональна размеру
// узла, а алгоритм раскладки стремится к квадратным пропорциям — вытянутые
// полоски и площадь сравнивать нельзя, и подписать невозможно.
func Treemap(cv *canvas.Canvas, o Opts) {
	w, h := cv.Width(), cv.Height()
	if w < 4 || h < 2 {
		return
	}
	// Раскладка считается в системе, где ячейка квадратная (высота ×2),
	// иначе «квадратные» плитки выходили бы вдвое приплюснутыми.
	draw(cv, o, o.Root, rect{0, 0, float64(w), float64(h) * cellAspect}, 1)
}

func draw(cv *canvas.Canvas, o Opts, parent *scan.DirNode, r rect, depth int) {
	kids := o.Tree.SortedChildren(parent)
	total := sumSizes(kids)
	if total <= 0 {
		return
	}

	area := r.w * r.h
	items := make([]item, 0, len(kids))
	for _, k := range kids {
		a := area * float64(k.Size) / float64(total)
		if a < 1 {
			continue // плитка меньше половины ячейки — не отрисуется
		}
		items = append(items, item{node: k, area: a})
	}

	for _, t := range squarify(items, r) {
		x0, y0 := int(math.Round(t.x)), int(math.Round(t.y/cellAspect))
		x1, y1 := int(math.Round(t.x+t.w)), int(math.Round((t.y+t.h)/cellAspect))
		tw, th := x1-x0, y1-y0
		if tw < 1 || th < 1 {
			continue
		}

		hue := 360 * float64(t.x+t.w/2) / float64(cv.Width())
		bg := Color(hue, depth, !related(t.node.Path, o.Highlight))
		cv.Fill(x0, y0, tw, th, bg)

		// Вложенность рисуется с отступом в 1 ячейку — оставшаяся рамка
		// родительского цвета и служит визуальной границей группы.
		if depth < treemapMaxDepth && t.node.Kind == scan.KindDir && tw >= nestMinW && th >= nestMinH {
			inner := rect{t.x + 1, t.y + cellAspect, t.w - 2, t.h - 2*cellAspect}
			label(cv, t.node, x0+1, y0, tw-2, bg)
			draw(cv, o, t.node, inner, depth+1)
			continue
		}
		if tw >= labelMinW && th >= labelMinH {
			label(cv, t.node, x0, y0, tw, bg)
		}
	}
}

// label подписывает плитку: имя, а если хватает места — ещё и размер.
func label(cv *canvas.Canvas, n *scan.DirNode, x, y, w int, bg string) {
	fg := Contrast(bg)
	name := n.Name
	if n.Kind == scan.KindDir {
		name += "/"
	}
	if full := fmt.Sprintf("%s %s", name, humanize.Bytes(n.Size)); len([]rune(full)) <= w {
		name = full
	}
	cv.Text(x, y, clip(name, w), fg, "")
}

func clip(s string, w int) string {
	r := []rune(s)
	if w <= 0 {
		return ""
	}
	if len(r) <= w {
		return string(r)
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

type item struct {
	node *scan.DirNode
	area float64
}

// squarify — классический алгоритм Bruls, Huizing, van Wijk (2000):
// плитки набираются в ряд вдоль короткой стороны свободного прямоугольника,
// пока это улучшает худшее соотношение сторон в ряду.
func squarify(items []item, r rect) []item2 {
	var out []item2
	for len(items) > 0 && r.w > 0 && r.h > 0 {
		side := math.Min(r.w, r.h)

		n := 1
		best := worstRatio(items[:1], side)
		for n < len(items) {
			cand := worstRatio(items[:n+1], side)
			if cand > best {
				break
			}
			best = cand
			n++
		}

		row := items[:n]
		var rowArea float64
		for _, it := range row {
			rowArea += it.area
		}

		if r.w >= r.h {
			cw := rowArea / r.h
			y := r.y
			for _, it := range row {
				th := it.area / cw
				out = append(out, item2{rect{r.x, y, cw, th}, it.node})
				y += th
			}
			r.x, r.w = r.x+cw, r.w-cw
		} else {
			ch := rowArea / r.w
			x := r.x
			for _, it := range row {
				tw := it.area / ch
				out = append(out, item2{rect{x, r.y, tw, ch}, it.node})
				x += tw
			}
			r.y, r.h = r.y+ch, r.h-ch
		}
		items = items[n:]
	}
	return out
}

type item2 struct {
	rect
	node *scan.DirNode
}

// worstRatio возвращает худшее (наибольшее) соотношение сторон среди плиток
// ряда, уложенного вдоль стороны длиной side.
func worstRatio(row []item, side float64) float64 {
	var sum, mx float64
	mn := math.MaxFloat64
	for _, it := range row {
		sum += it.area
		mx = math.Max(mx, it.area)
		mn = math.Min(mn, it.area)
	}
	if sum <= 0 || mn <= 0 || side <= 0 {
		return math.MaxFloat64
	}
	s2, l2 := sum*sum, side*side
	return math.Max(l2*mx/s2, s2/(l2*mn))
}
