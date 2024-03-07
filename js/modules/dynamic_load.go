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

const pluginLoadPrefix = "plugin:"

func (mr *ModuleResolver) dynamicLoad(name string) (interface{}, error) {
	info, _ := debug.ReadBuildInfo()

	pluginNameCleaned := strings.TrimPrefix(name, pluginLoadPrefix)
	request := pluginRequest{DebugInfo: info.String(), PluginName: pluginNameCleaned}
	var filename string
	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(pluginNameCleaned, "./") || strings.HasPrefix(pluginNameCleaned, "../") { //nolint:nestif
		f, err := os.Open(pluginNameCleaned) //nolint:forbidigo,gosec,govet
		if err != nil {
			return nil, err
		}
		defer f.Close() //nolint:errcheck
		filename = f.Name()
	} else {
		resp, err := http.Post("http://localhost:8080", "application/json", bytes.NewReader(b)) //nolint:noctx,govet
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		// TODO: Obviously this needs more checks and likely better loading idea.
		f, err := os.CreateTemp(os.TempDir(), "dynamicLoading*.so") //nolint:forbidigo
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = os.Remove(f.Name()) //nolint:forbidigo
		}()

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return nil, err
		}
		filename = f.Name()
	}

	_, err = plugin.Open(filename) // only init is important so we do not care about symbols
	if err != nil {
		return nil, fmt.Errorf("error loading %s from disk: %w", name, err)
	}
	jsExtensions := ext.Get(ext.JSExtension)
	for extensionName, extension := range jsExtensions {
		if _, ok := mr.goModules[extensionName]; !ok {
			// fmt.Println("Recognised", name, " as ", extensionName, extension.Path, extension.Version, extension.Name)
			mr.goModules[extensionName] = extension.Module
			mr.goModules[name] = extension.Module // short-circuit next time
			return extension.Module, nil
		}
	}

	return nil, errors.New("no new extension was added")
}
