package compiler

import (
	"strings"

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

	if hasError, err := esbuildCheckError(&result, src); hasError {
		return "", nil, err
	}

	return string(result.Code), result.Map, nil
}

func esbuildCheckError(result *api.TransformResult, src string) (bool, error) {
	if len(result.Errors) == 0 {
		return false, nil
	}

	msg := result.Errors[0]
	err := &parser.Error{Message: msg.Text, File: file.NewFile("", src, 1)}

	if msg.Location != nil {
		err.File = file.NewFile(msg.Location.File, src, 1)
		for line := 1; line < msg.Location.Line; line++ {
			next := strings.IndexByte(src[err.Offset:], '\n')
			if next < 0 {
				break
			}
			err.Offset += next + 1
		}
		err.Offset += max(msg.Location.Column-1, 0)
	}

	return true, err
}
