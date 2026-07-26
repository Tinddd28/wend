package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// makeFixture создаёт временное дерево:
//
//	root/
//	  a.txt         (100 байт)
//	  sub/
//	    b.txt       (50 байт)
//	    c.txt       (25 байт)
func makeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(os.WriteFile(filepath.Join(root, "a.txt"), make([]byte, 100), 0o644))
	must(os.Mkdir(filepath.Join(root, "sub"), 0o755))
	must(os.WriteFile(filepath.Join(root, "sub", "b.txt"), make([]byte, 50), 0o644))
	must(os.WriteFile(filepath.Join(root, "sub", "c.txt"), make([]byte, 25), 0o644))

	return root
}

// runScan прогоняет Scan до конца и возвращает наполненное дерево.
func runScan(t *testing.T, root string) *DirTree {
	t.Helper()

	tree := NewDirTree(root)
	ch := Scan(context.Background(), root)

	for res := range ch {
		if res.Err != nil {
			t.Fatalf("scan error: %v", res.Err)
		}
		for _, e := range res.Entries {
			tree.Upsert(e)
		}
		if res.Done {
			break
		}
	}
	return tree
}

func TestScan_AggregatesSizesBottomUp(t *testing.T) {
	root := makeFixture(t)
	tree := runScan(t, root)

	const wantTotal = 100 + 50 + 25
	if got := tree.Root().Size; got != wantTotal {
		t.Errorf("root size = %d, want %d", got, wantTotal)
	}

	sub := tree.Node(filepath.Join(root, "sub"))
	if sub == nil {
		t.Fatal("sub directory not found in tree")
	}
	if got := sub.Size; got != 75 {
		t.Errorf("sub size = %d, want 75", got)
	}
	if sub.Kind != KindDir {
		t.Errorf("sub kind = %v, want KindDir", sub.Kind)
	}
}

func TestScan_ChildrenSortedBySizeDesc(t *testing.T) {
	root := makeFixture(t)
	tree := runScan(t, root)

	children := tree.Children(root)
	if len(children) != 2 {
		t.Fatalf("root children count = %d, want 2", len(children))
	}
	// a.txt (100) должен быть перед sub/ (75).
	if children[0].Name != "a.txt" || children[1].Name != "sub" {
		t.Errorf("unexpected order: %s, %s", children[0].Name, children[1].Name)
	}

	subChildren := tree.Children(filepath.Join(root, "sub"))
	if len(subChildren) != 2 || subChildren[0].Name != "b.txt" || subChildren[1].Name != "c.txt" {
		t.Errorf("unexpected sub children order: %+v", subChildren)
	}
}

func TestScan_RespectsCancellation(t *testing.T) {
	root := makeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := Scan(ctx, root)
	for range ch {
		// просто дренируем канал — тест проверяет, что Scan не виснет
		// и корректно закрывает канал после отмены контекста.
	}
}

// TestScan_DeduplicatesHardLinks — файл, на который ведёт несколько жёстких
// ссылок, занимает место на диске один раз; без дедупликации по inode его
// размер засчитывался бы столько раз, сколько ссылок встретилось при обходе.
func TestScan_DeduplicatesHardLinks(t *testing.T) {
	root := makeFixture(t)
	if err := os.Link(filepath.Join(root, "a.txt"), filepath.Join(root, "a-link.txt")); err != nil {
		t.Skipf("жёсткие ссылки недоступны: %v", err)
	}

	tree := runScan(t, root)

	const wantTotal = 100 + 50 + 25
	if got := tree.Root().Size; got != wantTotal {
		t.Errorf("root size = %d, want %d (ссылка не должна учитываться дважды)", got, wantTotal)
	}
	if _, _, dups := tree.Stats(); dups != 1 {
		t.Errorf("пропущено дублей = %d, want 1", dups)
	}
	// Обе ссылки остаются в дереве, но ровно одна помечена как повтор и
	// вносит в суммы ноль. Какая именно — зависит от порядка обхода, поэтому
	// проверяем пару, а не конкретное имя.
	first, second := tree.Node(filepath.Join(root, "a-link.txt")), tree.Node(filepath.Join(root, "a.txt"))
	if first == nil || second == nil {
		t.Fatal("узел жёсткой ссылки пропал из дерева")
	}
	if first.Dup == second.Dup {
		t.Fatalf("ожидался ровно один повтор, получено Dup=%v/%v", first.Dup, second.Dup)
	}
	dup, orig := first, second
	if second.Dup {
		dup, orig = second, first
	}
	if dup.Size != 0 {
		t.Errorf("размер повтора = %d, want 0 (в суммы не входит)", dup.Size)
	}
	if orig.Size != 100 {
		t.Errorf("размер оригинала = %d, want 100", orig.Size)
	}
}
