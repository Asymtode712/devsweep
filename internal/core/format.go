package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HumanBytes uses decimal units, matching Finder, Windows Explorer and Docker.
func HumanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	v := float64(n) / float64(div)
	if v >= 100 {
		return fmt.Sprintf("%.0f %cB", v, "kMGTPE"[exp])
	}
	return fmt.Sprintf("%.1f %cB", v, "kMGTPE"[exp])
}

// ParseBytes reads sizes like "500MB", "1.5G", "200k" (decimal units).
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	s = strings.TrimSuffix(s, "B")
	mult := 1.0
	if s != "" {
		switch s[len(s)-1] {
		case 'K':
			mult = 1e3
		case 'M':
			mult = 1e6
		case 'G':
			mult = 1e9
		case 'T':
			mult = 1e12
		}
		if mult != 1 {
			s = s[:len(s)-1]
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("can't read size %q (try 500MB or 2GB)", s)
	}
	return int64(v * mult), nil
}

// Age is a compact "how long ago": 4h, 9d, 5mo, 2y.
func Age(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return "now"
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}

// ParseAge reads "30d", "2w", "6mo", "1y", "12h".
func ParseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	units := []struct {
		suffix string
		d      time.Duration
	}{{"mo", 30 * 24 * time.Hour}, {"y", 365 * 24 * time.Hour}, {"w", 7 * 24 * time.Hour}, {"d", 24 * time.Hour}, {"h", time.Hour}}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			n, err := strconv.ParseFloat(strings.TrimSuffix(s, u.suffix), 64)
			if err != nil {
				break
			}
			return time.Duration(n * float64(u.d)), nil
		}
	}
	return 0, fmt.Errorf("can't read age %q (try 30d, 2w, 6mo, 1y)", s)
}
