package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/pprof/profile"
	"go.k6.io/k6/lib"
	_ "go.k6.io/k6/lib/executor" // register executors so archive options can unmarshal
	"go.k6.io/k6/lib/fsext"
)

type fileStats struct {
	Samples      int64
	CPUNanos     int64
	AllocObjects int64
	AllocSpace   int64
	AsyncWaitNS  int64
	AsyncRunNS   int64
}

type lineStats map[int]fileStats

type appState struct {
	prof        *profile.Profile
	sampleIdx   map[string]int
	files       map[string]string
	mainFile    string
	perFile     map[string]fileStats
	perLine     map[string]lineStats
	sortMetric  string
	color       bool
	scopeTotals map[string]fileStats
}

func normalizeProfilePath(raw string) string {
	if raw == "" {
		return ""
	}
	// sobek emits file:// urls for local sources.
	if u, err := url.Parse(raw); err == nil {
		switch {
		case u.Scheme == "file":
			p, err := url.PathUnescape(u.Path)
			if err == nil {
				return path.Clean(p)
			}
			return path.Clean(u.Path)
		case u.Scheme == "":
			// not a URL, continue below
		default:
			return raw
		}
	}
	return path.Clean(raw)
}

func normalizeArchivePath(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := path.Clean(raw)
	if !strings.HasPrefix(cleaned, "/") {
		return "/" + cleaned
	}
	return cleaned
}

