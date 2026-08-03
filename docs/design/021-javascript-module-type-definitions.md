# Type definitions for remote and extension JavaScript modules

## Purpose

k6 scripts can import modules that the standard TypeScript language service does not know how to
resolve:

- remote modules, identified by an HTTP or HTTPS URL;
- k6 extension modules, identified by a specifier such as `k6/x/example`.

k6 can resolve these specifiers when it runs a script, but `tsserver` normally cannot associate them
with a TypeScript declaration. The result is a working k6 script with missing completion, navigation,
documentation, and type checking in the editor.

This document describes a reproducible workflow that provides those declarations to `tsserver`
without changing the import that k6 executes. It uses
[`compilerOptions.paths`](https://www.typescriptlang.org/tsconfig/paths.html) as the bridge between a
runtime module specifier and a local, versioned `.d.ts` file.

The checked-in remote example uses the exact AJV version also distributed by jslib.k6.io:

```javascript
import Ajv from "https://esm.sh/ajv@6.12.5?bundle";
```

esm.sh serves the JavaScript with an `X-TypeScript-Types` header pointing to
`https://esm.sh/ajv@6.12.5/lib/ajv.d.ts`. This makes the example reproducible without a custom
server, certificate, or manually maintained declaration.

## What this configuration does

There are two independent resolution systems:

1. k6 resolves and executes the JavaScript named by the import specifier.
2. `tsserver` resolves the same specifier to a declaration file for editor tooling.

The [`paths` option does not rewrite emitted imports][typescript-paths]. Consequently, the script
keeps its real k6 runtime import while TypeScript reads a local type-only representation of the
module.

```text
script.js
  import "https://esm.sh/ajv@6.12.5?bundle"
                |                              |
                | k6 runtime                   | tsserver + paths
                v                              v
  https://esm.sh/.../ajv.bundle.mjs    /tmp/k6-types-<random>/types/.../ajv...d.ts
```

This is editor and type-checker configuration only. It does not make k6 use Node.js module
resolution, bundle the script, download the runtime module, or change module resolution inside k6.

## Why Deno handles this directly

Deno owns the entire module-loading and type-checking pipeline. Its language server understands URL,
`jsr:`, and `npm:` specifiers, maintains a module cache, and knows how to associate JavaScript with
declarations. For HTTP JavaScript modules, a server can advertise declarations with the
`X-TypeScript-Types` response header. Deno also supports type directives in a JavaScript module or at
an import site. See Deno's
[`Providing declaration files`](https://docs.deno.com/runtime/fundamentals/typescript/#providing-declaration-files)
documentation.

Standard `tsserver` does not fetch arbitrary HTTP modules and does not use k6's runtime resolver.
That is why this workflow first puts the declaration on disk and then tells TypeScript how to find
it. k6 does not need to reimplement TypeScript's language features: the `k6 lsp` wrapper automates
project generation and delegates completion, hover, navigation, and diagnostics to an existing
TypeScript language server.

## Generated project layout

By default, each `k6 typecheck` invocation creates and retains a complete project in a randomly
named temporary directory:

```text
/tmp/k6-types-755373299/
├── tsconfig.json
└── types/
    ├── remotes/
    │   └── <host>/...
    └── extensions/
        └── <module>/<version>/index.d.ts
```

The command prints the exact paths and does not delete the project. This isolates concurrent and
successive runs while leaving all checker inputs available for inspection. `--in-place` instead
writes `tsconfig.json` in the current working directory and declarations below `.k6/types/`. It
refuses to overwrite an existing `tsconfig.json` unless the target is selected explicitly with
`--tsconfig` or `.k6/tsconfig.generated` identifies it as output from an earlier invocation.
Including the version in a remote URL or extension cache path is important: a declaration describes
a particular runtime API and should not silently be reused after the implementation changes.

Generated projects are machine-local artifacts. The repository does not check in their absolute
paths or downloaded declarations.

## TypeScript configuration

For the AJV example, `k6 typecheck` generates a configuration with this shape:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "allowJs": true,
    "checkJs": true,
    "noEmit": true,
    "strict": true,
    "skipLibCheck": false,
    "allowImportingTsExtensions": true,
    "types": ["k6"],
    "paths": {
      "https://esm.sh/ajv@6.12.5?bundle": [
        "/tmp/k6-types-755373299/types/remotes/esm.sh/ajv@6.12.5-<query-hash>.d.ts"
      ]
    }
  },
  "files": ["/absolute/path/to/examples/typecheck/remote.js"]
}
```

The relevant settings are:

- `allowJs` lets TypeScript load JavaScript k6 scripts.
- `checkJs` reports type errors in those JavaScript files. It can be disabled when only completion
  and navigation are wanted.
- `noEmit` makes TypeScript an editor and validation tool; k6 remains responsible for execution and
  TypeScript transformation.
- `moduleResolution: "Bundler"` accepts application-defined module mappings without imposing the
  Node.js runtime's resolution rules.
- `types: ["k6"]` loads the built-in k6 API declarations from `@types/k6`.
- `paths` maps the exact runtime specifier to the declaration cached in this project.
- `files` selects the script that was passed to `k6 typecheck` even though the generated project is
  under `/tmp`.

There is no `include` or `exclude` section. The command writes one explicit `files` entry for the
requested script.

## One-time project setup

Install TypeScript and the declarations for k6's built-in modules as development dependencies:

```bash
npm install --save-dev typescript @types/k6
```

These packages are authoring dependencies. k6 does not load them while running the script.

No manual declaration installation is needed for AJV. The command follows its advertised types URL,
downloads the declaration into the generated project, and writes the corresponding `paths` entry.

## Obtaining a declaration

Use the following order of preference for each remote module or extension:

1. Use a declaration published by the module author for that exact release.
2. Generate a declaration from the release's TypeScript sources or its build output.
3. Write a declaration from the documented public API and verify it against the implementation.

Do not point a versioned runtime import at declarations from an unpinned branch such as `main`. The
types could describe exports or signatures that the downloaded JavaScript does not implement.

AJV's runtime import and declaration are both pinned to `6.12.5`. The `?bundle` query affects the
runtime JavaScript selected by esm.sh and is therefore part of the exact module specifier used as
the `paths` key.

Treat remotely obtained declarations as code: pin the source, review it, and cache it in a location
that can be reproduced in development and CI. If a future synchronization command downloads these
files, it should record the resolved URL, version, and integrity hash in a manifest.

## Adding another remote module

For a second URL import:

1. Create a versioned location below `.k6/types/`.
2. Put the module's declaration at that location.
3. Add an exact `paths` entry whose key is identical to the script's import string.

For example:

```json
{
  "compilerOptions": {
    "paths": {
      "https://example.test/library/2.3.0/index.js": [
        "./.k6/types/library/2.3.0/index.d.ts"
      ]
    }
  }
}
```

Query strings, fragments, trailing slashes, and redirects can make otherwise similar URLs distinct
module specifiers. The mapping key must match what appears in source code. Prefer stable, canonical,
versioned URLs.

## Adding an extension module

The same mechanism works for an extension's bare k6 specifier:

```json
{
  "compilerOptions": {
    "paths": {
      "k6/x/example": [
        "./.k6/types/extensions/k6-x-example/1.2.3/index.d.ts"
      ]
    }
  }
}
```

The JavaScript continues to use the specifier registered by the extension:

```javascript
import example from "k6/x/example";
```

The declaration version must match the extension compiled into the k6 binary. The mapping cannot
detect that version or prove the extension is installed; it only describes the module to TypeScript.
An automated k6 workflow would need to inspect the binary's extension metadata before selecting or
downloading declarations.

## Prototype command

The prototype adds a command that performs the configuration and type-checking workflow:

```bash
k6 typecheck test.js
```

It performs the following operations:

1. Uses k6's module resolver to load the script's complete static import graph.
2. Finds remote URL imports and `k6/x/*` extension imports in that graph.
3. Resolves or downloads a declaration for each supported import.
4. Writes a generated project to `/tmp/k6-types-<random>/tsconfig.json`.
5. Runs `tsgo --project <generated-config>`, falling back to `tsc` when `tsgo` is unavailable.

The command uses the actual k6 resolver, so nested imports and canonical remote URLs are represented
the same way they are during a k6 run. It does not execute the test or its VU code.

### Installing a checker

The command first looks in the project's `node_modules/.bin` and then in `PATH`. Install either the
native TypeScript checker:

```bash
npm install --save-dev @typescript/native-preview @types/k6
```

or the JavaScript implementation:

```bash
npm install --save-dev typescript @types/k6
```

`tsgo` is preferred when both are available. A specific checker can be selected explicitly:

```bash
k6 typecheck --checker tsc test.js
k6 typecheck --checker /absolute/path/to/tsgo test.js
```

`tsgo` and `tsc` perform one-shot project checking. `tsserver` and the TypeScript 7 language server
provide the long-running editor protocol; they are not checker executables and should not be invoked
with the same command line.

## Prototype language server wrapper

The prototype also exposes a standard stdio LSP command:

```bash
k6 lsp test.js
```

This command is a supervisor and protocol proxy, not a new implementation of a TypeScript language
server. It:

1. Loads the entry script and its static import graph with k6's resolver.
2. Generates the same exact `paths` mappings used by `k6 typecheck`.
3. Starts a normal TypeScript language server as a child process.
4. Proxies LSP messages between the editor and that process without changing requests or responses.
5. Watches the resolved local source graph and refreshes mappings when an import changes.
6. Notifies the child language server that its generated configurations changed.

The local graph is polled every 500 milliseconds. The wrapper also refreshes after the editor sends
`textDocument/didSave` or `workspace/didChangeWatchedFiles`. Refresh requests are debounced so a
burst of file and editor events produces one k6 dependency scan.

### tsgo backend

The preferred backend is the native TypeScript language server included in
`@typescript/native-preview`:

```bash
npm install --save-dev @typescript/native-preview @types/k6
k6 lsp --server tsgo test.js
```

The child invocation is:

```bash
tsgo --lsp --stdio
```

The complete generated project is written to `/tmp/k6-lsp-<random>/tsconfig.json`, with declarations
below `/tmp/k6-lsp-<random>/types/`. tsgo selects configurations by walking parent directories, so
the wrapper also creates a small workspace bridge named `tsconfig.k6.<pid>.json`. It extends the
complete temporary configuration. After the client's `initialized` notification, the wrapper sends
this setting to tsgo:

```json
{
  "settings": {
    "js/ts": {
      "customConfigFileName": "tsconfig.k6.<pid>.json"
    }
  }
}
```

This uses tsgo's `customConfigFileName` preference and avoids replacing a user's `tsconfig.json`.
Both the bridge and temporary project are removed when the language-server process exits. Pass
`--in-place` to write the full project directly to `./tsconfig.json` and `.k6/types/`. In this mode
the conventional configuration is discovered without a bridge and remains after the language
server exits. The command refuses to overwrite an existing user-owned `tsconfig.json`, while the
`.k6/tsconfig.generated` marker allows later k6 invocations to update their own generated file.

### tsserver backend

The JavaScript `tsserver` process uses its own protocol rather than LSP. Editors that speak LSP need
the `typescript-language-server` adapter:

```bash
npm install --save-dev typescript typescript-language-server @types/k6
k6 lsp --server tsserver test.js
```

For this backend the child invocation is:

```bash
typescript-language-server --stdio
```

Unlike tsgo, this adapter has no setting for selecting an arbitrary configuration filename. The
prototype therefore creates a temporary conventional `tsconfig.json` bridge in the working
directory. It refuses to start when that file already exists; it never overwrites a project
configuration. Use the tsgo backend in an existing configured TypeScript project.

`--server auto` tries a project-local or `PATH`-visible `tsgo` first, then
`typescript-language-server`. `--server-path` can select a particular executable and requires an
explicit `--server tsgo` or `--server tsserver` so k6 knows which arguments and configuration
strategy that executable needs.

### Editor contract

Configure the editor to start this command as a stdio language server from the workspace root:

```text
/absolute/path/to/k6 lsp /absolute/path/to/test.js
```

The entry script is required because it defines the k6 import graph to resolve. stdout contains only
LSP frames; k6 warnings and child-server logs use stderr. A workspace should run one wrapper for each
independent entry script whose graph needs different remote or extension declarations.

For a minimal Neovim configuration, start the wrapper after a JavaScript buffer receives its
`FileType`, then use `LspAttach` for buffer-local behavior. Deferring startup allows the current
buffer to become the default entry script:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = { "javascript", "typescript" },
  callback = function(event)
    local entry = vim.api.nvim_buf_get_name(event.buf)
    local root = vim.fs.root(entry, { ".git" }) or vim.fs.dirname(entry)
    vim.lsp.start({
      name = "k6",
      cmd = { "/absolute/path/to/k6", "lsp", entry },
      cmd_cwd = root,
      root_dir = root,
    }, { bufnr = event.buf })
  end,
})

vim.api.nvim_create_autocmd("LspAttach", {
  callback = function(event)
    local client = vim.lsp.get_client_by_id(event.data.client_id)
    if client ~= nil and client.name == "k6" and client:supports_method("textDocument/completion") then
      vim.lsp.completion.enable(true, client.id, event.buf, { autotrigger = true })
    end
  end,
})
```

[`examples/typecheck/neovim.lua`](../../examples/typecheck/neovim.lua) provides a complete standalone
Neovim 0.11 configuration with entry-script selection, workspace attachment, diagnostics, and
runnable setup instructions.

VS Code's built-in JavaScript/TypeScript extension talks directly to `tsserver`, not to arbitrary
stdio LSP commands. Using `k6 lsp` there requires a small generic LSP client extension or a dedicated
k6 extension that launches the same command; the k6 process itself can remain this thin proxy.

### Generating without checking

To inspect the project or hand it to another tool without starting a checker:

```bash
k6 typecheck --generate-only test.js
```

The generated project contains an explicit `files` entry for the script and absolute declaration
targets. It records every ancestor `node_modules/@types` directory as a `typeRoots` candidate, so
moving `tsconfig.json` into `/tmp` does not hide the project's `@types/k6` dependency. It also
disables `skipLibCheck`, so a missing built-in declaration or an invalid downloaded declaration is
reported at its source instead of degrading into misleading inferred types. The generated project
is machine-local and should be regenerated rather than moved between workspaces.

To generate `./tsconfig.json` and `.k6/types/` in the working directory instead:

```bash
k6 typecheck --generate-only --in-place test.js
```

The implicit in-place path is protected. If a user-owned `tsconfig.json` already exists, the command
stops rather than replacing it. k6 records its own output in `.k6/tsconfig.generated`, allowing
subsequent in-place and watch invocations to update the generated file. Passing an explicit target
is the opt-in replacement mechanism:

```bash
k6 typecheck --generate-only --in-place --tsconfig ./tsconfig.json test.js
```

### Continuous checking

`--watch` keeps the selected checker running and also watches k6's resolved local import graph:

```bash
k6 typecheck --watch test.js
k6 typecheck --in-place --watch test.js
```

The child process receives `--project <generated-config> --watch`, so normal source edits are handled
by tsgo or tsc. In parallel, k6 polls the resolved local files. When an edit changes a URL import,
extension import, or local dependency graph, k6 reloads the entry script, refreshes declarations,
and rewrites `files` and `paths`. The checker observes the configuration update and reloads the
project. Refresh events are debounced using the same mechanism as `k6 lsp`.

`--watch` and `--generate-only` cannot be combined because generate-only mode has no long-running
checker to supervise.

The output and cache locations can be changed:

```bash
k6 typecheck \
  --tsconfig .cache/k6/tsconfig.json \
  --types-dir .cache/k6/types \
  test.js
```

Both relative locations are resolved from the command's working directory, not the script's
directory. This matters when checking a script in a temporary or external directory. For example,
the declaration prepared by this repository can be used while running from `/tmp` with:

```bash
cd /tmp
k6 typecheck \
  --types-dir /absolute/path/to/k6-worktree/.k6/types \
  test.js
```

Running `tsgo test.js` or `tsc test.js` directly does not use the generated project. To invoke a
checker manually, use the configuration path printed by that invocation:

```bash
tsgo --project /tmp/k6-types-755373299/tsconfig.json
```

See [`examples/typecheck/README.md`](../../examples/typecheck/README.md) for runnable remote-header
and binary-embedded extension examples.

### Remote declaration discovery

For each HTTPS module, the command checks sources in this order:

1. the canonical cache below `<types-dir>/remotes/<host>/`;
2. the earlier URL-shaped layout below `<types-dir>/`, retained for compatibility with existing
   caches;
3. the module's `X-TypeScript-Types` HTTP response header;
4. a declaration beside the JavaScript, such as `index.d.ts` for `index.js`.

Downloaded declarations are limited to 10 MiB and written under the canonical cache. Cache paths are
derived without allowing URL path traversal outside the types directory. An exact `paths` mapping is
emitted only when a declaration is found. Missing declarations produce warnings and remain visible
as normal TypeScript resolution errors.

This initial implementation expects a self-contained entry declaration. Recursively downloading
relative imports from a multi-file declaration graph is future work.

### Extension-provided declarations

The command first looks for a declaration cached below the selected generated types directory at the
extension's module name and compiled version:

```text
<types-dir>/extensions/k6-x-example/1.2.3/index.d.ts
```

An extension can instead embed and expose its own declaration by implementing
`modules.TypeScriptTypeProvider`:

```go
//go:embed index.d.ts
var typeScriptTypes []byte

func (*RootModule) TypeScriptTypes() []byte {
	return typeScriptTypes
}
```

Because this metadata comes from the module compiled into the running k6 binary, the declaration and
runtime implementation naturally share the same extension version. The command caches the embedded
declaration before generating the mapping.

## Validation

Run the TypeScript checker without executing the load test:

```bash
npx tsc --project tsconfig.json
```

To see why a module resolved to a particular file:

```bash
npx tsc --project tsconfig.json --traceResolution
```

For the generated project, use:

```bash
k6 typecheck test.js
tsgo --project /tmp/k6-types-755373299/tsconfig.json --traceResolution
```

Useful checks include:

- `ajv.compile({...})` accepts an object or Boolean JSON schema;
- `ajv.compile(42)` is rejected using the downloaded declaration;
- AJV methods and options appear in completion;
- the import string remains unchanged when the script is run by k6.

After changing `tsconfig.json`, restart the editor's TypeScript language service if it retains an old
resolution result. In Visual Studio Code, run **TypeScript: Restart TS server** from the command
palette.

The generated project sets `skipLibCheck` to `false`, so an invalid or incomplete downloaded
declaration is reported instead of silently degrading into inferred types.

## Updating a module version

When the runtime import changes from one version to another:

1. Fetch or generate the new version's declaration.
2. Store it in a new versioned directory.
3. Change the import in the script.
4. Change the corresponding `paths` key and target.
5. Run the validation checks.
6. Remove the old cached declaration only after no source file imports the old version.

Keeping both versions temporarily is valid: add one exact mapping for each URL. This makes migrations
explicit and allows different scripts in the same project to use different versions.

## Failure modes

### The editor says the module cannot be found

- Confirm the `paths` key exactly matches the import string.
- Resolve the target relative to `tsconfig.json`, not relative to the importing script.
- Confirm the target is a module declaration with top-level `export` declarations.
- Confirm the editor opened the script as part of the intended TypeScript project.
- Use `--traceResolution` and restart the editor's TypeScript service.

### Built-in k6 imports cannot be found

Install `@types/k6` and retain `"types": ["k6"]`. If the declaration refers to `k6/browser`, its
browser types also come from this package.

### Completion works but k6 fails at runtime

`paths` can describe a module that is absent or incompatible at runtime. Check the URL independently,
or check that the required extension and version are present in the k6 binary. TypeScript success is
not runtime module validation.

### k6 runs the script but the declaration is wrong

Confirm that the declaration and runtime module versions match. Regenerate or correct the cached
declaration, then validate it with representative calls. Do not fix a mismatch by weakening the
module to `any`; that only hides version drift.

## Editor and checker loop

The prototype remains a thin coordinator around standard TypeScript tools rather than implementing
a k6 type checker:

1. `k6 typecheck --watch` regenerates the selected project while tsgo or tsc performs continuous
   checking.
2. `k6 lsp` starts the TypeScript 7 LSP, or tsserver through an adapter, and proxies its standard
   input/output protocol to the editor.
3. Both modes refresh remote and extension declaration metadata when the resolved import graph
   changes.
4. The coordinator adds only k6-specific operations, such as module discovery or runtime-version
   diagnostics. Completion, hover, navigation, rename, and TypeScript diagnostics stay in the
   standard language service.

This division keeps the k6-specific surface small. k6 supplies an accurate runtime module graph and
declaration mappings; `tsgo`/`tsc` supplies type checking; and the TypeScript language service supplies
the actual LSP/editor behavior.

Before making the cache suitable for unattended watching or CI, it should also gain a manifest that
records declaration provenance and integrity, recursive declaration-graph fetching, stale-entry
cleanup, and a policy for authenticating or trusting remote type metadata.

[typescript-paths]: https://www.typescriptlang.org/docs/handbook/modules/reference.html#paths
