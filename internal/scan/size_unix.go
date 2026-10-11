//go:build !windows

package scan

import (
	"os"
	"syscall"
)

// statOf returns allocated bytes (not apparent size, so sparse files and APFS
// clones aren't overcounted), the inode key and the link count.
func statOf(fi os.FileInfo) (int64, fileKey, uint64) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fi.Size(), fileKey{}, 1
	}
	return int64(st.Blocks) * 512, fileKey{uint64(st.Dev), uint64(st.Ino)}, uint64(st.Nlink)
}
