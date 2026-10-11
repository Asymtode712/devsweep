// Package tui is devsweep's interactive interface. Its layout follows the
// devsweep design canvas: ecosystems on the left, items on the right, the
// focused item's rule explained underneath.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/Asymtode712/devsweep/internal/clean"
	"github.com/Asymtode712/devsweep/internal/core"
	"github.com/Asymtode712/devsweep/internal/rules"
	"github.com/Asymtode712/devsweep/internal/scan"
	"github.com/Asymtode712/devsweep/internal/ui"
)

type state int

const (
	scanning state = iota
	browsing
	confirming
	deleting
	finished
)

type sortMode int

const (
	bySize sortMode = iota
	byAge
	byName
)

var sortNames = [...]string{"size ↓", "oldest first", "name"}

const staleAfter = 30 * 24 * time.Hour

type (
	progressMsg scan.Progress
	scanDoneMsg *scan.Result
	cleanedMsg  clean.Result
	tickMsg     struct{}
)

type eco struct {
	name  string
	total int64
	count int
}

type model struct {
	env  *rules.Env
	opts scan.Options
	send func(tea.Msg)

	state  state
	w, h   int
	prog   scan.Progress
	spin   int
	res    *scan.Result
	items  []*core.Item
	ecos   []eco
	ecoIdx int

	listFocus bool
	cursor    int
	offset    int
	selected  map[string]bool
	sort      sortMode
	filtering bool
	filter    string
	trash     bool
	flash     string

	queue   []*core.Item
	results []clean.Result
	typed   string
}

func Run(env *rules.Env, opts scan.Options) error {
	m := &model{env: env, opts: opts, selected: map[string]bool{}, listFocus: true}
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.send = p.Send
	_, err := p.Run()
	return err
}

func (m *model) Init() tea.Cmd { return tea.Batch(m.startScan(), tick()) }

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) startScan() tea.Cmd {
	m.state = scanning
	m.prog = scan.Progress{Phase: "discovering"}
	opts := m.opts
	opts.Progress = func(p scan.Progress) { m.send(progressMsg(p)) }
	return func() tea.Msg { return scanDoneMsg(scan.Run(m.env, opts)) }
}

// ---------------------------------------------------------------- data views

func (m *model) setItems(items []*core.Item) {
	m.items = items
	totals := map[string]*eco{}
	all := &eco{name: "All"}
	for _, it := range items {
		e := totals[it.Ecosystem]
		if e == nil {
			e = &eco{name: it.Ecosystem}
			totals[it.Ecosystem] = e
		}
		e.total += it.Frees()
		e.count++
		all.total += it.Frees()
		all.count++
	}
	m.ecos = []eco{*all}
	for _, e := range totals {
		m.ecos = append(m.ecos, *e)
	}
	sort.SliceStable(m.ecos[1:], func(i, j int) bool { return m.ecos[1+i].total > m.ecos[1+j].total })
	if m.ecoIdx >= len(m.ecos) {
		m.ecoIdx = 0
	}
	m.clamp()
}

func (m *model) visible() []*core.Item {
	var out []*core.Item
	ecoName := ""
	if m.ecoIdx > 0 && m.ecoIdx < len(m.ecos) {
		ecoName = m.ecos[m.ecoIdx].name
	}
	f := strings.ToLower(m.filter)
	for _, it := range m.items {
		if ecoName != "" && it.Ecosystem != ecoName {
			continue
		}
		if f != "" && !strings.Contains(strings.ToLower(it.Name+" "+it.Source+" "+it.Display()), f) {
			continue
		}
		out = append(out, it)
	}
	switch m.sort {
	case byAge:
		sort.SliceStable(out, func(i, j int) bool { return out[i].ModTime.Before(out[j].ModTime) })
	case byName:
		sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	}
	return out
}

func (m *model) selection() []*core.Item {
	var out []*core.Item
	for _, it := range m.items {
		if m.selected[it.ID] {
			out = append(out, it)
		}
	}
	return out
}

func sum(items []*core.Item) (total int64) {
	for _, it := range items {
		total += it.Frees()
	}
	return
}

