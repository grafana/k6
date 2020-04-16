package lib

import (
	"fmt"
	goplugin "plugin"

	"github.com/loadimpact/k6/plugin"
)

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
	// TODO: check for things like the file existing and return nicer errors
	p, err := goplugin.Open(path)
	return p, err
}
