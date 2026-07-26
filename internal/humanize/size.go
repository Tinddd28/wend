// Package humanize содержит форматирование значений для UI.
package humanize

import (
	"fmt"
	"time"
)

// Bytes форматирует размер в бинарных единицах (KiB/MiB/...).
// Единственная реализация на весь проект: раньше она была скопирована
// в tui, sidebar и mainview и успела разойтись по ширине поля.
func Bytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

// Count компактно форматирует количество элементов: колонка «содержимое»
// узкая, а числа там доходят до сотен тысяч.
func Count(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return trim(float64(n)/1000) + "k"
	default:
		return trim(float64(n)/1_000_000) + "M"
	}
}

// trim печатает число с одним знаком после запятой, убирая бесполезный ".0".
func trim(v float64) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f", v)
	}
	s := fmt.Sprintf("%.1f", v)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

// Age форматирует «сколько назад» в узкую колонку. Нулевое время означает,
// что mtime получить не удалось.
func Age(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < 0:
		return "будущее"
	case d < 24*time.Hour:
		return "сегодня"
	case d < 48*time.Hour:
		return "вчера"
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d дн.", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d мес.", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%d г.", int(d.Hours()/24/365))
	}
}
