package scan

import (
	"path/filepath"
	"testing"
)

// TestUpsert_DirEntryDoesNotResetAccumulatedSize — сканер присылает для
// директории size==0. Раньше Upsert присваивал его существующему узлу и
// вычитал уже учтённый размер детей из всех предков; порядок WalkDir это
// маскировал, но контракт Upsert порядка не гарантирует.
func TestUpsert_DirEntryDoesNotResetAccumulatedSize(t *testing.T) {
	root := filepath.FromSlash("/root")
	tree := NewDirTree(root)

	sub := filepath.Join(root, "sub")
	tree.Upsert(Entry{Path: filepath.Join(sub, "a.bin"), Kind: KindFile, Size: 100})
	tree.Upsert(Entry{Path: filepath.Join(sub, "b.bin"), Kind: KindFile, Size: 50})

	// Запись самой директории приходит после её содержимого.
	tree.Upsert(Entry{Path: sub, Kind: KindDir})

	if got := tree.Node(sub).Size; got != 150 {
		t.Errorf("sub size = %d, want 150", got)
	}
	if got := tree.Root().Size; got != 150 {
		t.Errorf("root size = %d, want 150", got)
	}
	if got := tree.Node(sub).Kind; got != KindDir {
		t.Errorf("sub kind = %v, want KindDir", got)
	}
}

// TestReset_ClearsTreeForRescan — без Reset повторный скан суммировался бы
// поверх предыдущего и удваивал все размеры.
func TestReset_ClearsTreeForRescan(t *testing.T) {
	root := filepath.FromSlash("/root")
	tree := NewDirTree(root)

	fill := func() {
		tree.Upsert(Entry{Path: filepath.Join(root, "sub"), Kind: KindDir, Size: 0})
		tree.Upsert(Entry{Path: filepath.Join(root, "sub", "a.bin"), Kind: KindFile, Size: 100})
	}

	fill()
	tree.Reset()
	fill()

	if got := tree.Root().Size; got != 100 {
		t.Errorf("root size после rescan = %d, want 100", got)
	}
	if got := len(tree.Children(root)); got != 1 {
		t.Errorf("root children после rescan = %d, want 1", got)
	}
	files, dirs, _ := tree.Stats()
	if files != 1 || dirs != 1 {
		t.Errorf("stats = %d файлов / %d папок, want 1/1", files, dirs)
	}
}

// TestChildren_StableOrderOnTies — без тайбрейка по имени узлы одинакового
// размера переставлялись на каждой перерисовке во время скана.
func TestChildren_StableOrderOnTies(t *testing.T) {
	root := filepath.FromSlash("/root")
	tree := NewDirTree(root)
	for _, n := range []string{"c", "a", "b"} {
		tree.Upsert(Entry{Path: filepath.Join(root, n), Kind: KindDir, Size: 0})
	}

	for range 5 {
		kids := tree.Children(root)
		if len(kids) != 3 || kids[0].Name != "a" || kids[1].Name != "b" || kids[2].Name != "c" {
			t.Fatalf("нестабильный порядок: %s %s %s", kids[0].Name, kids[1].Name, kids[2].Name)
		}
	}
}
