package scan

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

// globalItems resolves per-user cache locations that exist on this machine.
func globalItems(env *rules.Env) []*core.Item {
	var out []*core.Item
	for _, r := range rules.All() {
		if r.Kind != rules.Global || !r.Supported(env.GOOS) || r.Paths == nil {
			continue
		}
		var existing []string
		seen := map[string]bool{}
		for _, p := range r.Paths(env) {
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			if fi, err := os.Lstat(p); err == nil && fi.IsDir() {
				existing = append(existing, p)
			}
		}
		if len(existing) == 0 {
			continue
		}
		if r.PerChild {
			for _, root := range existing {
				out = append(out, childItems(env, r, root)...)
			}
			continue
		}
		it := baseItem(env, r, r.Name, existing)
		if r.Command != nil {
			it.Command = r.Command(env)
		}
		out = append(out, it)
	}
	return out
}

func childItems(env *rules.Env, r *rules.Rule, root string) []*core.Item {
	depth := r.ChildDepth
	if depth < 1 {
		depth = 1
	}
	var dirs []string
	level := []string{root}
	for d := 0; d < depth; d++ {
		var next []string
		for _, dir := range level {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() && e.Name()[0] != '.' {
					next = append(next, filepath.Join(dir, e.Name()))
				}
			}
		}
		level = next
	}
	dirs = level
	sort.Strings(dirs)

	var out []*core.Item
	for _, d := range dirs {
		rel, _ := filepath.Rel(root, d)
		name := filepath.ToSlash(rel)
		if r.ChildName != nil {
			name = r.ChildName(name)
		}
		out = append(out, baseItem(env, r, name, []string{d}))
	}
	return out
}

func baseItem(env *rules.Env, r *rules.Rule, name string, paths []string) *core.Item {
	return &core.Item{
		ID:        r.ID + ":" + paths[0],
		RuleID:    r.ID,
		Ecosystem: r.Ecosystem,
		Source:    r.Source,
		Name:      name,
		Paths:     paths,
		Safety:    r.Safety,
		Why:       r.Why,
		Restore:   r.Restore,
		ModTime:   newestShallow(paths...),
	}
}
