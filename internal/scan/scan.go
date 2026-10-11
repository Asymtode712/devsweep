// Package scan discovers cleanable items and measures them.
package scan

import (
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
)

type Options struct {
	Roots      []string // project roots; default is the home directory
	NoProjects bool
	NoGlobal   bool
	MaxDepth   int
	// Progress is called periodically from a background goroutine.
	Progress func(Progress)
}

type Progress struct {
	Phase    string // "discovering" or "measuring"
	Dirs     int64
	Files    int64
	Found    int
	Measured int64
}

type Result struct {
	Items    []*core.Item
	Roots    []string
	Warnings []string
	Elapsed  time.Duration
}

func Run(env *rules.Env, opts Options) *Result {
	start := time.Now()
	var warnings []string
	if len(opts.Roots) == 0 {
		opts.Roots = []string{env.Home}
		if env.GOOS == "darwin" && !opts.NoProjects {
			warnings = append(warnings, "skipped ~/Desktop, ~/Documents, ~/Downloads (macOS privacy-protected); pass them as paths to include")
		}
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 12
	}
	parallel := runtime.NumCPU() * 4
	w := newWalker(env, opts.MaxDepth, parallel)
	sizer := NewSizer(parallel)

	var (
		mu       sync.Mutex
		items    []*core.Item
		found    atomic.Int64
		measured atomic.Int64
		phase    atomic.Value
	)
	phase.Store("discovering")
	add := func(its []*core.Item, count bool) {
		mu.Lock()
		items = append(items, its...)
		mu.Unlock()
		if count {
			found.Add(int64(len(its)))
		}
	}

	stop := make(chan struct{})
	if opts.Progress != nil {
		go func() {
			t := time.NewTicker(80 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					opts.Progress(Progress{
						Phase: phase.Load().(string), Dirs: w.dirs.Load(), Files: sizer.Files(),
						Found: int(found.Load() + w.found.Load()), Measured: measured.Load(),
					})
				}
			}
		}()
	}

	var wg sync.WaitGroup
	if !opts.NoProjects {
		wg.Add(1)
		go func() { defer wg.Done(); add(w.run(opts.Roots), false) }()
	}
	if !opts.NoGlobal {
		wg.Add(3)
		go func() { defer wg.Done(); add(globalItems(env), true) }()
		go func() { defer wg.Done(); add(ollamaItems(env), true); add(huggingFaceItems(env), true) }()
		go func() {
			defer wg.Done()
			its, warn := dockerItems(env)
			add(its, true)
			if warn != "" {
				mu.Lock()
				warnings = append(warnings, warn)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	items = dedupe(items)
	phase.Store("measuring")

	jobs := make(chan *core.Item)
	var mwg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		mwg.Add(1)
		go func() {
			defer mwg.Done()
			for it := range jobs {
				if !it.SizeKnown {
					it.Size, it.Shared = sizer.Measure(it.Paths)
					it.SizeKnown = true
				}
				measured.Add(1)
			}
		}()
	}
	for _, it := range items {
		jobs <- it
	}
	close(jobs)
	mwg.Wait()
	close(stop)

	var kept []*core.Item
	for _, it := range items {
		if it.Size > 0 {
			kept = append(kept, it)
		}
	}
	SortBySize(kept)
	return &Result{Items: kept, Roots: opts.Roots, Warnings: warnings, Elapsed: time.Since(start)}
}

func dedupe(items []*core.Item) []*core.Item {
	seen := map[string]bool{}
	var out []*core.Item
	for _, it := range items {
		key := it.ID
		if len(it.Paths) > 0 {
			key = filepath.Clean(it.Paths[0])
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

func SortBySize(items []*core.Item) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Frees() > items[j].Frees() })
}
