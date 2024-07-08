// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"encoding/json"
	"io"

	"go.k6.io/k6/lib/fsext"
)

const defaultAltConfig = ".dashboard.json"

func findDefaultConfig(fs fsext.Fs) string {
	if exists(fs, defaultAltConfig) {
		return defaultAltConfig
	}

	return ""
}

// customize allows using custom dashboard configuration.
func customize(uiConfig json.RawMessage, proc *process) (json.RawMessage, error) {
	filename, ok := proc.env["XK6_DASHBOARD_CONFIG"]
	if !ok || len(filename) == 0 {
		if filename = findDefaultConfig(proc.fs); len(filename) == 0 {
			return uiConfig, nil
		}
	}

	return loadConfigJSON(filename, proc)
}

func loadConfigJSON(filename string, proc *process) (json.RawMessage, error) {
	file, err := proc.fs.Open(filename)
	if err != nil {
		return nil, err
	}

	bin, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	conf := map[string]interface{}{}

	if err := json.Unmarshal(bin, &conf); err != nil {
		return nil, err
	}

	return json.Marshal(conf)
}

func exists(fs fsext.Fs, filename string) bool {
	if _, err := fs.Stat(filename); err != nil {
		return false
	}

	return true
}
