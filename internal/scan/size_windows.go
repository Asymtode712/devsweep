//go:build windows

package scan

import "os"

// statOf on Windows uses apparent size; hardlinks are rare enough in dev
// caches there that link accounting is skipped.
func statOf(fi os.FileInfo) (int64, fileKey, uint64) {
	return fi.Size(), fileKey{}, 1
}