func (m *model) clamp() {
	n := len(m.visible())
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	rows := m.listRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if rows > 0 && m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// ---------------------------------------------------------------- update

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.clamp()
	case tickMsg:
		m.spin++
		if m.state == scanning || m.state == deleting {
			return m, tick()
		}
	case progressMsg:
		m.prog = scan.Progress(msg)
	case scanDoneMsg:
		m.res = msg
		m.state = browsing
		m.setItems(msg.Items)
	case cleanedMsg:
		m.results = append(m.results, clean.Result(msg))
		if len(m.results) < len(m.queue) {
			return m, m.cleanNext()
		}
		m.state = finished
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.state {
	case scanning:
		if k.String() == "q" {
			return m, tea.Quit
		}
	case confirming:
		return m.keyConfirm(k)
	case finished:
		m.afterDelete()
		return m, nil
	case browsing:
		return m.keyBrowse(k)
	}
	return m, nil
}

func (m *model) keyBrowse(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.flash = ""
	if m.filtering {
		switch k.Type {
		case tea.KeyEnter:
			m.filtering = false
		case tea.KeyEsc:
			m.filtering, m.filter = false, ""
		case tea.KeyBackspace:
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
			}
		case tea.KeyRunes, tea.KeySpace:
			m.filter += string(k.Runes)
		}
		m.cursor, m.offset = 0, 0
		m.clamp()
		return m, nil
	}
	vis := m.visible()
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "tab", "shift+tab":
		m.listFocus = !m.listFocus
	case "left", "h":
		m.listFocus = false
	case "right", "l", "enter":
		m.listFocus = true
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.move(-m.listRows())
	case "pgdown":
		m.move(m.listRows())
	case "home", "g":
		m.move(-1 << 30)
	case "end", "G":
		m.move(1 << 30)
	case " ", "x":
		if m.cursor < len(vis) {
			id := vis[m.cursor].ID
			m.selected[id] = !m.selected[id]
			if !m.selected[id] {
				delete(m.selected, id)
			}
			m.move(1)
		}
	case "a":
		all := true
		for _, it := range vis {
			if it.Safety != core.Guarded && !m.selected[it.ID] {
				all = false
			}
		}
		for _, it := range vis {
			if all {
				delete(m.selected, it.ID)
			} else if it.Safety != core.Guarded {
				m.selected[it.ID] = true
			}
		}
	case "s":
		n := 0
		for _, it := range m.items {
			if it.Safety == core.Safe && ui.Stale(it, staleAfter) {
				m.selected[it.ID] = true
				n++
			}
		}
		m.flash = fmt.Sprintf("picked %d safe items untouched for 30+ days", n)
	case "c":
		m.selected = map[string]bool{}
	case "o":
		m.sort = (m.sort + 1) % 3
		m.cursor, m.offset = 0, 0
	case "/":
		m.filtering, m.listFocus = true, true
	case "t":
		m.toggleTrash()
	case "r":
		m.selected = map[string]bool{}
		return m, tea.Batch(m.startScan(), tick())
	case "d", "delete", "backspace":
		if len(m.selection()) == 0 {
			m.flash = "nothing selected — space selects, s picks safe + stale"
			return m, nil
		}
		m.state, m.typed = confirming, ""
	}
	m.clamp()
	return m, nil
}

func (m *model) toggleTrash() {
	if m.env.GOOS == "windows" {
		m.flash = "move to Trash isn't supported on Windows"
		return
	}
	m.trash = !m.trash
}

func (m *model) move(d int) {
	if !m.listFocus {
		m.ecoIdx += d
		if m.ecoIdx < 0 {
			m.ecoIdx = 0
		}
		if m.ecoIdx >= len(m.ecos) {
			m.ecoIdx = len(m.ecos) - 1
		}
		m.cursor, m.offset = 0, 0
		return
	}
	m.cursor += d
	m.clamp()
}

