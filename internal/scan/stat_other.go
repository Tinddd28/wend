//go:build !unix

package scan

import "io/fs"

// statDetails — заглушка для платформ без syscall.Stat_t: inode недоступен,
// поэтому дедупликация жёстких ссылок там просто не срабатывает.
func statDetails(info fs.FileInfo) (alloc int64, ino, nlink uint64) {
	return info.Size(), 0, 1
}
