package lib

import (
	"fmt"
	goplugin "plugin"

	"github.com/loadimpact/k6/plugin"
)

// LoadJavaScriptPlugin tries to load a dynamic library that should conform to
// the `plug.JavaScriptPlugin` interface.
func LoadJavaScriptPlugin(path string) (plugin.JavaScriptPlugin, error) {
	p, err := loadPlugin(path)
	if err != nil {
		return nil, err
	}

	jsSym, err := p.Lookup("JavaScriptPlugin")
	if err != nil {
		return nil, err
	}

	var jsPlugin plugin.JavaScriptPlugin
	jsPlugin, ok := jsSym.(plugin.JavaScriptPlugin)
	if !ok {
		return nil, fmt.Errorf("could not cast plugin to the right type")
	}

	return jsPlugin, nil
}

func loadPlugin(path string) (*goplugin.Plugin, error) {
	p, err := goplugin.Open(path)
	if err != nil {
		err = fmt.Errorf("error while loading plugin: %w", err)
	}
	return p, err
}