func (m *model) keyConfirm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	sel := m.selection()
	guarded := countSafety(sel, core.Guarded)
	switch k.Type {
	case tea.KeyEsc:
		m.state = browsing
		return m, nil
	case tea.KeyEnter:
		if guarded > 0 && m.typed != "delete" {
			return m, nil
		}
		m.queue, m.results = sel, nil
		m.state = deleting
		return m, tea.Batch(m.cleanNext(), tick())
	case tea.KeyBackspace:
		if len(m.typed) > 0 {
			m.typed = m.typed[:len(m.typed)-1]
		}
	case tea.KeyRunes:
		if guarded > 0 {
			m.typed += string(k.Runes)
		} else if k.String() == "t" {
			m.toggleTrash()
		}
	case tea.KeyTab:
		m.toggleTrash()
	}
	return m, nil
}

func (m *model) cleanNext() tea.Cmd {
	it := m.queue[len(m.results)]
	trash := m.trash
	return func() tea.Msg { return cleanedMsg(clean.Clean(m.env, it, clean.Options{Trash: trash})) }
}

func (m *model) afterDelete() {
	gone := map[string]bool{}
	for _, r := range m.results {
		if r.Err == nil {
			gone[r.Item.ID] = true
		}
		delete(m.selected, r.Item.ID)
	}
	var kept []*core.Item
	for _, it := range m.items {
		if !gone[it.ID] {
			kept = append(kept, it)
		}
	}
	m.state = browsing
	m.setItems(kept)
}

func countSafety(items []*core.Item, s core.Safety) (n int) {
	for _, it := range items {
		if it.Safety == s {
			n++
		}
	}
	return
}

// ---------------------------------------------------------------- view

const (
	sideW   = 32
	detailH = 8
)

func (m *model) bodyH() int { return m.h - 2 }

func (m *model) listRows() int {
	// list box: border(2) + title + header
	return m.bodyH() - (detailH + 2) - 4
}

func (m *model) View() string {
	if m.w == 0 {
		return ""
	}
	if m.w < 80 || m.h < 20 {
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center,
			ui.DimS.Render(fmt.Sprintf("devsweep needs at least 80×20 (now %d×%d)", m.w, m.h)))
	}
	switch m.state {
	case scanning:
		return m.viewScanning()
	case confirming:
		return m.viewConfirm()
	case deleting, finished:
		return m.viewDeleting()
	}
	return m.viewBrowse()
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *model) viewScanning() string {
	src := m.opts.Roots
	if len(src) == 0 {
		src = []string{m.env.Home}
	}
	roots := make([]string, len(src))
	for i, r := range src {
		roots[i] = scan.Tilde(m.env, r)
	}
	lines := []string{
		ui.Badge.Render("devsweep"),
		"",
		ui.AccentS.Render(spinner[m.spin%len(spinner)]) + ui.S.Render(" "+m.prog.Phase+" "+strings.Join(roots, ", ")),
		"",
		ui.DimS.Render(fmt.Sprintf("%-10s", "dirs")) + ui.S.Render(fmt.Sprint(m.prog.Dirs)),
		ui.DimS.Render(fmt.Sprintf("%-10s", "files")) + ui.S.Render(fmt.Sprint(m.prog.Files)),
		ui.DimS.Render(fmt.Sprintf("%-10s", "found")) + ui.S.Render(fmt.Sprint(m.prog.Found)),
		"",
		ui.DimS.Render("q to quit"),
	}
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n"))
}

func (m *model) header() string {
	sel := m.selection()
	right := ui.DimS.Render("reclaimable ") + ui.BoldS.Render(core.HumanBytes(sum(m.items))) + "   " +
		ui.DimS.Render("selected ") + ui.AccentS.Bold(true).Render(core.HumanBytes(sum(sel))) +
		ui.DimS.Render(fmt.Sprintf(" · %d", len(sel)))
	if m.trash {
		right = ui.SafetyStyle(core.Caution).Render("trash mode  ") + right
	}
	badge := ui.Badge.Render("devsweep") + " "
	info := fmt.Sprintf("scanned %s in %.1fs · %d items", m.rootsLabel(), m.res.Elapsed.Seconds(), len(m.items))
	room := m.w - 2 - lipgloss.Width(badge) - lipgloss.Width(right) - 2
	left := badge + ui.DimS.Render(strings.TrimRight(ui.FitMiddle(info, max(room, 0)), " "))
	gap := m.w - lipgloss.Width(left) - lipgloss.Width(right) - 2
	return " " + left + strings.Repeat(" ", max(gap, 1)) + right
}

