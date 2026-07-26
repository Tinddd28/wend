package scan

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// NodeKind различает файлы и директории в дереве.
type NodeKind int

const (
	KindFile NodeKind = iota
	KindDir
)

// DirNode — один узел дерева файловой системы.
// Size/Alloc для файла — его собственные значения; для директории — сумма
// всех потомков. Items — количество потомков (рекурсивно), для файла 0.
type DirNode struct {
	Name    string
	Path    string
	Kind    NodeKind
	Size    int64
	Alloc   int64
	Items   int
	ModTime time.Time
	Inode   uint64
	Nlink   uint64
	Dup     bool

	Children []*DirNode
	Parent   *DirNode
}

// Depth возвращает расстояние до корня дерева.
func (n *DirNode) Depth() int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// DirTree — потокобезопасное in-memory хранилище результатов скана.
// Живёт как *DirTree внутри RootModel: копирование Model остаётся дешёвым,
// т.к. копируется только указатель, а не содержимое дерева.
type DirTree struct {
	mu    sync.RWMutex
	root  *DirNode
	files int
	dirs  int
	dups  int
}

// NewDirTree создаёт дерево с корнем в rootPath (сам корень ещё не просканирован).
func NewDirTree(rootPath string) *DirTree {
	rootPath = filepath.Clean(rootPath)
	return &DirTree{
		root: &DirNode{
			Name: filepath.Base(rootPath),
			Path: rootPath,
			Kind: KindDir,
		},
	}
}

// Root возвращает корневой узел дерева.
func (t *DirTree) Root() *DirNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.root
}

// Stats возвращает количество учтённых файлов, директорий и файлов,
// пропущенных как повторные жёсткие ссылки.
func (t *DirTree) Stats() (files, dirs, dups int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.files, t.dirs, t.dups
}

// Reset очищает дерево, сохраняя корневой узел. Нужен для пересканирования:
// без него повторный скан того же пути удваивал бы все размеры.
func (t *DirTree) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.root.Children = nil
	t.root.Size, t.root.Alloc, t.root.Items = 0, 0, 0
	t.files, t.dirs, t.dups = 0, 0, 0
}

// Upsert добавляет узел и распространяет его размеры и счётчик элементов на
// всех предков. Родительская цепочка создаётся по мере необходимости — это
// ожидаемо, т.к. сканер обходит дерево сверху вниз и обычно директория уже
// создана к моменту прихода её содержимого, но порядок явно не гарантируется
// между батчами.
func (t *DirTree) Upsert(e Entry) *DirNode {
	path := filepath.Clean(e.Path)

	t.mu.Lock()
	defer t.mu.Unlock()

	if path == t.root.Path {
		t.root.Kind = e.Kind
		t.root.ModTime = e.ModTime
		t.root.Inode, t.root.Nlink = e.Inode, e.Nlink
		return t.root
	}

	// Повторная жёсткая ссылка: узел показывается со своим размером, но в
	// суммы предков не попадает — иначе один и тот же экстент на диске
	// засчитывался бы столько раз, сколько на него ведёт ссылок.
	size, alloc := e.Size, e.Alloc
	if e.Dup {
		size, alloc = 0, 0
		t.dups++
	}

	parent := t.ensureParentLocked(path)

	for _, c := range parent.Children {
		if c.Path != path {
			continue
		}
		c.ModTime, c.Inode, c.Nlink, c.Dup = e.ModTime, e.Inode, e.Nlink, e.Dup
		// Размер директории — это сумма потомков, накопленная снизу вверх.
		// Сканер присылает для директории size==0, поэтому присваивать его
		// нельзя: это обнулило бы уже учтённых детей и вычло бы их размер
		// из всех предков. Сейчас нас спасал только порядок WalkDir
		// (директория приходит раньше содержимого), но он не гарантирован
		// контрактом Upsert.
		if e.Kind == KindDir {
			c.Kind = e.Kind
			return c
		}
		dSize, dAlloc := size-c.Size, alloc-c.Alloc
		c.Kind, c.Size, c.Alloc = e.Kind, size, alloc
		for p := parent; p != nil; p = p.Parent {
			p.Size += dSize
			p.Alloc += dAlloc
		}
		return c
	}

	node := &DirNode{
		Name:    filepath.Base(path),
		Path:    path,
		Kind:    e.Kind,
		Size:    size,
		Alloc:   alloc,
		ModTime: e.ModTime,
		Inode:   e.Inode,
		Nlink:   e.Nlink,
		Dup:     e.Dup,
		Parent:  parent,
	}
	parent.Children = append(parent.Children, node)
	t.countLocked(e.Kind)

	for p := parent; p != nil; p = p.Parent {
		p.Size += size
		p.Alloc += alloc
		p.Items++
	}
	return node
}

func (t *DirTree) countLocked(kind NodeKind) {
	if kind == KindDir {
		t.dirs++
	} else {
		t.files++
	}
}

// ensureParentLocked находит либо создаёт цепочку директорий-предков между
// корнем и path. Вызывающий код обязан держать t.mu на запись.
func (t *DirTree) ensureParentLocked(path string) *DirNode {
	dir := filepath.Dir(path)
	if dir == t.root.Path {
		return t.root
	}
	rel, err := filepath.Rel(t.root.Path, dir)
	if err != nil || rel == "." {
		return t.root
	}

	cur := t.root
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		var next *DirNode
		for _, c := range cur.Children {
			if c.Name == part {
				next = c
				break
			}
		}
		if next == nil {
			next = &DirNode{
				Name:   part,
				Path:   filepath.Join(cur.Path, part),
				Kind:   KindDir,
				Parent: cur,
			}
			cur.Children = append(cur.Children, next)
			t.countLocked(KindDir)
			for p := cur; p != nil; p = p.Parent {
				p.Items++
			}
		}
		cur = next
	}
	return cur
}

// Node возвращает узел по пути или nil, если он ещё не просканирован.
func (t *DirTree) Node(path string) *DirNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.findLocked(filepath.Clean(path))
}

func (t *DirTree) findLocked(path string) *DirNode {
	if path == t.root.Path {
		return t.root
	}
	rel, err := filepath.Rel(t.root.Path, path)
	if err != nil {
		return nil
	}
	cur := t.root
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		var next *DirNode
		for _, c := range cur.Children {
			if c.Name == part {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// Children возвращает снапшот дочерних узлов, отсортированный по убыванию Size —
// готовый порядок и для списка, и для диаграмм.
func (t *DirTree) Children(path string) []*DirNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := t.findLocked(filepath.Clean(path))
	if n == nil {
		return nil
	}
	out := make([]*DirNode, len(n.Children))
	copy(out, n.Children)
	sortNodes(out)
	return out
}

// SortedChildren — то же, что Children, но для уже полученного узла.
// Используется диаграммами при рекурсивном спуске, чтобы не искать узел
// по пути на каждом уровне.
func (t *DirTree) SortedChildren(n *DirNode) []*DirNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*DirNode, len(n.Children))
	copy(out, n.Children)
	sortNodes(out)
	return out
}

// sortNodes упорядочивает по убыванию размера. Тайбрейк по имени обязателен:
// во время инкрементального скана снапшот берётся на каждом батче, и без него
// узлы одинакового размера (в частности, все ещё нулевые директории)
// переставлялись бы на каждой перерисовке — список визуально «дрожал».
func sortNodes(out []*DirNode) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Name < out[j].Name
	})
}
