// Package cli implements devsweep's non-interactive commands.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Asymtode712/devsweep/internal/clean"
	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
	"github.com/Asymtode712/devsweep/internal/scan"
	"github.com/Asymtode712/devsweep/internal/tui"
	"github.com/Asymtode712/devsweep/internal/ui"
)

var Version = "0.1.0"

const usage = `devsweep — find and safely reclaim disk space eaten by developer tooling

Usage:
  devsweep [paths...]          interactive TUI (default root: your home directory)
  devsweep scan [paths...]     list what can be reclaimed
  devsweep clean [paths...]    delete matching items (asks first)
  devsweep rules               show every rule and why it's considered safe
  devsweep version

Filters (scan, clean):
  --eco LIST         ecosystems or sources: ai,docker,node,python,xcode,rust,go,jvm,ollama,hf…
  --safety LEVEL     highest safety level to include: safe | caution | guarded
                     (scan default: guarded, clean default: safe)
  --older-than AGE   only items untouched for AGE: 30d, 2w, 6mo, 1y
  --min-size SIZE    only items freeing at least SIZE: 500MB, 2GB
  --no-global        skip global caches, models and Docker; only scan project folders
  --no-projects      skip project folders; only global caches, models and Docker
  --depth N          max directory depth for the project walk (default 12)

Output:
  --json             machine-readable output (scan, rules)

Clean:
  --dry-run          show what would be deleted, touch nothing
  --yes              don't prompt (guarded items still need --safety guarded)
  --trash            move to Trash instead of deleting (frees nothing until emptied)
`

type opts struct {
	cmd        string
	roots      []string
	ecos       []string
	safety     core.Safety
	safetySet  bool
	olderThan  time.Duration
	minSize    int64
	noGlobal   bool
	noProjects bool
	depth      int
	json       bool
	dryRun     bool
	yes        bool
	trash      bool
}

func Main(args []string) int {
	o, err := parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "devsweep:", err)
		fmt.Fprintln(os.Stderr, "run `devsweep --help` for usage")
		return 2
	}
	env := rules.NewEnv()
	switch o.cmd {
	case "help":
		fmt.Print(usage)
		return 0
	case "version":
		fmt.Println("devsweep", Version)
		return 0
	case "rules":
		return cmdRules(env, o)
	case "scan":
		return cmdScan(env, o)
	case "clean":
		return cmdClean(env, o)
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return cmdScan(env, o)
	}
	if err := tui.Run(env, scanOpts(o)); err != nil {
		fmt.Fprintln(os.Stderr, "devsweep:", err)
		return 1
	}
	return 0
}

func parse(args []string) (*opts, error) {
	o := &opts{safety: core.Guarded}
	if len(args) > 0 {
		switch args[0] {
		case "scan", "clean", "rules", "version", "help", "tui":
			o.cmd, args = args[0], args[1:]
		case "-h", "--help":
			o.cmd = "help"
			return o, nil
		case "-v", "--version":
			o.cmd = "version"
			return o, nil
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			o.roots = append(o.roots, a)
			continue
		}
		key, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		next := func() (string, error) {
			if hasVal {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("--%s needs a value", key)
			}
			i++
			return args[i], nil
		}
		var err error
		var v string
		switch key {
		case "h", "help":
			o.cmd = "help"
		case "json":
			o.json = true
		case "dry-run", "n":
			o.dryRun = true
		case "yes", "y":
			o.yes = true
		case "trash":
			o.trash = true
		case "no-global":
			o.noGlobal = true
		case "no-projects":
			o.noProjects = true
		case "eco", "ecosystem":
			if v, err = next(); err == nil {
				o.ecos = append(o.ecos, strings.Split(v, ",")...)
			}
		case "safety":
			if v, err = next(); err == nil {
				o.safety, err = core.ParseSafety(v)
				o.safetySet = true
			}
		case "older-than":
			if v, err = next(); err == nil {
				o.olderThan, err = core.ParseAge(v)
			}
		case "min-size":
			if v, err = next(); err == nil {
				o.minSize, err = core.ParseBytes(v)
			}
		case "depth":
			if v, err = next(); err == nil {
				o.depth, err = strconv.Atoi(v)
			}
		default:
			return nil, fmt.Errorf("unknown flag %s", a)
		}
		if err != nil {
			return nil, err
		}
	}
	if o.cmd == "clean" && !o.safetySet {
		o.safety = core.Safe
	}
	for i, r := range o.roots {
		abs, err := filepath.Abs(expandHome(r))
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", r)
		}
		o.roots[i] = abs
	}
	return o, nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}

