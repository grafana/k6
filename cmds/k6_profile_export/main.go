package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/google/pprof/profile"
)

type lineMetric struct {
	CPUNanos     int64 `json:"cpu_ns"`
	AllocSpace   int64 `json:"alloc_space"`
	AllocObjects int64 `json:"alloc_objects"`
	Samples      int64 `json:"samples"`
}

type output struct {
	Files map[string]map[int]lineMetric `json:"files"`
}

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

func aggregate(p *profile.Profile) map[string]map[int]lineMetric {
	idx := sampleTypeIndex(p)
	out := make(map[string]map[int]lineMetric)
	for _, s := range p.Sample {
		if len(s.Location) == 0 {
			continue
		}
		loc := s.Location[0]
		if len(loc.Line) == 0 || loc.Line[0].Function == nil {
			continue
		}
		fn := loc.Line[0].Function
		file := normalizeProfilePath(fn.Filename)
		if file == "" || file == "<native>" {
			continue
		}
		line := int(loc.Line[0].Line)
		m := out[file]
		if m == nil {
			m = make(map[int]lineMetric)
			out[file] = m
		}
		v := m[line]
		v.Samples += sampleValue(s, idx["samples"])
		v.CPUNanos += sampleValue(s, idx["cpu"])
		v.AllocSpace += sampleValue(s, idx["alloc_space"])
		v.AllocObjects += sampleValue(s, idx["alloc_objects"])
		m[line] = v
	}
	return out
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
	if err := enc.Encode(output{Files: aggregate(p)}); err != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", err)
		os.Exit(1)
	}
}
