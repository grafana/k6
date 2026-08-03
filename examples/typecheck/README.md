# k6 type discovery examples

These examples exercise both declaration sources supported by the `k6 lsp` prototype:

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

The prototype deliberately delegates language features to TypeScript instead of implementing a new
k6 language server. Its preferred backend is tsgo, the native TypeScript language server written in
Go. The classic TypeScript implementation remains supported through the
`typescript-language-server` adapter over tsserver.

This division can eventually make k6 authoring independent of npm: k6 can provide its built-in and
extension declarations while invoking a separately distributed tsgo binary. The current prototype
does not reach that goal yet because the preview tsgo build and declarations for built-in k6 modules
are installed from npm.

The implementation is a proof of concept. It prioritizes automatic editor configuration and accurate
type discovery over persistent caching and performance. The checked-in example uses esm.sh, but the
resolver is not tied to that service: any HTTPS module can advertise a declaration with the
`X-TypeScript-Types` header. esm.sh can also serve packages from JSR and GitHub, which makes it
possible to experiment with repository-owned jslib packages without first changing jslib.k6.io.
Direct `jsr:` import support and decisions about how jslib should be hosted are separate topics.

## Install tsgo

Install the preferred native language server and the declarations for the built-in k6 modules:

```bash
npm install --save-dev @typescript/native-preview @types/k6
```

The alternative tsserver backend is documented below and requires `typescript-language-server` and
`typescript` instead.

## Start the language server

Build k6, then configure an LSP client to launch `k6 lsp` from the repository root:

```bash
go build -o ./k6 .
./k6 lsp --server tsgo examples/typecheck/remote.js
```

`--server tsgo` makes the preferred backend explicit. The default `--server auto` behavior also
selects a project-local or `PATH`-visible tsgo before considering the tsserver adapter.

`k6 lsp` is a stdio protocol process, not an interactive shell command. The editor starts it and
exchanges LSP messages over stdin and stdout. k6 discovers declarations, generates a TypeScript
project, starts tsgo, and then stays in front of tsgo to refresh the mappings as imports change.

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

When `k6 lsp` starts for this script, its generated `tsconfig.json` contains an exact mapping from
the unchanged esm.sh import to the downloaded AJV declaration:

```json
{
  "compilerOptions": {
    "paths": {
      "https://esm.sh/ajv@6.12.5?bundle": [
        "/tmp/k6-lsp-755373299/types/remotes/esm.sh/ajv@6.12.5-<query-hash>.d.ts"
      ]
    }
  }
}
```

Open the script using the Neovim setup below and hover over `Ajv` or `compile` to confirm that tsgo
loaded the declaration. To test diagnostics, temporarily change the schema to a number:

```javascript
const validate = ajv.compile(42);
```

tsgo should report that `number` is not assignable to AJV's schema parameter. Restore the original
schema, then run the k6 script once to verify runtime loading independently:

```bash
go run . run --iterations 1 examples/typecheck/remote.js
```

No local server, development certificate, or custom trust configuration is required. Both the
runtime JavaScript and declaration use esm.sh's publicly trusted HTTPS endpoint.

## Extension declarations from the binary

[`xk6-types/extension.go`](xk6-types/extension.go) embeds its adjacent `index.d.ts`, implements
`modules.TypeScriptTypeProvider`, and registers `k6/x/types-example`. The small
[`k6-with-types`](k6-with-types/main.go) entry point imports that extension into a real k6 binary.

When launched through the extension-enabled binary, `k6 lsp` extracts the declaration and generates
an exact mapping similar to:

```json
{
  "compilerOptions": {
    "paths": {
      "k6/x/types-example": [
        "/tmp/k6-lsp-755373299/types/extensions/k6-x-types-example/index.d.ts"
      ]
    }
  }
}
```

The declaration at that target did not come from the network or source checkout. `k6 lsp` asked the
registered module in the running binary for it and copied the embedded bytes into the generated
project before starting tsgo.

The binary is part of type resolution. Running the repository's ordinary `./k6` against
`extension.js` cannot extract this declaration because that binary does not register
`k6/x/types-example`. This applies equally to `typecheck`, `--watch`, and `lsp`: always launch the
custom binary containing the extension whose embedded declaration is needed.

## Inspect the generated LSP project

During an editor session, inspect the generated state from another terminal:

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

The configuration recognizes two optional environment variables:

- `K6_BIN` is the absolute k6 executable to launch. It defaults to `<workspace>/k6`.
- `K6_LSP_SERVER` optionally selects `tsgo` or `tsserver`; it defaults to `tsgo`.

Entry selection happens inside the `FileType` callback rather than while `init.lua` is loading. This
also works when Neovim starts with a directory or session and the k6 script is opened later. The first
JavaScript or TypeScript buffer that starts the client becomes the k6 import-graph entry point.

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
```

Use `K` for hover, `gd` for definition, `grn` for rename, `grr` for references, and
`CTRL-X CTRL-O` for completion. If a separate JavaScript/TypeScript language server is enabled in the
normal Neovim configuration, disable it for this test to avoid duplicate diagnostics.

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

That configuration delegates to `neovim.lua` after setting the extension-enabled binary:

```text
K6_BIN=<workspace>/k6-with-types
```

Once attached, hovering over `greet` should show `(name: string) => string`, and changing
`greet("k6")` to `greet(42)` should produce a TypeScript diagnostic.

## Optional: one-shot and watch type checking

`k6 typecheck` uses the same discovery and project-generation code without starting an editor
language server. It is useful for experiments, CI, and inspecting the exact project given to tsgo,
but it is not the primary workflow demonstrated here.

Run discovery and checking once:

```bash
go run . typecheck examples/typecheck/remote.js
```

Generate a project without invoking a checker:

```bash
go run . typecheck --generate-only examples/typecheck/remote.js
```

By default, this retains a project under `/tmp/k6-types-<random>/`. Pass `--in-place` to write
`./tsconfig.json` and `.k6/types/` in the current working directory. The implicit in-place target
refuses to overwrite a user-owned `tsconfig.json`; `--tsconfig` is the explicit opt-in replacement.

For continuous command-line checking:

```bash
go run . typecheck --watch examples/typecheck/remote.js
```

The child tsgo process receives `--project <generated-config> --watch`, while k6 watches the local
import graph and regenerates declaration mappings when imports change. `--watch` cannot be combined
with `--generate-only`.

To inspect resolution manually, use the configuration path printed by `k6 typecheck`:

```bash
tsgo --project /tmp/k6-types-755373299/tsconfig.json --traceResolution
```
