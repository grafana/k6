// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"encoding/json"
	"io/fs"

	dassets "github.com/grafana/xk6-dashboard-assets"
	"github.com/sirupsen/logrus"
)

type assets struct {
	config json.RawMessage
	ui     fs.FS
	report []byte
}

func newAssets() *assets {
	return &assets{
		ui:     dassets.UI(),
		report: dassets.Report(),
		config: dassets.Config(),
	}
}

func newCustomizedAssets(proc *process) *assets {
	assets := newAssets()

	custom, err := customize(assets.config, proc)
	if err != nil {
		logrus.Fatal(err)
	}

	assets.config = custom

	return assets
}
