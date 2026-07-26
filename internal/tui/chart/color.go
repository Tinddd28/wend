// Package chart рисует диаграммы использования диска на символьном холсте:
// кольцевую (sunburst) и treemap.
package chart

import (
	"github.com/lucasb-eyer/go-colorful"
)

// Цвета сегментов задаются в HSL, а не выбираются из фиксированного списка:
// тон берётся из углового положения сегмента, поэтому диаграмма всегда
// выглядит непрерывной радугой независимо от количества элементов, а
// вложенные уровни получают тот же тон с большей светлотой — родство
// сегментов читается визуально.
const (
	baseSat   = 0.66
	baseLight = 0.46
	// depthLighten — насколько светлеет каждый следующий уровень вложенности.
	// Небольшой шаг намеренно: при 0.09 пятое кольцо выцветало почти в белый
	// и переставало читаться как цвет.
	depthLighten = 0.055
	// dimSat/dimLight — приглушение сегментов вне выделенной ветки.
	dimSat   = 0.22
	dimLight = 0.34
)

// Color возвращает hex-цвет сегмента с тоном hue (0..360) на глубине depth.
func Color(hue float64, depth int, dim bool) string {
	s, l := baseSat, baseLight+float64(depth-1)*depthLighten
	if dim {
		s, l = dimSat, dimLight+float64(depth-1)*0.05
	}
	return colorful.Hsl(norm(hue), clamp01(s), clamp01(l)).Hex()
}

// Contrast подбирает цвет текста, читаемый на фоне hex.
func Contrast(hex string) string {
	c, err := colorful.Hex(hex)
	if err != nil {
		return "#ffffff"
	}
	// Порог по L* из CIELAB, а не по «сырой» яркости RGB: жёлтый и синий
	// одной RGB-яркости воспринимаются совершенно по-разному, и текст на
	// жёлтом сегменте оказывался бы белым по светлому.
	if l, _, _ := c.Lab(); l > 0.62 {
		return "#101010"
	}
	return "#ffffff"
}

func norm(h float64) float64 {
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}
	return h
}

func clamp01(v float64) float64 { return min(max(v, 0), 1) }