func profileCandidateKeys(raw string) []string {
	keys := make([]string, 0, 4)
	keys = append(keys, raw)
	if u, err := url.Parse(raw); err == nil {
		switch u.Scheme {
		case "file":
			keys = append(keys, normalizeArchivePath(u.Path))
		case "http", "https":
			keys = append(keys, normalizeArchivePath("/"+u.Host+u.Path))
			keys = append(keys, normalizeArchivePath(u.Path))
		}
	}
	keys = append(keys, normalizeArchivePath(raw))
	// dedupe
	out := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func parseArchive(tarPath string) (map[string]string, string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	arc, err := lib.ReadArchive(f)
	if err != nil {
		return nil, "", err
	}
	out := make(map[string]string)
	mainFile := ""
	if arc.FilenameURL != nil && arc.FilenameURL.Scheme == "file" {
		mainFile = normalizeArchivePath(arc.FilenameURL.Path)
	}
	for _, fsName := range []string{"file", "https"} {
		fs := arc.Filesystems[fsName]
		if fs == nil {
			continue
		}
		err := fsext.Walk(fs, fsext.FilePathSeparator, func(filePath string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			b, err := fsext.ReadFile(fs, filePath)
			if err != nil {
				return err
			}
			p := normalizeArchivePath(filepath.ToSlash(filePath))
			out[p] = string(b)
			return nil
		})
		if err != nil {
			return nil, "", err
		}
	}
	if mainFile != "" && len(arc.Data) > 0 {
		out[mainFile] = string(arc.Data)
	}
	return out, mainFile, nil
}

func sampleTypeIndex(p *profile.Profile) map[string]int {
	m := make(map[string]int, len(p.SampleType))
	for i, st := range p.SampleType {
		m[st.Type] = i
	}
	return m
}

func sampleValue(sample *profile.Sample, idx int) int64 {
	if idx < 0 || idx >= len(sample.Value) {
		return 0
	}
	return sample.Value[idx]
}

func sampleLabelInt(sample *profile.Sample, key string) int64 {
	if sample == nil || sample.Label == nil {
		return 0
	}
	vals := sample.Label[key]
	if len(vals) == 0 {
		return 0
	}
	v, err := strconv.ParseInt(vals[0], 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func aggregate(p *profile.Profile) (map[string]fileStats, map[string]lineStats) {
	idx := sampleTypeIndex(p)
	perFile := make(map[string]fileStats)
	perLine := make(map[string]lineStats)

	for _, s := range p.Sample {
		if len(s.Location) == 0 {
			continue
		}
		loc := s.Location[0] // leaf frame attribution
		if len(loc.Line) == 0 || loc.Line[0].Function == nil {
			continue
		}
		fn := loc.Line[0].Function
		file := normalizeProfilePath(fn.Filename)
		if file == "" || file == "<native>" {
			continue
		}
		line := int(loc.Line[0].Line)

		fs := perFile[file]
		fs.Samples += sampleValue(s, idx["samples"])
		fs.CPUNanos += sampleValue(s, idx["cpu"])
		fs.AllocObjects += sampleValue(s, idx["alloc_objects"])
		fs.AllocSpace += sampleValue(s, idx["alloc_space"])
		fs.AsyncWaitNS += sampleLabelInt(s, "js.async.wait_ns")
		fs.AsyncRunNS += sampleLabelInt(s, "js.async.run_ns")
		perFile[file] = fs

		ls := perLine[file]
		if ls == nil {
			ls = make(lineStats)
			perLine[file] = ls
		}
		v := ls[line]
		v.Samples += sampleValue(s, idx["samples"])
		v.CPUNanos += sampleValue(s, idx["cpu"])
		v.AllocObjects += sampleValue(s, idx["alloc_objects"])
		v.AllocSpace += sampleValue(s, idx["alloc_space"])
		v.AsyncWaitNS += sampleLabelInt(s, "js.async.wait_ns")
		v.AsyncRunNS += sampleLabelInt(s, "js.async.run_ns")
		ls[line] = v
	}
	return perFile, perLine
}

func sumStats(perFile map[string]fileStats) fileStats {
	var out fileStats
	for _, v := range perFile {
		out.Samples += v.Samples
		out.CPUNanos += v.CPUNanos
		out.AllocObjects += v.AllocObjects
		out.AllocSpace += v.AllocSpace
		out.AsyncWaitNS += v.AsyncWaitNS
		out.AsyncRunNS += v.AsyncRunNS
	}
	return out
}

func fmtBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.2f MB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.2f KB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func fmtDurationNS(ns int64) string {
	if ns < 1_000 {
		return fmt.Sprintf("%d ns", ns)
	}
	if ns < 1_000_000 {
		return fmt.Sprintf("%.2f us", float64(ns)/1_000.0)
	}
	if ns < 1_000_000_000 {
		return fmt.Sprintf("%.2f ms", float64(ns)/1_000_000.0)
	}
	return fmt.Sprintf("%.2f s", float64(ns)/1_000_000_000.0)
}

func supportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}

func colorize(enabled bool, code, s string) string {
	if !enabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func colorCPU(enabled bool, v int64, s string) string {
	switch {
	case v > 500_000_000:
		return colorize(enabled, "1;31", s) // bright red
	case v > 50_000_000:
		return colorize(enabled, "31", s) // red
	case v > 5_000_000:
		return colorize(enabled, "33", s) // yellow
	default:
		return s
	}
}

func colorMem(enabled bool, v int64, s string) string {
	switch {
	case v > 100*1024*1024:
		return colorize(enabled, "1;35", s) // bright magenta
	case v > 10*1024*1024:
		return colorize(enabled, "35", s) // magenta
	case v > 1*1024*1024:
		return colorize(enabled, "36", s) // cyan
	default:
		return s
	}
}

func colorObj(enabled bool, v int64, s string) string {
	switch {
	case v > 1_000_000:
		return colorize(enabled, "1;34", s) // bright blue
	case v > 100_000:
		return colorize(enabled, "34", s) // blue
	default:
		return s
	}
}

func highlightLineJS(line string, enabled bool) string {
	if !enabled {
		return line
	}
	keywords := []string{
		"export", "import", "const", "let", "var", "function", "return",
		"if", "else", "for", "while", "new", "class", "try", "catch",
	}
	out := line
	for _, kw := range keywords {
		out = strings.ReplaceAll(out, kw, "\x1b[36m"+kw+"\x1b[0m")
	}
	return out
}

func sortedFiles(perFile map[string]fileStats, metric string) []string {
	files := make([]string, 0, len(perFile))
	for f := range perFile {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool {
		a := perFile[files[i]]
		b := perFile[files[j]]
		switch metric {
		case "cpu":
			return a.CPUNanos > b.CPUNanos
		case "alloc_objects":
			return a.AllocObjects > b.AllocObjects
		default:
			return a.AllocSpace > b.AllocSpace
		}
	})
	return files
}

type viewerModel struct {
	st appState

	width  int
	height int

	focusLeft bool

	fileOrder []string
	selected  int

	currentFile  string
	sourceLines  []string
	sourceOffset int
}

func newViewerModel(st appState) viewerModel {
	m := viewerModel{
		st:        st,
		focusLeft: true,
	}
	m.refreshFileOrder()
	if len(m.fileOrder) > 0 {
		m.openSelectedFile()
	}
	return m
}

func (m *viewerModel) refreshFileOrder() {
	m.fileOrder = sortedFiles(m.st.perFile, m.st.sortMetric)
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.fileOrder) && len(m.fileOrder) > 0 {
		m.selected = len(m.fileOrder) - 1
	}
}

func (m *viewerModel) openSelectedFile() {
	if len(m.fileOrder) == 0 {
		m.currentFile = ""
		m.sourceLines = nil
		return
	}
	f := m.fileOrder[m.selected]
	var src string
	var ok bool
	for _, key := range profileCandidateKeys(f) {
		src, ok = m.st.files[key]
		if ok {
			break
		}
	}
	if !ok {
		src = "source not found in archive"
	}
	m.currentFile = f
	m.sourceLines = strings.Split(src, "\n")
	m.sourceOffset = 0
}

func (m viewerModel) Init() tea.Cmd { return nil }

func (m viewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.focusLeft = !m.focusLeft
		case "c":
			m.st.sortMetric = "cpu"
			m.refreshFileOrder()
		case "m":
			m.st.sortMetric = "alloc_space"
			m.refreshFileOrder()
		case "o":
			m.st.sortMetric = "alloc_objects"
			m.refreshFileOrder()
		case "enter":
			if m.focusLeft {
				m.openSelectedFile()
			}
		case "up", "k":
			if m.focusLeft {
				if m.selected > 0 {
					m.selected--
				}
			} else if m.sourceOffset > 0 {
				m.sourceOffset--
			}
		case "down", "j":
			if m.focusLeft {
				if m.selected < len(m.fileOrder)-1 {
					m.selected++
				}
			} else if m.sourceOffset < max(0, len(m.sourceLines)-1) {
				m.sourceOffset++
			}
		case "pgdown":
			if !m.focusLeft {
				m.sourceOffset += max(1, m.height-6)
				if m.sourceOffset >= len(m.sourceLines) {
					m.sourceOffset = max(0, len(m.sourceLines)-1)
				}
			}
		case "pgup":
			if !m.focusLeft {
				m.sourceOffset -= max(1, m.height-6)
				if m.sourceOffset < 0 {
					m.sourceOffset = 0
				}
			}
		}
	}
	return m, nil
}