func (m *model) rootsLabel() string {
	var r []string
	for _, p := range m.res.Roots {
		r = append(r, scan.Tilde(m.env, p))
	}
	return strings.Join(r, ", ")
}

func box(content []string, w, h int, focused bool) string {
	c := ui.Border
	if focused {
		c = ui.Accent
	}
	for len(content) < h {
		content = append(content, "")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).
		Padding(0, 1).Width(w - 2).Render(strings.Join(content[:h], "\n"))
}

func (m *model) viewBrowse() string {
	bodyH := m.bodyH()
	side := m.viewSidebar(bodyH - 2)
	mainW := m.w - sideW - 1
	list := m.viewList(mainW, bodyH-(detailH+2)-2)
	detail := m.viewDetail(mainW)
	right := lipgloss.JoinVertical(lipgloss.Left, list, detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, side, " ", right)
	return lipgloss.JoinVertical(lipgloss.Left, m.header(), body, m.footer())
}

func (m *model) viewSidebar(h int) string {
	inner := sideW - 4
	var lines []string
	lines = append(lines, ui.DimS.Render("ECOSYSTEMS"))
	legend := []string{
		ui.DimS.Render(strings.Repeat("─", inner)),
		ui.SafetyStyle(core.Safe).Render("●") + ui.DimS.Render(" safe — rebuilt for you"),
		ui.SafetyStyle(core.Caution).Render("▲") + ui.DimS.Render(" caution — slow to restore"),
		ui.SafetyStyle(core.Guarded).Render("■") + ui.DimS.Render(" guarded — opt-in only"),
	}
	avail := h - 1 - len(legend)
	twoLine := len(m.ecos)*2 <= avail
	max := int64(1)
	for _, e := range m.ecos[1:] {
		if e.total > max {
			max = e.total
		}
	}
	for i, e := range m.ecos {
		cur := i == m.ecoIdx
		bg := lipgloss.NewStyle()
		if cur {
			bg = bg.Background(ui.RowBG)
		}
		mark := "  "
		nameS := ui.S
		if cur {
			mark = "▸ "
			nameS = ui.BoldS
		}
		size := core.HumanBytes(e.total)
		name := ui.Fit(mark+e.name, inner-len(size))
		lines = append(lines, nameS.Inherit(bg).Render(name)+ui.S.Inherit(bg).Render(size))
		if twoLine {
			if i == 0 {
				lines = append(lines, "")
				continue
			}
			filled := int(float64(inner) * float64(e.total) / float64(max))
			if filled < 1 && e.total > 0 {
				filled = 1
			}
			barC := ui.Faint
			if cur {
				barC = ui.Accent
			}
			lines = append(lines, lipgloss.NewStyle().Foreground(barC).Render(strings.Repeat("━", filled))+
				lipgloss.NewStyle().Foreground(ui.Border).Render(strings.Repeat("─", inner-filled)))
		}
	}
	if len(lines) > avail+1 {
		lines = lines[:avail+1]
	}
	for len(lines) < h-len(legend) {
		lines = append(lines, "")
	}
	lines = append(lines, legend...)
	return box(lines, sideW, h, !m.listFocus)
}

