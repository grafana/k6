# k6 type discovery examples

These examples exercise both declaration sources supported by the `k6 typecheck` prototype:

- [`remote.js`](remote.js) imports AJV from esm.sh. The public JavaScript response advertises its
  declaration with `X-TypeScript-Types`.
- [`extension.js`](extension.js) imports an extension whose declaration is embedded in the custom
  k6 binary.

Run all commands from the repository root.

## Why declarations should travel with modules

The goal is to align k6 with the wider JavaScript and TypeScript ecosystem, where a module can
publish and version its own declarations. A remote module can advertise its declaration alongside
the JavaScript it serves, while a k6 extension can embed its declaration in the binary that contains
the implementation. In both cases, the module author remains responsible for the public API and its
types, and k6 only connects that information to the standard TypeScript tooling.

Using centrally maintained `@types/*` packages for every k6 module would introduce a second release
and maintenance path. Declarations can lag behind the implementation, users must discover and
install the correct package through npm even when their k6 scripts do not otherwise use npm, and a
central registry becomes a bottleneck for additions and updates. It also forces an awkward package
layout: either every remote module and extension needs a separate package, or one very large package
must contain many independently versioned APIs. The former creates substantial package overhead;
the latter is difficult to maintain and makes matching declarations to runtime versions unreliable.

Most importantly, a central package cannot describe arbitrary custom, private, or locally built k6
extensions. Their APIs depend on the extensions compiled into a particular k6 binary. Letting each
extension embed its own declaration keeps the implementation and types together and makes the same
workflow work without registering that extension in a central repository first.

`@types/k6` is still used by this prototype for k6's built-in modules. This might change in the future.

Other notes:
* This still depends on tsserver/tsgo - reimplementing this is completely outside the scope here. While tsgo is now in golang - it doesn't have proper public golang API and it seems it might never have any. Future development might actually use tsgo plugin extensions or more integrated versions. This is skipped for this demo in order to support both and because tsgo doesn't support this currently.
* With the above it will actually be possible to have completely npm independent usage.
* The current implementation tries not to break anything for the user and to be as close to zero-config(after initial IDE/editor setup) as possible - it just use k6.
* There is no try to do good caching or to be fast - we are going for PoC.
* The current prototype uses esm.sh, but making this work for jslib will be fairly simple. Even more importantly it will be fairly simple to let esm.sh work with the jslibs directly from their repos wihtout any need for us to host jslib. Discussion on hosting jslibs is separate. But this does allow also easier third party hosting - they can just have github repo and use esm.sh.
* Same as the above for jsr (which is accesible from esm.sh) - we are not discussing support jsr:@someone/something here.

## Install a TypeScript checker

Install the native TypeScript checker and the declarations for the built-in k6 modules:

```bash
npm install --save-dev @typescript/native-preview @types/k6
```

Use `typescript` instead of `@typescript/native-preview` to run `tsc` rather than `tsgo`.

## Remote declarations from esm.sh

[`remote.js`](remote.js) imports the exact AJV version also distributed by jslib.k6.io:

```javascript
import Ajv from "https://esm.sh/ajv@6.12.5?bundle";
```

The import is pinned and `?bundle` keeps the runtime example to one remotely built module. Verify
that esm.sh advertises a declaration and that the declaration is available:

```bash
curl --head 'https://esm.sh/ajv@6.12.5?bundle'
curl --head 'https://esm.sh/ajv@6.12.5/lib/ajv.d.ts'
```

The first response should contain:

```text
x-typescript-types: https://esm.sh/ajv@6.12.5/lib/ajv.d.ts
```

Generate the TypeScript project without running a checker:

```bash
go run . typecheck --generate-only examples/typecheck/remote.js
```

The command prints a random project directory such as `/tmp/k6-types-755373299`. Its
`tsconfig.json` contains an exact mapping from the unchanged esm.sh import to the downloaded AJV
declaration:

```json
{
  "compilerOptions": {
    "paths": {
      "https://esm.sh/ajv@6.12.5?bundle": [
        "/tmp/k6-types-755373299/types/remotes/esm.sh/ajv@6.12.5-<query-hash>.d.ts"
      ]
    }
  }
}
```

Run discovery and type checking together:

```bash
go run . typecheck examples/typecheck/remote.js
```

To prove that AJV's declaration is active, temporarily change the schema passed to `ajv.compile()`
in `remote.js` to a number:

```javascript
const validate = ajv.compile(42);
```

Running the command again should report that `number` is not assignable to AJV's schema parameter.
Restore the original schema, then run the k6 script once to verify runtime loading independently:

