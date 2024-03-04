package modules

import (
	"errors"
	"fmt"
	"plugin"
	"strings"

	"go.k6.io/k6/ext"
)

func (mr *ModuleResolver) dynamicLoad(name string) (interface{}, error) {
	path := strings.TrimPrefix(name, "k6/dynamicLoad/")
	// TODO: Obviously this needs more checks and likely better loading idea.
	_, err := plugin.Open(path + ".so") // only init is important so we do not care about symbols
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