func scanOpts(o *opts) scan.Options {
	return scan.Options{Roots: o.roots, NoGlobal: o.noGlobal, NoProjects: o.noProjects, MaxDepth: o.depth}
}

func runScan(env *rules.Env, o *opts) *scan.Result {
	so := scanOpts(o)
	if term.IsTerminal(int(os.Stderr.Fd())) && !o.json {
		so.Progress = func(p scan.Progress) {
			fmt.Fprintf(os.Stderr, "\r\033[K%s  %d dirs · %d files · %d items", p.Phase, p.Dirs, p.Files, p.Found)
		}
		defer fmt.Fprint(os.Stderr, "\r\033[K")
	}
	return scan.Run(env, so)
}

func filter(items []*core.Item, o *opts) []*core.Item {
	var out []*core.Item
	for _, it := range items {
		if it.Safety > o.safety || it.Frees() < o.minSize {
			continue
		}
		if o.olderThan > 0 && !ui.Stale(it, o.olderThan) {
			continue
		}
		if len(o.ecos) > 0 {
			ok := false
			for _, e := range o.ecos {
				ok = ok || ui.MatchEco(it, e)
			}
			if !ok {
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

func width() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 60 {
		return w
	}
	return 110
}

func printTable(w io.Writer, env *rules.Env, items []*core.Item) {
	cols := width()
	pathW := cols - 2 - 16 - 12 - 10 - 6 - 4
	fmt.Fprintln(w, ui.DimS.Render("  "+ui.Fit("ECOSYSTEM", 16)+ui.Fit("SOURCE", 12)+ui.Fit("ITEM", pathW)+ui.FitLeft("FREES", 10)+"  "+ui.FitLeft("AGE", 6)))
	for _, it := range items {
		name := it.Name
		fmt.Fprintln(w, ui.SafetyStyle(it.Safety).Render(it.Safety.Glyph())+" "+
			ui.S.Render(ui.Fit(it.Ecosystem, 16))+ui.DimS.Render(ui.Fit(it.Source, 12))+
			ui.S.Render(ui.FitMiddle(name, pathW))+ui.BoldS.Render(ui.FitLeft(core.HumanBytes(it.Frees()), 10))+"  "+
			ui.DimS.Render(ui.FitLeft(core.Age(it.ModTime), 6)))
	}
}

func totals(items []*core.Item) (by [3]int64, all int64) {
	for _, it := range items {
		by[it.Safety] += it.Frees()
		all += it.Frees()
	}
	return
}

func cmdScan(env *rules.Env, o *opts) int {
	res := runScan(env, o)
	items := filter(res.Items, o)
	if o.json {
		type jitem struct {
			*core.Item
			Safety string `json:"safety"`
			Frees  int64  `json:"frees"`
		}
		out := make([]jitem, len(items))
		for i, it := range items {
			out[i] = jitem{it, it.Safety.String(), it.Frees()}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return 0
	}
	roots := make([]string, len(res.Roots))
	for i, r := range res.Roots {
		roots[i] = scan.Tilde(env, r)
	}
	fmt.Println(ui.DimS.Render(fmt.Sprintf("scanned %s · %d items · %.1fs", strings.Join(roots, ", "), len(items), res.Elapsed.Seconds())))
	for _, w := range res.Warnings {
		fmt.Println(ui.DimS.Render(w))
	}
	if len(items) == 0 {
		fmt.Println("nothing to reclaim with these filters")
		return 0
	}
	fmt.Println()
	printTable(os.Stdout, env, items)
	by, all := totals(items)
	fmt.Printf("\n%s   %s   %s   %s\n",
		ui.SafetyStyle(core.Safe).Render("● safe "+core.HumanBytes(by[0])),
		ui.SafetyStyle(core.Caution).Render("▲ caution "+core.HumanBytes(by[1])),
		ui.SafetyStyle(core.Guarded).Render("■ guarded "+core.HumanBytes(by[2])),
		ui.BoldS.Render("total "+core.HumanBytes(all)))
	return 0
}

func cmdClean(env *rules.Env, o *opts) int {
	res := runScan(env, o)
	items := filter(res.Items, o)
	if len(items) == 0 {
		fmt.Println("nothing to clean with these filters")
		return 0
	}
	printTable(os.Stdout, env, items)
	_, all := totals(items)
	guarded := 0
	for _, it := range items {
		if it.Safety == core.Guarded {
			guarded++
		}
	}
	if o.dryRun {
		fmt.Printf("\nwould free %s across %d items  %s\n", ui.BoldS.Render(core.HumanBytes(all)), len(items), ui.DimS.Render("(dry run — nothing touched)"))
		return 0
	}
	if o.trash && env.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "devsweep:", clean.ErrTrashUnsupported)
		return 2
	}
	if !o.yes {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "devsweep: refusing to delete without a terminal; pass --yes or --dry-run")
			return 2
		}
		in := bufio.NewReader(os.Stdin)
		fmt.Printf("\nDelete %d items, freeing %s? [y/N] ", len(items), ui.BoldS.Render(core.HumanBytes(all)))
		line, _ := in.ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Println("cancelled")
			return 1
		}
		if guarded > 0 {
			fmt.Printf("%s %d guarded items may hold data nothing can recreate. Type %s to continue: ",
				ui.SafetyStyle(core.Guarded).Render("■"), guarded, ui.BoldS.Render("delete"))
			line, _ = in.ReadString('\n')
			if strings.TrimSpace(line) != "delete" {
				fmt.Println("cancelled")
				return 1
			}
		}
	}
	var freed int64
	failed := 0
	for _, it := range items {
		r := clean.Clean(env, it, clean.Options{Trash: o.trash})
		if r.Err != nil {
			failed++
			fmt.Printf("%s %s  %s\n", ui.SafetyStyle(core.Guarded).Render("✗"), it.Name, ui.DimS.Render(r.Err.Error()))
			continue
		}
		freed += r.Freed
		fmt.Printf("%s %s  %s\n", ui.SafetyStyle(core.Safe).Render("✓"), it.Name, ui.DimS.Render(r.Method+" · "+core.HumanBytes(r.Freed)))
	}
	fmt.Printf("\nfreed %s", ui.BoldS.Render(core.HumanBytes(freed)))
	if failed > 0 {
		fmt.Printf(" · %d failed", failed)
	}
	fmt.Println(ui.DimS.Render("  · log: " + scan.Tilde(env, filepath.Join(clean.StateDir(env), "history.jsonl"))))
	if failed > 0 {
		return 1
	}
	return 0
}