```bash
go run . run --iterations 1 examples/typecheck/remote.js
```

No local server, development certificate, or custom trust configuration is required. Both the
runtime JavaScript and declaration use esm.sh's publicly trusted HTTPS endpoint.

## Generated project location

By default, each invocation creates and retains a complete project at a random path such as:

```text
/tmp/k6-types-755373299/
├── tsconfig.json
└── types/
    ├── extensions/...
    └── remotes/...
```

The command prints the exact directory and configuration path. The random project prevents one run
from overwriting another and makes it easy to inspect exactly what was given to the checker. It is
not deleted automatically.

To keep the generated state with the script instead, pass `--in-place`. This writes
`./tsconfig.json` and `.k6/types/` below the current working directory:

```bash
go run . typecheck --generate-only --in-place examples/typecheck/remote.js
```

`--tsconfig` and `--types-dir` can override either path explicitly.

The implicit in-place target refuses to overwrite an existing user-owned `tsconfig.json`. It writes
`.k6/tsconfig.generated` so subsequent k6 invocations can safely identify and update their own
configuration. Passing `--tsconfig ./tsconfig.json` explicitly opts into replacing another file.

To keep the checker and k6 declaration mappings updated as files change, run:

```bash
go run . typecheck --watch examples/typecheck/remote.js
go run . typecheck --in-place --watch examples/typecheck/remote.js
```

The first form watches a randomly named temporary project. The second keeps `./tsconfig.json` and
`.k6/types/` in the repository root. `--watch` cannot be combined with `--generate-only`.

## Extension declarations from the binary

[`xk6-types/extension.go`](xk6-types/extension.go) embeds its adjacent `index.d.ts`, implements
`modules.TypeScriptTypeProvider`, and registers `k6/x/types-example`. The small
[`k6-with-types`](k6-with-types/main.go) entry point imports that extension into a real k6 binary.

Generate and inspect the project:

```bash
go run ./examples/typecheck/k6-with-types \
  typecheck --generate-only examples/typecheck/extension.js
```

The generated configuration contains an exact mapping similar to:

```json
{
  "compilerOptions": {
    "paths": {
      "k6/x/types-example": [
        "/tmp/k6-types-755373299/types/extensions/k6-x-types-example/index.d.ts"
      ]
    }
  }
}
```

The declaration at that target did not come from the network or source checkout. `k6 typecheck`
asked the registered module in the running binary for it and copied the embedded bytes into the
generated project. Running the command without `--generate-only` invokes the selected checker:

```bash
go run ./examples/typecheck/k6-with-types \
  typecheck examples/typecheck/extension.js
```

The binary is part of type resolution. Running the repository's ordinary `./k6` against
`extension.js` cannot extract this declaration because that binary does not register
`k6/x/types-example`. This applies equally to `typecheck`, `--watch`, and `lsp`: always launch the
custom binary containing the extension whose embedded declaration is needed.

## Use the generated project directly

The command already runs the checker with `--project`. For diagnosis, copy the printed
configuration path into a manual command:

```bash
tsgo --project /tmp/k6-types-755373299/tsconfig.json --traceResolution
```

Running `tsgo remote.js` or `tsc remote.js` does not load the generated project and will report URL
imports as unresolved.

## Run the language-server wrapper

Build the prototype command and install the native language server:

```bash
go build -o ./k6 .
npm install --save-dev @typescript/native-preview @types/k6
```

Configure an LSP client to launch this command from the repository root:

```bash
./k6 lsp --server tsgo examples/typecheck/remote.js
```

This is a stdio protocol process, so running it directly in a terminal waits silently for LSP input.
It is not an interactive shell command. During the session, inspect the generated state from another
terminal:

```bash
find /tmp -maxdepth 2 -path '/tmp/k6-lsp-*/tsconfig.json' -print
find . -maxdepth 1 -name 'tsconfig.k6.*.json' -print
```

The first file is the complete generated project. The second is a small bridge that lets tsgo attach
the open script to that project. Both disappear when the client closes the language server.

To test refresh behavior, open [`remote.js`](remote.js) through that LSP client and temporarily change
the import to another pinned AJV URL. Save the file. Within roughly one second, the `paths` key in the
temporary configuration changes to the new exact import, and the wrapper sends a watched-file
notification to tsgo. Restore the original import when finished.

To retain the complete generated state instead of using `/tmp`, launch:

```bash
./k6 lsp --server tsgo --in-place examples/typecheck/remote.js
```

