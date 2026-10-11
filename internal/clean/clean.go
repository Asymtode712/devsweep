// Package clean deletes items, after re-checking that each one still matches
// the rule that found it.
package clean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

type Options struct {
	Trash  bool
	DryRun bool
}

type Result struct {
	Item   *core.Item
	Freed  int64
	Method string // "deleted", "trashed", "command", "dry-run", "skipped"
	Err    error
}

// Validate re-checks an item right before deletion. A scan may be minutes old;
// the project could have moved, the dir could now be a symlink, or a path
// could resolve to somewhere we must never touch.
func Validate(env *rules.Env, it *core.Item) error {
	r := rules.ByID(it.RuleID)
	if r == nil {
		return fmt.Errorf("unknown rule %q", it.RuleID)
	}
	for _, p := range it.Paths {
		if err := checkPath(env, p); err != nil {
			return err
		}
	}
	if r.Kind != rules.Project {
		return nil
	}
	path := it.Paths[0]
	parent := filepath.Dir(path)
	names := map[string]bool{}
	if entries, err := os.ReadDir(parent); err == nil {
		for _, e := range entries {
			names[e.Name()] = true
		}
	}
	if r.InnerMarker != "" {
		if _, err := os.Lstat(filepath.Join(path, r.InnerMarker)); err != nil {
			return fmt.Errorf("%s no longer contains %s", path, r.InnerMarker)
		}
	}
	if len(r.Dirs) > 0 {
		base := filepath.Base(path)
		ok := false
		for _, d := range r.Dirs {
			ok = ok || d == base
		}
		if !ok || !r.MatchMarker(names) {
			return fmt.Errorf("%s no longer sits next to its project manifest", path)
		}
	}
	if tracked, _ := gitTracked(path); tracked {
		return fmt.Errorf("%s is tracked in git; refusing to delete committed files", path)
	}
	return nil
}

func checkPath(env *rules.Env, p string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return fmt.Errorf("refusing non-canonical path %q", p)
	}
	protected := []string{env.Home, env.UserCache, env.XDGCache, env.LocalAppData,
		filepath.Join(env.Home, "Library"), filepath.Join(env.Home, "Library", "Developer"),
		filepath.Join(env.Home, ".local"), filepath.Join(env.Home, ".local", "share")}
	for _, q := range protected {
		if q != "" && filepath.Clean(q) == p {
			return fmt.Errorf("refusing to delete protected directory %s", p)
		}
	}
	if strings.Count(filepath.ToSlash(p), "/") < 2 || filepath.Dir(p) == p {
		return fmt.Errorf("refusing to delete top-level path %s", p)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return nil // already gone; nothing to do
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s is now a symlink; refusing to follow it", p)
	}
	return nil
}

func gitTracked(path string) (bool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", filepath.Dir(path), "ls-files", "--", filepath.Base(path))
	out, err := cmd.Output()
	if err != nil {
		return false, nil // not a repo
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// Clean removes one item.
func Clean(env *rules.Env, it *core.Item, opts Options) Result {
	res := Result{Item: it}
	if err := Validate(env, it); err != nil {
		res.Method, res.Err = "skipped", err
		return res
	}
	if opts.DryRun {
		res.Method, res.Freed = "dry-run", it.Frees()
		return res
	}

	if len(it.Command) > 0 {
		err := runCommand(it.Command)
		if err == nil {
			res.Method, res.Freed = "command", it.Frees()
			logResult(env, res)
			return res
		}
		if len(it.Paths) == 0 {
			res.Err = err
			return res
		}
		// The tool failed (e.g. ollama's server isn't running): fall back to
		// removing the exact files the rule identified.
	}

	for _, p := range it.Paths {
		var err error
		if opts.Trash {
			err = moveToTrash(env, p)
		} else {
			err = removeAll(p)
		}
		if err != nil {
			res.Err = err
			break
		}
	}
	if res.Err == nil && it.RuleID == "ai.ollama" && len(it.Paths) > 0 && !opts.Trash {
		ollamaGC(it.Paths[0])
	}
	res.Method = "deleted"
	if opts.Trash {
		res.Method = "trashed"
	}
	if res.Err == nil {
		res.Freed = it.Frees()
	}
	logResult(env, res)
	return res
}

func runCommand(argv []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return fmt.Errorf("%s: %v %s", strings.Join(argv, " "), err, msg)
	}
	return nil
}

// removeAll handles read-only trees (Go's module cache, some Windows files)
// by making them writable and retrying.
func removeAll(p string) error {
	if err := os.RemoveAll(p); err == nil {
		return nil
	}
	filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.Type()&fs.ModeSymlink == 0 {
			if info, e := d.Info(); e == nil {
				os.Chmod(path, info.Mode().Perm()|0o700)
			}
		}
		return nil
	})
	return os.RemoveAll(p)
}

// StateDir holds devsweep's history log.
func StateDir(env *rules.Env) string {
	if env.GOOS == "windows" && env.LocalAppData != "" {
		return filepath.Join(env.LocalAppData, "devsweep")
	}
	if v := env.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, "devsweep")
	}
	return filepath.Join(env.Home, ".local", "state", "devsweep")
}

func logResult(env *rules.Env, res Result) {
	dir := StateDir(env)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "history.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := map[string]any{
		"time": time.Now().Format(time.RFC3339), "rule": res.Item.RuleID, "name": res.Item.Name,
		"paths": res.Item.Paths, "command": res.Item.Command, "freed": res.Freed,
		"method": res.Method, "restore": res.Item.Restore,
	}
	if res.Err != nil {
		entry["error"] = res.Err.Error()
	}
	json.NewEncoder(f).Encode(entry)
}

var ErrTrashUnsupported = errors.New("move to trash isn't supported on this OS; use permanent delete")
