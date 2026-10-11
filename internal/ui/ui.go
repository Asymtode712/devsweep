// Package ui holds the palette and text helpers shared by the CLI and the TUI.
// Colors mirror the devsweep design canvas: one cyan accent, and safety tiers
// that differ by glyph and lightness, not only hue.
package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/Asymtode712/devsweep/internal/core"
)

var (
	Accent  = lipgloss.AdaptiveColor{Light: "#0b7fa8", Dark: "#56c2e6"}
	Text    = lipgloss.AdaptiveColor{Light: "#1d2128", Dark: "#d6dae0"}
	Dim     = lipgloss.AdaptiveColor{Light: "#5d6670", Dark: "#8a929c"}
	Faint   = lipgloss.AdaptiveColor{Light: "#b6bcc4", Dark: "#3a414b"}
	Border  = lipgloss.AdaptiveColor{Light: "#c9ced5", Dark: "#2f353e"}
	RowBG   = lipgloss.AdaptiveColor{Light: "#e3f1f7", Dark: "#1a2129"}
	OnColor = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#0e1013"}

	SafeC    = lipgloss.AdaptiveColor{Light: "#1f7a35", Dark: "#7fd18b"}
	CautionC = lipgloss.AdaptiveColor{Light: "#8a5a00", Dark: "#e5b454"}
	GuardedC = lipgloss.AdaptiveColor{Light: "#b3262f", Dark: "#f0717a"}

	S       = lipgloss.NewStyle().Foreground(Text)
	DimS    = lipgloss.NewStyle().Foreground(Dim)
	BoldS   = lipgloss.NewStyle().Foreground(Text).Bold(true)
	AccentS = lipgloss.NewStyle().Foreground(Accent)
	Badge   = lipgloss.NewStyle().Background(Accent).Foreground(OnColor).Bold(true).Padding(0, 1)
)

func SafetyColor(s core.Safety) lipgloss.AdaptiveColor {
	switch s {
	case core.Safe:
		return SafeC
	case core.Caution:
		return CautionC
	}
	return GuardedC
}

func SafetyStyle(s core.Safety) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(SafetyColor(s))
}

// Fit truncates or pads plain text to exactly w cells.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) > w {
		s = runewidth.Truncate(s, w, "…")
	}
	return runewidth.FillRight(s, w)
}

// FitLeft right-aligns plain text in w cells.
func FitLeft(s string, w int) string {
	if runewidth.StringWidth(s) > w {
		s = runewidth.Truncate(s, w, "…")
	}
	return runewidth.FillLeft(s, w)
}

// FitMiddle keeps both ends of a long path: ~/code/…/node_modules.
func FitMiddle(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return runewidth.FillRight(s, w)
	}
	if w < 8 {
		return Fit(s, w)
	}
	keepR := (w - 1) * 3 / 5
	keepL := w - 1 - keepR
	rs := []rune(s)
	left := runewidth.Truncate(s, keepL, "")
	right := ""
	for i := len(rs) - 1; i >= 0; i-- {
		if runewidth.StringWidth(string(rs[i:])) > keepR {
			break
		}
		right = string(rs[i:])
	}
	return runewidth.FillRight(left+"…"+right, w)
}

// Stale reports whether an item hasn't been touched for at least d. Items with
// no timestamp (Docker) count as stale: their age isn't meaningful.
func Stale(it *core.Item, d time.Duration) bool {
	return it.ModTime.IsZero() || time.Since(it.ModTime) >= d
}

// MatchEco matches an --eco value against an item: "ai", "node", "ollama", "hf"…
func MatchEco(it *core.Item, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	eco := strings.ToLower(it.Ecosystem)
	return strings.HasPrefix(eco, want) || strings.ToLower(it.Source) == want || it.RuleID == want ||
		strings.HasPrefix(it.RuleID, want+".")
}