func cmdRules(env *rules.Env, o *opts) int {
	if o.json {
		type jr struct {
			ID, Ecosystem, Name, Kind, Safety, Why, Restore string
			Dirs, Markers                                   []string `json:",omitempty"`
			Paths                                           []string `json:",omitempty"`
		}
		var out []jr
		for _, r := range rules.All() {
			j := jr{ID: r.ID, Ecosystem: r.Ecosystem, Name: r.Name, Kind: [...]string{"project", "global", "provider"}[r.Kind],
				Safety: r.Safety.String(), Why: r.Why, Restore: r.Restore, Dirs: r.Dirs, Markers: r.Markers}
			if r.Paths != nil && r.Supported(env.GOOS) {
				for _, p := range r.Paths(env) {
					if p != "" {
						j.Paths = append(j.Paths, scan.Tilde(env, p))
					}
				}
			}
			out = append(out, j)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return 0
	}
	cols := width()
	eco := ""
	for _, r := range rules.All() {
		if !r.Supported(env.GOOS) {
			continue
		}
		if r.Ecosystem != eco {
			eco = r.Ecosystem
			fmt.Println("\n" + ui.BoldS.Render(eco))
		}
		match := ""
		switch r.Kind {
		case rules.Project:
			match = strings.Join(r.Dirs, ", ")
			if r.InnerMarker != "" {
				match += " containing " + r.InnerMarker
			}
			if len(r.Markers) > 0 {
				match += " next to " + strings.Join(r.Markers, " | ")
			}
			if match == "" {
				match = "any dir containing " + r.InnerMarker
			}
		case rules.Global:
			var ps []string
			for _, p := range r.Paths(env) {
				if p != "" {
					ps = append(ps, scan.Tilde(env, p))
				}
			}
			match = strings.Join(ps, ", ")
		default:
			match = "read from the tool's own metadata"
		}
		fmt.Println(ui.SafetyStyle(r.Safety).Render(r.Safety.Glyph()+" "+ui.Fit(r.Safety.String(), 8)) + ui.S.Render(ui.Fit(r.ID, 26)) + ui.DimS.Render(ui.Fit(match, cols-36)))
		fmt.Println(ui.DimS.Render("  " + ui.Fit("", 34) + wrap(r.Why, cols-36, 36)))
	}
	return 0
}

func wrap(s string, w, indent int) string {
	if w < 20 {
		return s
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if len(line)+len(word)+1 > w && line != "" {
			lines = append(lines, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n"+strings.Repeat(" ", indent))
}
