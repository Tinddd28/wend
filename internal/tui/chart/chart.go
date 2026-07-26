package chart

import (
	"path/filepath"
	"strings"

	"github.com/Tinddd28/wend/internal/scan"
)

// Opts — общие параметры отрисовки диаграмм.
type Opts struct {
	Tree *scan.DirTree
	// Root — узел, содержимое которого показывается (первый уровень диаграммы).
	Root *scan.DirNode
	// Highlight — путь узла под курсором: его ветка рисуется в полном цвете,
	// остальное приглушается, поэтому таблица слева и диаграмма справа
	// читаются как одно целое.
	Highlight string
}

// related сообщает, лежит ли узел на ветке выделенного пути (сам узел,
// его предок или его потомок).
func related(path, highlight string) bool {
	if highlight == "" {
		return true
	}
	if path == highlight {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(highlight, path+sep) || strings.HasPrefix(path, highlight+sep)
}

// sumSizes складывает размеры узлов. Именно эта сумма, а не Size родителя,
// служит знаменателем: у родителя размер может быть уже обновлён батчем,
// содержимое которого ещё не разложено по детям, и доли не сошлись бы в 100%.
func sumSizes(nodes []*scan.DirNode) int64 {
	var total int64
	for _, n := range nodes {
		total += n.Size
	}
	return total
}