func (m viewerModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}
	leftW := m.width / 2
	rightW := m.width - leftW - 1
	bodyH := max(3, m.height-4)

	focus := map[bool]string{true: "files ", false: "source"}[m.focusLeft]
	selectedInfo := "selected: n/a"
	if len(m.fileOrder) > 0 && m.selected < len(m.fileOrder) {
		sf := m.fileOrder[m.selected]
		sv := m.st.perFile[sf]
		selectedInfo = fmt.Sprintf(
			"selected: cpu=%s mem=%s obj=%s wait=%s run=%s smp=%d",
			fmtDurationNS(sv.CPUNanos),
			fmtBytes(sv.AllocSpace),
			strconv.FormatInt(sv.AllocObjects, 10),
			fmtDurationNS(sv.AsyncWaitNS),
			fmtDurationNS(sv.AsyncRunNS),
			sv.Samples,
		)
	}
	header := fmt.Sprintf(
		"pprof_viewer | sort=%-12s | focus=%s | %s",
		m.st.sortMetric,
		focus,
		selectedInfo,
	)
	if len(m.st.scopeTotals) > 0 {
		initS := m.st.scopeTotals["init"]
		vuS := m.st.scopeTotals["vu"]
		combinedS := m.st.scopeTotals["combined"]
		header = header + fmt.Sprintf(
			" | scope cpu(init/vu/combined)=%s/%s/%s",
			fmtDurationNS(initS.CPUNanos),
			fmtDurationNS(vuS.CPUNanos),
			fmtDurationNS(combinedS.CPUNanos),
		)
	}

	left := m.renderLeft(leftW, bodyH)
	right := m.renderRight(rightW, bodyH)
	linesL := strings.Split(left, "\n")
	linesR := strings.Split(right, "\n")
	maxLines := bodyH

	var b strings.Builder
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("=", m.width) + "\n")
	for i := 0; i < maxLines; i++ {
		l := ""
		r := ""
		if i < len(linesL) {
			l = linesL[i]
		}
		if i < len(linesR) {
			r = linesR[i]
		}
		b.WriteString(padRight(l, leftW) + "│" + padRight(r, rightW) + "\n")
	}
	b.WriteString(padRight("keys: tab q enter c/m/o ↑/↓ pgup/pgdn", m.width))
	return b.String()
}