func (m *model) viewList(w, h int) string {
	inner := w - 4
	vis := m.visible()
	title := "All ecosystems"
	if m.ecoIdx > 0 {
		title = m.ecos[m.ecoIdx].name
	}
	meta := "sort: " + sortNames[m.sort]
	if m.filtering {
		meta = "filter: " + m.filter + "█"
	} else if m.filter != "" {
		meta += " · filter: " + m.filter
	}
	gap := inner - runewidth.StringWidth(title) - runewidth.StringWidth(meta)
	if gap < 1 {
		gap = 1
	}
	lines := []string{ui.BoldS.Render(title) + strings.Repeat(" ", gap) + ui.DimS.Render(meta)}

	const checkW, glyphW, srcW, sizeW, ageW = 4, 2, 11, 10, 6
	nameW := inner - checkW - glyphW - srcW - sizeW - ageW - 1
	lines = append(lines, ui.DimS.Render(strings.Repeat(" ", checkW+glyphW)+ui.Fit("SOURCE", srcW)+ui.Fit("NAME", nameW)+
		ui.FitLeft("FREES", sizeW)+" "+ui.FitLeft("AGE", ageW)))

	rows := h - 2
	if len(vis) == 0 {
		lines = append(lines, "", ui.DimS.Render("  nothing here"))
	}
	for i := m.offset; i < len(vis) && i < m.offset+rows; i++ {
		it := vis[i]
		cur := i == m.cursor && m.listFocus
		st := func(s lipgloss.Style) lipgloss.Style {
			if cur {
				return s.Background(ui.RowBG)
			}
			return s
		}
		check, checkS := "[ ]", lipgloss.NewStyle().Foreground(ui.Faint)
		if m.selected[it.ID] {
			check, checkS = "[x]", ui.AccentS
		}
		name := it.Name
		nameS := ui.S
		if cur {
			nameS = ui.BoldS
		}
		age := core.Age(it.ModTime)
		ageS := ui.DimS
		if !ui.Stale(it, 7*24*time.Hour) {
			ageS = ui.AccentS // recently used: think twice
		}
		lines = append(lines,
			st(checkS).Render(ui.Fit(check, checkW))+
				st(ui.SafetyStyle(it.Safety)).Render(ui.Fit(it.Safety.Glyph(), glyphW))+
				st(ui.DimS).Render(ui.Fit(it.Source, srcW))+
				st(nameS).Render(ui.FitMiddle(name, nameW))+
				st(ui.BoldS).Render(ui.FitLeft(core.HumanBytes(it.Frees()), sizeW))+
				st(ui.S).Render(" ")+
				st(ageS).Render(ui.FitLeft(age, ageW)))
	}
	if len(vis) > rows && rows > 0 {
		lines[0] = ui.BoldS.Render(title) + ui.DimS.Render(fmt.Sprintf("  %d–%d of %d", m.offset+1, min(m.offset+rows, len(vis)), len(vis)))
		pad := inner - lipgloss.Width(lines[0]) - runewidth.StringWidth(meta)
		lines[0] += strings.Repeat(" ", max(1, pad)) + ui.DimS.Render(meta)
	}
	return box(lines, w, h, m.listFocus)
}

func (m *model) viewDetail(w int) string {
	inner := w - 4
	vis := m.visible()
	if m.cursor >= len(vis) {
		return box([]string{ui.DimS.Render("no item focused")}, w, detailH, false)
	}
	it := vis[m.cursor]
	label := 9
	kv := func(k, v string, vs lipgloss.Style) string {
		return ui.DimS.Render(ui.Fit(k, label)) + vs.Render(ui.Fit(v, inner-label))
	}
	right := ui.SafetyStyle(it.Safety).Render(it.Safety.Glyph()+" "+it.Safety.String()) + ui.DimS.Render(" · rule "+it.RuleID)
	nameW := inner - lipgloss.Width(right) - 1
	lines := []string{ui.BoldS.Render(ui.FitMiddle(it.Name, nameW)) + " " + right}

	path := "—"
	if len(it.Paths) > 0 {
		path = scan.Tilde(m.env, it.Paths[0])
		if len(it.Paths) > 1 {
			path += fmt.Sprintf("  (+%d files)", len(it.Paths)-1)
		}
	}
	lines = append(lines, kv("path", path, ui.S))
	why := wrapWords(it.Why, inner-label)
	lines = append(lines, kv("why", why[0], ui.S))
	if len(why) > 1 {
		lines = append(lines, kv("", why[1], ui.S))
	}
	frees := core.HumanBytes(it.Frees())
	if it.Shared > 0 {
		frees += fmt.Sprintf(" of %s — %s is shared and stays", core.HumanBytes(it.Size), core.HumanBytes(it.Shared))
	}
	lines = append(lines, kv("frees", frees, ui.S))
	action := "deletes permanently"
	if m.trash {
		action = "moves to Trash"
	}
	if len(it.Command) > 0 {
		action = "runs " + strings.Join(it.Command, " ")
		if len(it.Paths) > 0 {
			action += " (falls back to removing the files above)"
		}
	}
	lines = append(lines, kv("action", action, ui.AccentS))
	restore := it.Restore
	if restore == "" {
		restore = "—"
	}
	lines = append(lines, kv("restore", restore, ui.S))
	if it.Note != "" {
		lines = append(lines, kv("note", it.Note, ui.SafetyStyle(core.Caution)))
	}
	return box(lines, w, detailH, false)
}

