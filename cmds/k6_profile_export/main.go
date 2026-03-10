package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/pprof/profile"
)

type lineMetric struct {
	CPUNanos     int64 `json:"cpu_ns"`
	AllocSpace   int64 `json:"alloc_space"`
	AllocObjects int64 `json:"alloc_objects"`
	Samples      int64 `json:"samples"`
}

type output struct {
	Files  map[string]map[int]lineMetric `json:"files"`
	Totals lineMetric                    `json:"totals"`
}

type fileLine struct {
	file string
	line int
}

type aggregateResult struct {
	Files  map[string]map[int]lineMetric
	Totals lineMetric
}

var importPathRe = regexp.MustCompile(`^\s*import\s+(?:[^"']+?\s+from\s+)?["']([^"']+)["']`)
var exportFromPathRe = regexp.MustCompile(`^\s*export\s+.*\s+from\s+["']([^"']+)["']`)

func normalizeProfilePath(raw string) string {
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil {
		switch {
		case u.Scheme == "file":
			p, err := url.PathUnescape(u.Path)
			if err == nil {
				return path.Clean(p)
			}
			return path.Clean(u.Path)
		case u.Scheme == "":
		default:
			return raw
		}
	}
	return path.Clean(raw)
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

func addMetric(out map[string]map[int]lineMetric, file string, line int, samples, cpu, allocSpace, allocObjects int64) {
	m := out[file]
	if m == nil {
		m = make(map[int]lineMetric)
		out[file] = m
	}
	v := m[line]
	v.Samples += samples
	v.CPUNanos += cpu
	v.AllocSpace += allocSpace
	v.AllocObjects += allocObjects
	m[line] = v
}

func collectProfileFiles(p *profile.Profile) map[string]struct{} {
	out := make(map[string]struct{})
	for _, fn := range p.Function {
		file := normalizeProfilePath(fn.Filename)
		if file == "" || file == "<native>" {
			continue
		}
		out[file] = struct{}{}
	}
	for _, s := range p.Sample {
		for _, loc := range s.Location {
			if len(loc.Line) == 0 || loc.Line[0].Function == nil {
				continue
			}
			file := normalizeProfilePath(loc.Line[0].Function.Filename)
			if file == "" || file == "<native>" {
				continue
			}
			out[file] = struct{}{}
		}
	}
	return out
}

func resolvedImportCandidates(importerFile, importPath string) []string {
	if !strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "../") {
		return nil
	}
	base := filepath.Dir(importerFile)
	resolved := normalizeProfilePath(filepath.Join(base, importPath))
	if path.Ext(resolved) != "" {
		return []string{resolved}
	}
	return []string{
		resolved,
		resolved + ".js",
		resolved + ".mjs",
		resolved + ".ts",
		path.Join(resolved, "index.js"),
		path.Join(resolved, "index.mjs"),
		path.Join(resolved, "index.ts"),
	}
}

func buildImportAttributionIndex(files map[string]struct{}) map[string][]fileLine {
	index := make(map[string][]fileLine)
	for importer := range files {
		b, err := os.ReadFile(importer)
		if err != nil {
			continue
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			m := importPathRe.FindStringSubmatch(line)
			if len(m) < 2 {
				m = exportFromPathRe.FindStringSubmatch(line)
			}
			if len(m) < 2 {
				continue
			}
			for _, candidate := range resolvedImportCandidates(importer, m[1]) {
				if _, ok := files[candidate]; ok {
					index[candidate] = append(index[candidate], fileLine{file: importer, line: i + 1})
					break
				}
			}
		}
	}
	return index
}

func aggregate(p *profile.Profile) aggregateResult {
	idx := sampleTypeIndex(p)
	out := make(map[string]map[int]lineMetric)
	res := aggregateResult{Files: out}
	importIndex := buildImportAttributionIndex(collectProfileFiles(p))

	for _, s := range p.Sample {
		if len(s.Location) == 0 {
			continue
		}

		cpu := sampleValue(s, idx["cpu"])
		allocSpace := sampleValue(s, idx["alloc_space"])
		allocObjects := sampleValue(s, idx["alloc_objects"])
		samples := sampleValue(s, idx["samples"])

		res.Totals.CPUNanos += cpu
		res.Totals.AllocSpace += allocSpace
		res.Totals.AllocObjects += allocObjects
		res.Totals.Samples += samples

		// Inclusive attribution: apply sample cost to each JS frame in the stack,
		// so callers and callees both receive attribution.
		seen := make(map[string]struct{}, len(s.Location))
		topFile := ""
		hasCrossFileCaller := false
		for i, loc := range s.Location {
			if len(loc.Line) == 0 || loc.Line[0].Function == nil {
				continue
			}
			fn := loc.Line[0].Function
			file := normalizeProfilePath(fn.Filename)
			if file == "" || file == "<native>" {
				continue
			}
			line := int(loc.Line[0].Line)

			if i == 0 {
				topFile = file
			} else if topFile != "" && file != topFile {
				hasCrossFileCaller = true
			}

			key := fmt.Sprintf("%s:%d", file, line)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			addMetric(out, file, line, samples, cpu, allocSpace, allocObjects)
		}

		// Import attribution fallback: if a sample only exposes frames in one file,
		// attribute it to importer lines of that file as well.
		if topFile != "" && !hasCrossFileCaller {
			for _, imp := range importIndex[topFile] {
				key := fmt.Sprintf("%s:%d", imp.file, imp.line)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				addMetric(out, imp.file, imp.line, samples, cpu, allocSpace, allocObjects)
			}
		}
	}
	return res
}

func main() {
	var pprofPath string
	flag.StringVar(&pprofPath, "pprof", "", "path to pprof profile")
	flag.Parse()

	if pprofPath == "" {
		fmt.Fprintln(os.Stderr, "missing -pprof argument")
		os.Exit(2)
	}

	f, err := os.Open(pprofPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open profile: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	p, err := profile.Parse(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse profile: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	agg := aggregate(p)
	if err := enc.Encode(output{Files: agg.Files, Totals: agg.Totals}); err != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", err)
		os.Exit(1)
	}
}