This keeps `./tsconfig.json` and `.k6/types/` after the LSP exits. No bridge is needed because tsgo
discovers the conventional local filename. The command updates a configuration marked by an earlier
k6 invocation but refuses to overwrite an existing user-owned `tsconfig.json`.

To exercise the classic TypeScript implementation through its LSP adapter, use a directory that does
not already contain `tsconfig.json`:

```bash
npm install --save-dev typescript typescript-language-server @types/k6
./k6 lsp --server tsserver examples/typecheck/remote.js
```

The tsserver backend creates a conventional `tsconfig.json` bridge for the lifetime of the process
because `typescript-language-server` cannot select tsgo's custom configuration filename. It refuses
to overwrite an existing `tsconfig.json`.

## Neovim

[`neovim.lua`](neovim.lua) is a standalone configuration for Neovim 0.11 or newer. It uses the
built-in LSP client and starts one `k6 lsp` process for the selected entry script. No
`nvim-lspconfig` plugin is required. A `FileType` autocommand starts or reuses the process;
`LspAttach` configures each buffer after the k6 client attaches.

Build k6 and install tsgo first:

```bash
go build -o ./k6 .
npm install --save-dev @typescript/native-preview @types/k6
```

Then launch Neovim from the repository root:

```bash
K6_BIN="$PWD/k6" \
nvim -u examples/typecheck/neovim.lua examples/typecheck/remote.js
```

The environment variables are intentionally explicit:

- `K6_BIN` is the absolute k6 executable to launch. It defaults to `<workspace>/k6`.
- `K6_LSP_ENTRY` optionally identifies the entry script whose complete k6 import graph is resolved.
  Without it, the first JavaScript or TypeScript buffer that starts the client becomes the entry.
- `K6_LSP_SERVER` optionally selects `tsgo` or `tsserver`; it defaults to `tsgo`.

Entry selection happens inside the `FileType` callback rather than while `init.lua` is loading. This
also works when Neovim starts with a directory or session and the k6 script is opened later.
`K6_LSP_ENTRY` is useful only when that first buffer is a helper module or a different test should be
the graph root.

The callback sets both `root_dir` and `cmd_cwd` to the nearest Git root. This is important because
`k6 lsp` requires its working directory to be an ancestor of the entry script and searches that
directory's `node_modules/.bin` for tsgo.

Buffer-local behavior is configured in Neovim's
[`LspAttach`](https://neovim.io/doc/user/lsp/#lsp-attach) autocommand. The handler verifies that the
attached client is k6, installs hover/definition/reference/rename mappings, and enables native
autocompletion only when the proxied TypeScript server advertises completion support.

If the configured k6 binary is missing or is not executable, the `FileType` handler emits one
warning and skips only the k6 client. It does not raise a Lua error or interfere with LSP clients for
the same buffer or other file types. The `LspAttach` handler likewise ignores missing, stopped, and
non-k6 clients without asserting.

Inside Neovim, verify the connection with:

```vim
:checkhealth vim.lsp
:lua =vim.lsp.get_clients({ name = "k6" })
:K6LspInfo
```

`K6LspInfo` prints the exact binary, entry script, workspace root, backend, and attached client
count. Check its `binary` field first when an extension import has no types.

Use `K` for hover, `grn` for rename, `grr` for references, and `CTRL-X CTRL-O` for completion. These
are Neovim's built-in LSP mappings and omnifunc. If a separate JavaScript/TypeScript language server
is enabled in the normal Neovim configuration, disable it for this test to avoid duplicate
diagnostics.

To adapt the example into an existing `init.lua`, keep the `FileType` startup and `LspAttach`
configuration autocommands, then replace the environment-variable defaults with the preferred k6
binary and entry-script selection policy.

### Neovim with the typed extension

The extension example needs the extension-enabled binary, not the ordinary `./k6` build. Build it
and use the dedicated [`neovim-extension.lua`](neovim-extension.lua) configuration:

```bash
go build -o ./k6-with-types ./examples/typecheck/k6-with-types
npm install --save-dev @typescript/native-preview @types/k6
nvim -u examples/typecheck/neovim-extension.lua examples/typecheck/extension.js
```

That configuration delegates to `neovim.lua` after setting:

```text
K6_BIN=<workspace>/k6-with-types
K6_LSP_ENTRY=<workspace>/examples/typecheck/extension.js
```

Once attached, `:K6LspInfo` must report `k6-with-types` in the binary path. Hovering over `greet`
should show `(name: string) => string`, and changing `greet("k6")` to `greet(42)` should produce a
TypeScript diagnostic.
