// Package assets contains embedded static assets for xk6-dashboard.
package assets

import (
	"embed"
	"encoding/json"
	"io/fs"
)

var (
	//go:embed packages/ui/dist/*
	ui embed.FS
	//go:embed packages/report/dist/index.html
	report []byte
	//go:embed packages/config/dist/config.json
	config []byte
)

// UI returns an embedded filesystem containing the dashboard's single-page application.
func UI() fs.FS {
	sub, err := fs.Sub(ui, "packages/ui/dist")
	if err != nil {
		panic(err)
	}

	return sub
}

// Report returns the embedded HTML content for the report single-page application.
func Report() []byte {
	return report
}

// Config returns the embedded default configuration as raw JSON.
func Config() json.RawMessage {
	return config
}