func (m *model) footer() string {
	if m.flash != "" {
		return " " + ui.AccentS.Render(m.flash)
	}
	keys := [][2]string{{"↑↓", "move"}, {"space", "select"}, {"tab", "pane"}, {"a", "all in view"}, {"s", "smart-pick safe + stale"},
		{"/", "filter"}, {"o", "sort"}, {"d", "delete"}, {"q", "quit"}, {"t", "trash mode"}, {"r", "rescan"}}
	var parts []string
	width := 1
	for _, k := range keys {
		p := ui.S.Render(k[0]) + " " + ui.DimS.Render(k[1])
		pw := lipgloss.Width(p) + 3
		if width+pw > m.w {
			break
		}
		width += pw
		parts = append(parts, p)
	}
	return " " + strings.Join(parts, "   ")
}

func (m *model) viewConfirm() string {
	sel := m.selection()
	w := min(m.w-4, 100)
	inner := w - 6
	caution, guarded := countSafety(sel, core.Caution), countSafety(sel, core.Guarded)

	title := ui.BoldS.Render(fmt.Sprintf("Delete %d items · ", len(sel))) + ui.AccentS.Bold(true).Render(core.HumanBytes(sum(sel)))
	counts := ui.DimS.Render(fmt.Sprintf("%d caution · %d guarded", caution, guarded))
	lines := []string{title + strings.Repeat(" ", max(1, inner-lipgloss.Width(title)-lipgloss.Width(counts))) + counts, ""}

	var cmds, dirs []*core.Item
	for _, it := range sel {
		if len(it.Command) > 0 {
			cmds = append(cmds, it)
		} else {
			dirs = append(dirs, it)
		}
	}
	budget := m.h - 22
	row := func(it *core.Item, text string) string {
		size := core.HumanBytes(it.Frees())
		return "  " + ui.SafetyStyle(it.Safety).Render(it.Safety.Glyph()) + " " +
			ui.S.Render(ui.FitMiddle(text, inner-4-10)) + ui.BoldS.Render(ui.FitLeft(size, 10))
	}
	section := func(head string, items []*core.Item, text func(*core.Item) string) {
		if len(items) == 0 {
			return
		}
		lines = append(lines, ui.DimS.Render(head))
		for i, it := range items {
			if budget <= 1 && i < len(items)-1 {
				lines = append(lines, ui.DimS.Render(fmt.Sprintf("    + %d more", len(items)-i)))
				break
			}
			lines = append(lines, row(it, text(it)))
			budget--
		}
		lines = append(lines, "")
	}
	section("NATIVE COMMANDS — run instead of rm, so the tool's own state stays consistent", cmds,
		func(it *core.Item) string { return strings.Join(it.Command, " ") })
	verb := "removed permanently"
	if m.trash {
		verb = "moved to Trash (frees nothing until emptied)"
	}
	section("DIRECTORIES — "+verb, dirs, func(it *core.Item) string { return scan.Tilde(m.env, it.Paths[0]) })

	var restores []string
	seen := map[string]bool{}
	for _, it := range sel {
		if it.Safety != core.Safe && it.Restore != "" && !seen[it.Restore] {
			seen[it.Restore] = true
			restores = append(restores, it.Restore)
		}
	}
	if len(restores) > 0 {
		lines = append(lines, ui.DimS.Render("TO RESTORE LATER"))
		for i, r := range restores {
			if i == 3 {
				lines = append(lines, ui.DimS.Render(fmt.Sprintf("  + %d more in the history log", len(restores)-3)))
				break
			}
			lines = append(lines, "  "+ui.S.Render(ui.Fit(r, inner-2)))
		}
		lines = append(lines, "")
	}

	mode := "permanent delete"
	if m.trash {
		mode = "move to Trash"
	}
	lines = append(lines, ui.DimS.Render("mode ")+ui.BoldS.Render(mode)+ui.DimS.Render("   tab  toggle Trash"))
	lines = append(lines, ui.DimS.Render(ui.Fit("Each path is re-checked against its rule first. Log: "+
		scan.Tilde(m.env, clean.StateDir(m.env))+"/history.jsonl", inner)))
	lines = append(lines, "")
	if guarded > 0 {
		lines = append(lines, ui.SafetyStyle(core.Guarded).Render(fmt.Sprintf("■ %d guarded items may hold data nothing can recreate.", guarded)))
		lines = append(lines, ui.S.Render("Type ")+ui.BoldS.Render("delete")+ui.S.Render(" to enable: ")+ui.AccentS.Render(m.typed+"█"))
		lines = append(lines, "")
	}
	btn := ui.Badge
	if guarded > 0 && m.typed != "delete" {
		btn = lipgloss.NewStyle().Foreground(ui.Dim).Background(ui.RowBG).Padding(0, 1)
	}
	lines = append(lines, btn.Render("enter  delete "+core.HumanBytes(sum(sel)))+"   "+
		lipgloss.NewStyle().Foreground(ui.Text).Border(lipgloss.HiddenBorder(), false).Padding(0, 1).Render("esc  back"))

	modal := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Accent).Padding(1, 2).Width(w - 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, modal)
}

