package scan

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// fileKey identifies an inode for hardlink accounting.
type fileKey struct{ dev, ino uint64 }

type linkInfo struct {
	size  int64
	nlink uint64
	seen  uint64
}

// Sizer measures allocated bytes with a shared bound on concurrent directory reads.
type Sizer struct {
	sem   chan struct{}
	files atomic.Int64
}

func NewSizer(parallel int) *Sizer {
	return &Sizer{sem: make(chan struct{}, parallel)}
}

type acc struct {
	total atomic.Int64
	mu    sync.Mutex
	links map[fileKey]*linkInfo
}

// Measure returns bytes on disk under paths, and how many of those bytes are
// hard-linked from outside paths (and therefore would not be freed).
// Symlinks are never followed.
func (s *Sizer) Measure(paths []string) (total, shared int64) {
	a := &acc{links: map[fileKey]*linkInfo{}}
	var wg sync.WaitGroup
	for _, p := range paths {
		fi, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			wg.Add(1)
			s.walk(p, a, &wg)
		} else if fi.Mode().IsRegular() {
			s.add(a, fi)
		}
	}
	wg.Wait()
	total = a.total.Load()
	for _, l := range a.links {
		if l.seen < l.nlink {
			shared += l.size
		}
	}
	return total, shared
}

func (s *Sizer) walk(dir string, a *acc, wg *sync.WaitGroup) {
	defer wg.Done()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			wg.Add(1)
			select {
			case s.sem <- struct{}{}:
				go func() {
					defer func() { <-s.sem }()
					s.walk(p, a, wg)
				}()
			default:
				s.walk(p, a, wg)
			}
			continue
		}
		if !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		s.add(a, fi)
	}
}

func (s *Sizer) add(a *acc, fi os.FileInfo) {
	s.files.Add(1)
	size, key, nlink := statOf(fi)
	if nlink <= 1 {
		a.total.Add(size)
		return
	}
	a.mu.Lock()
	l := a.links[key]
	if l == nil {
		l = &linkInfo{size: size, nlink: nlink}
		a.links[key] = l
		a.total.Add(size)
	}
	l.seen++
	a.mu.Unlock()
}

// Files is how many files this sizer has looked at so far.
func (s *Sizer) Files() int64 { return s.files.Load() }

// newestShallow is the latest mtime of a path and its direct children: a cheap
// "last touched" signal that doesn't need a full walk.
func newestShallow(paths ...string) time.Time {
	var newest time.Time
	for _, p := range paths {
		fi, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
		if !fi.IsDir() {
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
	}
	return newest
}
