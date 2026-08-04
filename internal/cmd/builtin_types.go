package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	pathpkg "path"
	"path/filepath"
	"strings"

	"go.k6.io/k6/v2/lib/fsext"
)

const builtinK6TypesSource = "builtin-types/k6"

// builtinK6Types contains the declarations used by generated typecheck and LSP projects.
//
//go:embed builtin-types/LICENSE builtin-types/k6
var builtinK6Types embed.FS

type materializedBuiltinTypes struct {
	typeRoot string
	paths    map[string][]string
}

func materializeBuiltinK6Types(fileSystem fsext.Fs, typesDir string) (materializedBuiltinTypes, error) {
	packageDir := filepath.Join(typesDir, "builtin", "k6")
	result := materializedBuiltinTypes{
		typeRoot: filepath.Dir(packageDir),
		paths:    make(map[string][]string),
	}

	err := fs.WalkDir(builtinK6Types, builtinK6TypesSource, func(sourcePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relativePath := strings.TrimPrefix(sourcePath, builtinK6TypesSource+"/")
		if pathpkg.Base(relativePath) != "package.json" && !strings.HasSuffix(relativePath, ".d.ts") {
			return nil
		}

		contents, err := builtinK6Types.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read embedded k6 declaration %s: %w", sourcePath, err)
		}
		target := filepath.Join(packageDir, filepath.FromSlash(relativePath))
		if err := fileSystem.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create built-in k6 types directory: %w", err)
		}
		if err := fsext.WriteFile(fileSystem, target, contents, 0o644); err != nil {
			return fmt.Errorf("write built-in k6 declaration %s: %w", target, err)
		}

		moduleName, ok := builtinK6ModuleName(relativePath)
		if ok {
			result.paths[moduleName] = []string{target}
		}
		return nil
	})
	if err != nil {
		return materializedBuiltinTypes{}, fmt.Errorf("materialize built-in k6 types: %w", err)
	}

	return result, nil
}

func builtinK6ModuleName(relativePath string) (string, bool) {
	if relativePath == "index.d.ts" {
		return "k6", true
	}
	if pathpkg.Base(relativePath) != "index.d.ts" {
		return "", false
	}
	return "k6/" + pathpkg.Dir(relativePath), true
}
