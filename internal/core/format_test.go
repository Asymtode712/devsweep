package core

import (
	"testing"
	"time"
)

func TestParseBytes(t *testing.T) {
	for in, want := range map[string]int64{"500MB": 500e6, "1.5G": 1.5e9, "200k": 200e3, "42": 42, "2gb": 2e9} {
		got, err := ParseBytes(in)
		if err != nil || got != want {
			t.Errorf("ParseBytes(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := ParseBytes("lots"); err == nil {
		t.Error("expected error for nonsense")
	}
}

func TestParseAge(t *testing.T) {
	day := 24 * time.Hour
	for in, want := range map[string]time.Duration{"30d": 30 * day, "2w": 14 * day, "6mo": 180 * day, "1y": 365 * day, "12h": 12 * time.Hour} {
		got, err := ParseAge(in)
		if err != nil || got != want {
			t.Errorf("ParseAge(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[int64]string{999: "999 B", 1500: "1.5 kB", 39_600_000_000: "39.6 GB", 263_000_000_000: "263 GB"} {
		if got := HumanBytes(in); got != want {
			t.Errorf("HumanBytes(%d) = %q; want %q", in, got, want)
		}
	}
}

func TestDowngrade(t *testing.T) {
	if Safe.Downgrade() != Caution || Caution.Downgrade() != Guarded || Guarded.Downgrade() != Guarded {
		t.Error("downgrade order wrong")
	}
}
