package clean

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Asymtode712/devsweep/internal/rules"
)

// moveToTrash renames p into the user's trash. It never copies: a trash on
// another volume would mean duplicating gigabytes, so that's reported instead.
func moveToTrash(env *rules.Env, p string) error {
	if _, err := os.Lstat(p); err != nil {
		return nil
	}
	switch env.GOOS {
	case "darwin":
		return renameInto(p, filepath.Join(env.Home, ".Trash"), filepath.Base(p))
	case "windows":
		return ErrTrashUnsupported
	}
	// freedesktop.org trash spec
	base := filepath.Join(env.Home, ".local", "share", "Trash")
	if v := env.Getenv("XDG_DATA_HOME"); v != "" {
		base = filepath.Join(v, "Trash")
	}
	files, info := filepath.Join(base, "files"), filepath.Join(base, "info")
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(info, 0o700); err != nil {
		return err
	}
	name := uniqueName(files, filepath.Base(p))
	trashInfo := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		(&url.URL{Path: p}).EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
	if err := os.WriteFile(filepath.Join(info, name+".trashinfo"), []byte(trashInfo), 0o600); err != nil {
		return err
	}
	if err := renameInto(p, files, name); err != nil {
		os.Remove(filepath.Join(info, name+".trashinfo"))
		return err
	}
	return nil
}

func renameInto(p, dir, name string) error {
	dest := filepath.Join(dir, uniqueName(dir, name))
	err := os.Rename(p, dest)
	if errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("%s is on a different volume than the Trash; use permanent delete", p)
	}
	return err
}

func uniqueName(dir, name string) string {
	candidate := name
	for i := 2; ; i++ {
		if _, err := os.Lstat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s %d", name, i)
	}
}