func (m *model) viewDeleting() string {
	w := min(m.w-4, 90)
	inner := w - 6
	done := len(m.results)
	total := len(m.queue)
	var freed int64
	failed := 0
	for _, r := range m.results {
		freed += r.Freed
		if r.Err != nil {
			failed++
		}
	}
	var lines []string
	if m.state == deleting {
		lines = append(lines, ui.AccentS.Render(spinner[m.spin%len(spinner)])+ui.BoldS.Render(fmt.Sprintf(" Deleting %d of %d", done+1, total)))
	} else {
		head := ui.BoldS.Render("Freed ") + ui.AccentS.Bold(true).Render(core.HumanBytes(freed))
		if failed > 0 {
			head += ui.SafetyStyle(core.Guarded).Render(fmt.Sprintf(" · %d failed", failed))
		}
		lines = append(lines, head)
	}
	barW := inner
	filled := barW * done / max(total, 1)
	lines = append(lines, ui.AccentS.Render(strings.Repeat("━", filled))+lipgloss.NewStyle().Foreground(ui.Border).Render(strings.Repeat("─", barW-filled)), "")

	start := max(0, len(m.results)-(m.h-14))
	for _, r := range m.results[start:] {
		if r.Err != nil {
			lines = append(lines, ui.SafetyStyle(core.Guarded).Render("✗ ")+ui.S.Render(ui.FitMiddle(r.Item.Name, inner/2-2))+
				ui.DimS.Render(ui.Fit(" "+r.Err.Error(), inner-inner/2)))
			continue
		}
		lines = append(lines, ui.SafetyStyle(core.Safe).Render("✓ ")+ui.S.Render(ui.FitMiddle(r.Item.Name, inner-2-22))+
			ui.DimS.Render(ui.FitLeft(r.Method, 10))+ui.BoldS.Render(ui.FitLeft(core.HumanBytes(r.Freed), 12)))
	}
	if m.state == deleting && done < total {
		lines = append(lines, ui.DimS.Render("  "+ui.FitMiddle(m.queue[done].Name, inner-2)))
	}
	if m.state == finished {
		lines = append(lines, "", ui.DimS.Render("any key to continue"))
	}
	modal := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ui.Accent).Padding(1, 2).Width(w - 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, modal)
}

// wrapWords splits text into lines of at most w cells; the last kept line is truncated by Fit.
func wrapWords(s string, w int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line != "" && runewidth.StringWidth(line)+1+runewidth.StringWidth(word) > w {
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
	if len(lines) > 2 {
		lines[1] = strings.Join(lines[1:], " ")
		lines = lines[:2]
	}
	return lines
}
