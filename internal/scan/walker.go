package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

// walker finds per-project artifacts under the scan roots.
type walker struct {
	env      *rules.Env
	byName   map[string][]*rules.Rule
	venv     *rules.Rule
	maxDepth int
	sem      chan struct{}
	dirs     atomic.Int64
	found    atomic.Int64

	mu    sync.Mutex
	items []*core.Item
}

// Directories never worth descending into: other tools' private trees (which
// contain package.json files we must not treat as projects) and huge media.
var skipAnywhere = map[string]bool{
	"node_modules": true, "site-packages": true, "dist-packages": true, "__pycache__": true,
	".git": true, ".hg": true, ".svn": true,
}

var skipUnderHome = map[string]bool{
	"Library": true, "Applications": true, "Pictures": true, "Music": true, "Movies": true,
	"AppData": true, "snap": true,
	"miniconda3": true, "anaconda3": true, "miniforge3": true, "mambaforge": true,
}

var macProtected = map[string]bool{"Desktop": true, "Documents": true, "Downloads": true}

func newWalker(env *rules.Env, maxDepth, parallel int) *walker {
	return &walker{
		env:      env,
		byName:   rules.ProjectDirNames(),
		venv:     rules.ByID("python.venv"),
		maxDepth: maxDepth,
		sem:      make(chan struct{}, parallel),
	}
}

func (w *walker) run(roots []string) []*core.Item {
	var wg sync.WaitGroup
	for _, r := range roots {
		wg.Add(1)
		w.walk(r, 0, nil, &wg)
	}
	wg.Wait()
	return w.items
}

func (w *walker) skip(path, name string, depth int) bool {
	if skipAnywhere[name] || strings.HasPrefix(name, ".") {
		return true
	}
	parent := filepath.Dir(path)
	if parent == w.env.Home && skipUnderHome[name] {
		return true
	}
	// macOS blocks reads of these until the user answers a privacy prompt,
	// which would hang the scan. They're scanned only when passed as a root.
	if parent == w.env.Home && w.env.GOOS == "darwin" && macProtected[name] {
		return true
	}
	// GOPATH's module cache is full of other people's projects.
	if name == "pkg" && filepath.Base(parent) == "go" {
		return true
	}
	return false
}

func (w *walker) walk(dir string, depth int, parentNames map[string]bool, wg *sync.WaitGroup) {
	defer wg.Done()
	w.dirs.Add(1)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}

	// A virtualenv can have any name; it's identified by pyvenv.cfg.
	if depth > 0 && names["pyvenv.cfg"] {
		w.emit(w.venv, dir, parentNames)
		return
	}

	for _, e := range entries {
		if !e.IsDir() { // symlinks report false here and are never followed
			continue
		}
		name := e.Name()
		path := filepath.Join(dir, name)
		if w.match(name, path, names) {
			continue
		}
		// .venv and friends are hidden, so check them before the hidden-dir skip.
		if strings.HasPrefix(name, ".") && depth > 0 {
			if _, err := os.Lstat(filepath.Join(path, "pyvenv.cfg")); err == nil {
				w.emit(w.venv, path, names)
				continue
			}
		}
		if depth+1 > w.maxDepth || w.skip(path, name, depth+1) {
			continue
		}
		wg.Add(1)
		select {
		case w.sem <- struct{}{}:
			go func() {
				defer func() { <-w.sem }()
				w.walk(path, depth+1, names, wg)
			}()
		default:
			w.walk(path, depth+1, names, wg)
		}
	}
}

// match checks a child directory against project rules and emits an item.
func (w *walker) match(name, path string, siblings map[string]bool) bool {
	for _, r := range w.byName[name] {
		if !r.MatchMarker(siblings) {
			continue
		}
		if r.InnerMarker != "" {
			if _, err := os.Lstat(filepath.Join(path, r.InnerMarker)); err != nil {
				continue
			}
		}
		w.emit(r, path, siblings)
		return true
	}
	return false
}

func (w *walker) emit(r *rules.Rule, path string, siblings map[string]bool) {
	project := filepath.Dir(path)
	it := &core.Item{
		ID:        path,
		RuleID:    r.ID,
		Ecosystem: r.Ecosystem,
		Source:    r.Source,
		Name:      Tilde(w.env, path),
		Paths:     []string{path},
		Safety:    r.Safety,
		Why:       r.Why,
		Restore:   r.Restore,
		Project:   project,
		ModTime:   projectActivity(path, project, siblings),
	}
	if lock, ok := r.HasLock(siblings); !ok {
		it.Safety = it.Safety.Downgrade()
		it.Note = r.NoLockNote
	} else if lock != "" && r.LockRestore != nil {
		it.Restore = r.LockRestore[lock]
	}
	if it.Restore != "" {
		it.Restore = "cd " + shellQuote(Tilde(w.env, project)) + " && " + it.Restore
	}
	w.mu.Lock()
	w.items = append(w.items, it)
	w.mu.Unlock()
	w.found.Add(1)
}

// projectActivity is when the project was last touched: the newest of the
// artifact dir itself and the files in the project root.
func projectActivity(path, project string, siblings map[string]bool) time.Time {
	var newest time.Time
	if fi, err := os.Lstat(path); err == nil {
		newest = fi.ModTime()
	}
	for n := range siblings {
		fi, err := os.Lstat(filepath.Join(project, n))
		if err != nil || fi.Mode()&fs.ModeType != 0 {
			continue
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest
}

// Tilde shortens a path under home to ~/…
func Tilde(env *rules.Env, p string) string {
	if env.Home != "" && (p == env.Home || strings.HasPrefix(p, env.Home+string(filepath.Separator))) {
		return "~" + p[len(env.Home):]
	}
	return p
}

// shellQuote quotes a path for copy-pasting into a shell, keeping a leading ~ expandable.
func shellQuote(p string) string {
	if !strings.ContainsAny(p, " '\"$`\\") {
		return p
	}
	rest, tilde := strings.CutPrefix(p, "~/")
	q := "'" + strings.ReplaceAll(rest, "'", `'\''`) + "'"
	if tilde {
		return "~/" + q
	}
	return q
}
