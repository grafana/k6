package compiler

import (
	"path/filepath"

	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
	"github.com/evanw/esbuild/pkg/api"
)

func esbuildTransform(src, filename string) (code string, srcMap []byte, err error) {
	opts := api.TransformOptions{
		Sourcefile:     filename,
		Loader:         api.LoaderJS,
		Target:         api.ESNext,
		Format:         api.FormatCommonJS,
		Sourcemap:      api.SourceMapExternal,
		SourcesContent: api.SourcesContentInclude,
		LegalComments:  api.LegalCommentsNone,
		Platform:       api.PlatformNeutral,
		LogLevel:       api.LogLevelSilent,
		Charset:        api.CharsetUTF8,
	}

	if filepath.Ext(filename) == ".ts" {
		opts.Loader = api.LoaderTS
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
