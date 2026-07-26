package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Entry — один элемент ФС, полученный во время скана.
type Entry struct {
	Path    string
	Kind    NodeKind
	Size    int64 // логический размер
	Alloc   int64 // фактически занято на диске (blocks*512)
	ModTime time.Time
	Inode   uint64
	Nlink   uint64
	// Dup — файл уже учтён в этом скане по другой жёсткой ссылке. Сам узел
	// показывается со своим размером, но в суммы предков не входит.
	Dup bool
}

// Result — сообщение, отправляемое в канал сканером: либо батч Entry,
// либо финальный сигнал завершения (Done=true) с ошибкой, если она была.
type Result struct {
	Entries []Entry
	Done    bool
	Err     error
}

const batchSize = 256

// Scan обходит root в отдельной горутине и стримит результаты батчами через
// канал, чтобы вызывающий код (bubbletea) мог рендерить дерево инкрементально,
// а не ждать полного завершения обхода. Отмена ctx останавливает обход —
// канал в любом случае закрывается ровно один раз, финальным Result{Done: true}.
//
// Реализация однопоточная (filepath.WalkDir): для локальных дисков этого
// достаточно и это самый простой корректный вариант. Конкурентный worker pool
// имеет смысл добавлять отдельно, если профилирование покажет, что обход —
// узкое место (сетевые ФС, много мелких файлов).
func Scan(ctx context.Context, root string) <-chan Result {
	out := make(chan Result)

	go func() {
		defer close(out)

		// seen хранит inode файлов с nlink > 1: без этого файл, на который
		// ведёт несколько жёстких ссылок, засчитывался бы в общий размер
		// столько раз, сколько ссылок встретилось при обходе.
		seen := make(map[uint64]struct{})

		batch := make([]Entry, 0, batchSize)
		flush := func() bool {
			if len(batch) == 0 {
				return true
			}
			select {
			case out <- Result{Entries: batch}:
				batch = make([]Entry, 0, batchSize)
				return true
			case <-ctx.Done():
				return false
			}
		}

		walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				// Пропускаем недоступные записи (permission denied и т.п.),
				// не прерывая обход остального дерева.
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			e := Entry{Path: path, Kind: KindFile}
			if d.IsDir() {
				e.Kind = KindDir
			}

			if info, ierr := d.Info(); ierr == nil {
				e.ModTime = info.ModTime()
				alloc, ino, nlink := statDetails(info)
				e.Inode, e.Nlink = ino, nlink
				if e.Kind == KindFile {
					e.Size, e.Alloc = info.Size(), alloc
					if nlink > 1 && ino != 0 {
						if _, ok := seen[ino]; ok {
							e.Dup = true
						} else {
							seen[ino] = struct{}{}
						}
					}
				}
			}

			batch = append(batch, e)
			if len(batch) >= batchSize && !flush() {
				return ctx.Err()
			}
			return nil
		})

		flush()

		if errors.Is(walkErr, context.Canceled) {
			walkErr = nil
		}
		// select обязателен: после отмены получатель уже не читает канал,
		// и безусловная отправка навсегда подвесила бы эту горутину.
		select {
		case out <- Result{Done: true, Err: walkErr}:
		case <-ctx.Done():
		}
	}()

	return out
}
