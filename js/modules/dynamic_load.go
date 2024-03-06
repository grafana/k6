package modules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"plugin"
	"runtime/debug"
	"strings"

	"go.k6.io/k6/ext"
)

type pluginRequest struct {
	DebugInfo  string `json:"debugInfo"`
	PluginName string `json:"pluginName"`
}

func (mr *ModuleResolver) dynamicLoad(name string) (interface{}, error) {
	info, _ := debug.ReadBuildInfo()

	pluginNameCleaned := strings.TrimPrefix(name, "k6/dynamicLoad/")
	request := pluginRequest{DebugInfo: info.String(), PluginName: pluginNameCleaned}
	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// TODO: Obviously this needs more checks and likely better loading idea.
	f, err := os.CreateTemp(os.TempDir(), pluginNameCleaned+".so")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return nil, err
	}

	_, err = plugin.Open(f.Name()) // only init is important so we do not care about symbols
	if err != nil {
		return nil, fmt.Errorf("error loading %s from disk: %w", name, err)
	}
	jsExtensions := ext.Get(ext.JSExtension)
	for extensionName, extension := range jsExtensions {
		if _, ok := mr.goModules[extensionName]; !ok {
			mr.goModules[extensionName] = extension.Module
			mr.goModules[name] = extension.Module // short-circuit next time
			return extension.Module, nil
		}
	}

	return nil, errors.New("no new extension was added")
}