func (m viewerModel) renderLeft(width, height int) string {
	var b strings.Builder
	title := "Files"
	if m.focusLeft {
		title = "> Files"
	}
	b.WriteString(title + "\n")
	b.WriteString(fmt.Sprintf("%-4s %-9s %-9s %-9s %-9s %-9s %-7s %s\n", "#", "cpu", "mem", "objs", "wait", "run", "smp", "file"))
	start := 0
	if m.selected >= height-2 {
		start = m.selected - (height - 3)
	}
	for i := 0; i < height-2; i++ {
		idx := start + i
		if idx >= len(m.fileOrder) {
			b.WriteString("\n")
			continue
		}
		f := m.fileOrder[idx]
		v := m.st.perFile[f]
		prefix := " "
		if idx == m.selected {
			prefix = ">"
		}
		cpuText := fmt.Sprintf("%-9s", fmtDurationNS(v.CPUNanos))
		memText := fmt.Sprintf("%-9s", fmtBytes(v.AllocSpace))
		objText := fmt.Sprintf("%-9d", v.AllocObjects)
		waitText := fmt.Sprintf("%-9s", fmtDurationNS(v.AsyncWaitNS))
		runText := fmt.Sprintf("%-9s", fmtDurationNS(v.AsyncRunNS))
		smpText := fmt.Sprintf("%-7d", v.Samples)
		line := fmt.Sprintf(
			"%s%-3d %s %s %s %s %s %s %s",
			prefix, idx+1,
			colorCPU(m.st.color, v.CPUNanos, cpuText),
			colorMem(m.st.color, v.AllocSpace, memText),
			colorObj(m.st.color, v.AllocObjects, objText),
			waitText,
			runText,
			smpText,
			f,
		)
		b.WriteString(truncate(line, width) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (m viewerModel) renderRight(width, height int) string {
	var b strings.Builder
	title := "Source"
	if !m.focusLeft {
		title = "> Source"
	}
	b.WriteString(title + "\n")
	if m.currentFile == "" {
		b.WriteString("(no file)\n")
		return strings.TrimRight(b.String(), "\n")
	}
	b.WriteString(truncate(m.currentFile, width) + "\n")
	hot := m.st.perLine[m.currentFile]
	if hot == nil {
		hot = m.st.perLine[normalizeArchivePath(m.currentFile)]
	}
	for i := 0; i < height-2; i++ {
		idx := m.sourceOffset + i
		if idx >= len(m.sourceLines) {
			b.WriteString("\n")
			continue
		}
		lineNo := idx + 1
		line := m.sourceLines[idx]
		marker := " "
		stat := hot[lineNo]
		if stat.CPUNanos > 0 || stat.AllocSpace > 0 || stat.AllocObjects > 0 {
			marker = colorize(m.st.color, "32", ">")
		}
		cpuText := colorCPU(m.st.color, stat.CPUNanos, fmt.Sprintf("%8s", fmtDurationNS(stat.CPUNanos)))
		memText := colorMem(m.st.color, stat.AllocSpace, fmt.Sprintf("%8s", fmtBytes(stat.AllocSpace)))
		objText := colorObj(m.st.color, stat.AllocObjects, fmt.Sprintf("%6d", stat.AllocObjects))
		waitText := fmt.Sprintf("%8s", fmtDurationNS(stat.AsyncWaitNS))
		runText := fmt.Sprintf("%8s", fmtDurationNS(stat.AsyncRunNS))
		rendered := fmt.Sprintf(
			"%s %4d | %s %s %s %s %s | %s",
			marker, lineNo, cpuText, memText, objText, waitText, runText, highlightLineJS(line, m.st.color),
		)
		b.WriteString(truncate(rendered, width) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func padRight(s string, width int) string {
	visible := ansiStrip(s)
	if len(visible) >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-len(visible))
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	visible := ansiStrip(s)
	if len(visible) <= width {
		return s
	}
	// Avoid cutting ANSI escapes; fall back to plain text in narrow views.
	if visible != s {
		s = visible
	}
	if width < 4 {
		return s[:width]
	}
	return s[:width-1]
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func ansiStrip(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func usage() {
	fmt.Println("pprof_viewer -pprof <profile.pprof> -archive <archive.tar>")
	fmt.Println("or: pprof_viewer -pprof-init <init.pprof> -pprof-vu <vu.pprof> -archive <archive.tar>")
}

func main() {
	var (
		pprofPath         string
		pprofInitPath     string
		pprofVUPath       string
		pprofCombinedPath string
		archivePath       string
		sortMetric        string
		noColor           bool
	)
	flag.StringVar(&pprofPath, "pprof", "", "path to pprof profile")
	flag.StringVar(&pprofInitPath, "pprof-init", "", "path to init scope pprof profile")
	flag.StringVar(&pprofVUPath, "pprof-vu", "", "path to vu scope pprof profile")
	flag.StringVar(&pprofCombinedPath, "pprof-combined", "", "path to combined scope pprof profile")
	flag.StringVar(&archivePath, "archive", "", "path to k6 archive.tar")
	flag.StringVar(&sortMetric, "sort", "alloc_space", "initial sort metric: cpu|alloc_space|alloc_objects")
	flag.BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	flag.Parse()

	if archivePath == "" {
		usage()
		os.Exit(2)
	}
	scopeProfiles := make(map[string]*profile.Profile)
	openProfile := func(p string) (*profile.Profile, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return profile.Parse(f)
	}
	if pprofPath != "" {
		p, err := openProfile(pprofPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse profile: %v\n", err)
			os.Exit(1)
		}
		scopeProfiles["combined"] = p
	}
	if pprofInitPath != "" {
		p, err := openProfile(pprofInitPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse init profile: %v\n", err)
			os.Exit(1)
		}
		scopeProfiles["init"] = p
	}
	if pprofVUPath != "" {
		p, err := openProfile(pprofVUPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse vu profile: %v\n", err)
			os.Exit(1)
		}
		scopeProfiles["vu"] = p
	}
	if pprofCombinedPath != "" {
		p, err := openProfile(pprofCombinedPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse combined profile: %v\n", err)
			os.Exit(1)
		}
		scopeProfiles["combined"] = p
	}
	if len(scopeProfiles) == 0 {
		usage()
		os.Exit(2)
	}
	if scopeProfiles["combined"] == nil {
		var toMerge []*profile.Profile
		if p := scopeProfiles["init"]; p != nil {
			toMerge = append(toMerge, p)
		}
		if p := scopeProfiles["vu"]; p != nil {
			toMerge = append(toMerge, p)
		}
		merged, err := profile.Merge(toMerge)
		if err != nil {
			fmt.Fprintf(os.Stderr, "merge profile scopes: %v\n", err)
			os.Exit(1)
		}
		scopeProfiles["combined"] = merged
	}
	files, mainFile, err := parseArchive(archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse archive: %v\n", err)
		os.Exit(1)
	}
	prof := scopeProfiles["combined"]
	perFile, perLine := aggregate(prof)
	scopeTotals := map[string]fileStats{}
	for scope, p := range scopeProfiles {
		pf, _ := aggregate(p)
		scopeTotals[scope] = sumStats(pf)
	}
	st := &appState{
		prof:        prof,
		sampleIdx:   sampleTypeIndex(prof),
		files:       files,
		mainFile:    mainFile,
		perFile:     perFile,
		perLine:     perLine,
		sortMetric:  sortMetric,
		color:       supportsColor() && !noColor,
		scopeTotals: scopeTotals,
	}
	m := newViewerModel(*st)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "run tui: %v\n", err)
		os.Exit(1)
	}
}
