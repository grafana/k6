package compiler

import (
	"github.com/evanw/esbuild/pkg/api"
	"github.com/grafana/sobek/file"
	"github.com/grafana/sobek/parser"
)

// StripTypes transpiles the input source string and strip types from it.
// this is done using esbuild
func StripTypes(src, filename string) (code string, srcMap []byte, err error) {
	opts := api.TransformOptions{
		Loader:         api.LoaderTS,
		Sourcefile:     filename,
		Target:         api.ESNext,
		Format:         api.FormatDefault,
		Sourcemap:      api.SourceMapExternal,
		SourcesContent: api.SourcesContentInclude,
		LegalComments:  api.LegalCommentsNone,
		Platform:       api.PlatformNeutral,
		LogLevel:       api.LogLevelSilent,
		Charset:        api.CharsetUTF8,
	}

	result := api.Transform(src, opts)

	if hasError, err := esbuildCheckError(&result); hasError {
		return "", nil, err
	}

	return string(result.Code), result.Map, nil
}

func esbuildCheckError(result *api.TransformResult) (bool, error) {
	if len(result.Errors) == 0 {
		return false, nil
	}

	msg := result.Errors[0]
	err := &parser.Error{Message: msg.Text}

	if msg.Location != nil {
		err.Position = file.Position{
			Filename: msg.Location.File,
			Line:     msg.Location.Line,
			Column:   msg.Location.Column,
		}
	}

	return true, err
}
