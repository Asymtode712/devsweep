// Package core holds the types shared by the scanner, the cleaner and the UIs.
package core

import (
	"fmt"
	"strings"
	"time"
)

// Safety is how much it costs to get a deleted item back.
type Safety int

const (
	// Safe items are pure caches or build output that tools rebuild on their own.
	Safe Safety = iota
	// Caution items are reproducible, but slow or bandwidth-heavy to restore
	// (model weights, virtualenvs, dependency stores).
	Caution
	// Guarded items may hold something you cannot recreate. Never auto-selected.
	Guarded
)

func (s Safety) String() string {
	switch s {
	case Safe:
		return "safe"
	case Caution:
		return "caution"
	default:
		return "guarded"
	}
}

func (s Safety) Glyph() string {
	switch s {
	case Safe:
		return "●"
	case Caution:
		return "▲"
	default:
		return "■"
	}
}

// Downgrade moves one step toward Guarded.
func (s Safety) Downgrade() Safety {
	if s < Guarded {
		return s + 1
	}
	return s
}

func ParseSafety(v string) (Safety, error) {
	switch strings.ToLower(v) {
	case "safe":
		return Safe, nil
	case "caution":
		return Caution, nil
	case "guarded":
		return Guarded, nil
	}
	return Safe, fmt.Errorf("unknown safety level %q (want safe, caution or guarded)", v)
}

// Item is one thing that can be cleaned: a directory, a set of files, or a
// tool-managed resource that is removed by running a native command.
type Item struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"rule"`
	Ecosystem string    `json:"ecosystem"`
	Source    string    `json:"source"`
	Name      string    `json:"name"`
	Paths     []string  `json:"paths,omitempty"`
	Size      int64     `json:"size"`   // bytes on disk
	Shared    int64     `json:"shared"` // bytes that stay because something else still references them
	ModTime   time.Time `json:"modified,omitempty"`
	Safety    Safety    `json:"-"`
	Why       string    `json:"why"`
	Note      string    `json:"note,omitempty"`
	Restore   string    `json:"restore,omitempty"`
	Command   []string  `json:"command,omitempty"` // when set, run this instead of deleting Paths
	Project   string    `json:"project,omitempty"` // project dir for per-project artifacts

	// SizeKnown is false for items whose size comes from a tool that may not report it.
	SizeKnown bool `json:"-"`
}

// Frees is what deleting the item actually returns to the filesystem.
func (it *Item) Frees() int64 {
	if it.Shared > it.Size {
		return 0
	}
	return it.Size - it.Shared
}

// Display is the primary path, or a short description for command items.
func (it *Item) Display() string {
	if len(it.Paths) == 1 {
		return it.Paths[0]
	}
	if len(it.Paths) > 1 {
		return fmt.Sprintf("%s (+%d more)", it.Paths[0], len(it.Paths)-1)
	}
	return strings.Join(it.Command, " ")
}

// Ecosystem display order: the largest offenders on a typical dev machine first.
var EcosystemOrder = []string{
	"AI models", "Docker", "Node", "Python", "Xcode", "Android", "Rust", "Go",
	"JVM", "Native builds", "Other languages", "Tool caches",
}

func EcosystemRank(name string) int {
	for i, e := range EcosystemOrder {
		if e == name {
			return i
		}
	}
	return len(EcosystemOrder)
}
