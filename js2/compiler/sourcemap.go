package compiler

type SourceMap struct {
	Version    int
	File       string
	SourceRoot string
	Sources    []string
	Names      []string
	Mappings   string
}
