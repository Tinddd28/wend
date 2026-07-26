package chart

import (
	"math"
	"sort"

	"github.com/Tinddd28/wend/internal/humanize"
	"github.com/Tinddd28/wend/internal/scan"
	"github.com/Tinddd28/wend/internal/tui/canvas"
)

// cellAspect — во сколько раз символьная ячейка выше своей ширины. Без этой
// поправки «круг» выходил бы вдвое сплюснутым по вертикали.
const cellAspect = 2.0

const (
	maxRings    = 5
	minRingSpan = 2.2  // минимальная толщина кольца в единицах радиуса
	minArcDeg   = 0.45 // сегменты уже этого не видны и только съедают время
	gapCells    = 0.7  // ширина тёмного зазора между сегментами, в ячейках
	radialGap   = 0.32 // зазор между кольцами, в единицах радиуса
)

// arc — сегмент кольца: узел, его угловой диапазон (градусы, 0 — вверх,
// по часовой стрелке) и глубина вложенности, равная номеру кольца.
type arc struct {
	node  *scan.DirNode
	a0    float64
	a1    float64
	color string
}

// Rings рисует кольцевую диаграмму (sunburst): центр — суммарный объём,
// каждое следующее кольцо — уровень вложенности, угловой размер сегмента
// пропорционален размеру узла.
func Rings(cv *canvas.Canvas, o Opts) {
	w, h := cv.Width(), cv.Height()
	if w < 8 || h < 5 {
		return
	}

	cx, cy := float64(w-1)/2, float64(h-1)/2
	// Радиус измеряется в «горизонтальных» единицах: по вертикали доступно
	// h ячеек, что после умножения dy на cellAspect даёт h единиц.
	maxR := min(float64(w)/2, float64(h)) * 0.97
	holeR := max(maxR*0.24, 2.5)

	rings := int((maxR - holeR) / minRingSpan)
	rings = min(max(rings, 1), maxRings)
	ringW := (maxR - holeR) / float64(rings)

	byDepth := buildArcs(o, rings)

	for y := range h {
		for x := range w {
			dx := float64(x) - cx
			dy := (float64(y) - cy) * cellAspect
			r := math.Hypot(dx, dy)

			if r < holeR || r > maxR {
				continue
			}
			depth := int((r-holeR)/ringW) + 1
			if depth > rings {
				continue
			}
			// Радиальный зазор отделяет кольца друг от друга.
			if depth > 1 && math.Mod(r-holeR, ringW) < radialGap {
				continue
			}

			// 0° — вверх, далее по часовой стрелке.
			theta := math.Mod(math.Atan2(dx, -dy)*180/math.Pi+360, 360)

			a := findArc(byDepth[depth], theta)
			if a == nil {
				continue
			}
			// Угловой зазор постоянной ширины в ячейках: у внешних колец
			// он в градусах меньше, иначе широкие кольца «разъедало» бы.
			if a.a1-a.a0 < 359 {
				gap := gapCells / r * 180 / math.Pi / 2
				if theta-a.a0 < gap || a.a1-theta < gap {
					continue
				}
			}
			cv.Set(x, y, canvas.Cell{R: ' ', BG: a.color})
		}
	}

	drawHub(cv, o, int(cx), int(cy))
}

// drawHub подписывает центр диаграммы суммарным объёмом и числом элементов.
func drawHub(cv *canvas.Canvas, o Opts, cx, cy int) {
	total := humanize.Bytes(o.Root.Size)
	cv.Text(cx-len([]rune(total))/2, cy, total, "", "")
	if items := humanize.Count(o.Root.Items); cv.Height() > 8 {
		cv.Text(cx-len([]rune(items))/2, cy+1, items, "", "")
	}
}

// buildArcs раскладывает поддерево на угловые сегменты по кольцам.
// Тон берётся из углового положения самого сегмента, поэтому диаграмма
// читается как непрерывная радуга, а вложенные сегменты сохраняют тон
// родителя (они лежат внутри его углового диапазона) и отличаются светлотой.
func buildArcs(o Opts, rings int) map[int][]arc {
	byDepth := make(map[int][]arc, rings)

	var rec func(n *scan.DirNode, a0, a1 float64, depth int)
	rec = func(n *scan.DirNode, a0, a1 float64, depth int) {
		if depth > rings || a1-a0 < minArcDeg {
			return
		}
		kids := o.Tree.SortedChildren(n)
		total := sumSizes(kids)
		if total <= 0 {
			return
		}

		cur := a0
		for _, k := range kids {
			span := (a1 - a0) * float64(k.Size) / float64(total)
			if span < minArcDeg {
				cur += span
				continue
			}
			hue := (cur + span/2) - 90 // сдвиг, чтобы красное было справа
			byDepth[depth] = append(byDepth[depth], arc{
				node:  k,
				a0:    cur,
				a1:    cur + span,
				color: Color(hue, depth, !related(k.Path, o.Highlight)),
			})
			if k.Kind == scan.KindDir {
				rec(k, cur, cur+span, depth+1)
			}
			cur += span
		}
	}
	rec(o.Root, 0, 360, 1)

	for d := range byDepth {
		sort.Slice(byDepth[d], func(i, j int) bool { return byDepth[d][i].a0 < byDepth[d][j].a0 })
	}
	return byDepth
}

// findArc ищет сегмент, накрывающий угол theta. Сегменты одного кольца
// отсортированы и не пересекаются, поэтому достаточно бинарного поиска —
// линейный перебор на каждую из тысяч ячеек заметно тормозил бы перерисовку.
func findArc(arcs []arc, theta float64) *arc {
	i := sort.Search(len(arcs), func(i int) bool { return arcs[i].a1 > theta })
	if i < len(arcs) && arcs[i].a0 <= theta {
		return &arcs[i]
	}
	return nil
}
