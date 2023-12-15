// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"

	"github.com/sirupsen/logrus"
)

type assets struct {
	config json.RawMessage
	ui     fs.FS
	report fs.FS
}

//go:embed assets/packages/ui/dist assets/packages/report/dist assets/packages/config/dist
var assetsFS embed.FS

const assetsPackages = "assets/packages/"

func newAssets() *assets {
	return newAssetsFrom(assetsFS)
}

func newCustomizedAssets(proc *process) *assets {
	assets := newAssetsFrom(assetsFS)

	custom, err := customize(assets.config, proc)
	if err != nil {
		logrus.Fatal(err)
	}

	assets.config = custom

	return assets
}

func newAssetsFrom(efs embed.FS) *assets {
	config, err := efs.ReadFile(assetsPackages + "config/dist/config.json")
	if err != nil {
		panic(err)
	}

	return &assets{
		ui:     assetDir(assetsPackages+"ui/dist", efs),
		report: assetDir(assetsPackages+"report/dist", efs),
		config: config,
	}
}

func assetDir(dirname string, parent fs.FS) fs.FS {
	subfs, err := fs.Sub(parent, dirname)
	if err != nil {
		panic(err)
	}

	return subfs
}
