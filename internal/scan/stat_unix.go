//go:build unix

package scan

import (
	"io/fs"
	"syscall"
)

// statDetails достаёт из FileInfo данные, которых нет в переносимом io/fs:
// реально занятое на диске место, inode и число жёстких ссылок.
//
// Alloc (blocks*512) отличается от Size для sparse-файлов и из-за округления
// до размера блока: «логический» размер каталога с тысячами мелких файлов
// систематически занижает реальный расход диска.
func statDetails(info fs.FileInfo) (alloc int64, ino, nlink uint64) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size(), 0, 1
	}
	return st.Blocks * 512, st.Ino, uint64(st.Nlink)
}
