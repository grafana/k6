package httpbin

import (
	"bytes"
	"embed"
	"path"
	"text/template"
)

//go:embed static/*
var staticAssets embed.FS

// staticAsset loads an embedded static asset by name.
func staticAsset(name string) ([]byte, error) {
	return staticAssets.ReadFile(path.Join("static", name))
}

// mustStaticAsset loads an embedded static asset by name, panicking on error.
func mustStaticAsset(name string) []byte {
	b, err := staticAsset(name)
	if err != nil {
		panic(err)
	}
	return b
}

func mustRenderTemplate(name string, data any) []byte {
	t := template.Must(template.New(name).Parse(string(mustStaticAsset(name)))).Option("missingkey=error")
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
