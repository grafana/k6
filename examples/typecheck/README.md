# k6 type discovery examples

These examples exercise the remote and extension type-discovery cases supported by the `k6 lsp`
prototype:

- [`remote.js`](remote.js) imports AJV from esm.sh. The public JavaScript response advertises its
  declaration with `X-TypeScript-Types`.
- [`totp-jsr.js`](totp-jsr.js) imports a TypeScript TOTP implementation directly from JSR, so the
  runtime module is also its type source.
- [`totp-jslib.js`](totp-jslib.js) provides the equivalent example with the existing jslib TOTP
  bundle, which currently does not publish discoverable declarations.
- [`extension.js`](extension.js) imports an extension whose declaration is embedded in the custom
  k6 binary.
- [`icmp.js`](icmp.js) imports the real `k6/x/icmp` runtime through a thin adapter that embeds the
  declaration published by `xk6-icmp`.

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

The k6 binary embeds the declarations for its built-in modules and writes them into each generated
TypeScript project. Users do not need to install `@types/k6`.

The prototype deliberately delegates language features to TypeScript instead of implementing a new
k6 language server. Its preferred backend is tsgo, the native TypeScript language server written in
Go. The classic TypeScript implementation remains supported through the
`typescript-language-server` adapter over tsserver.

This division makes k6 authoring independent of npm when tsgo is installed directly with Go: k6
provides its built-in declarations, extensions provide their declarations, and tsgo provides the
language features.

The implementation is a proof of concept. It prioritizes automatic editor configuration and accurate
type discovery over persistent caching and performance. The checked-in JavaScript-header example uses
esm.sh, but the resolver is not tied to that service: any HTTPS module can advertise a declaration
with the `X-TypeScript-Types` header. esm.sh can also serve packages from JSR and GitHub, which makes
it possible to experiment with repository-owned jslib packages without first changing jslib.k6.io.
Direct `jsr:` import support and decisions about how jslib should be hosted are separate topics.

## Install tsgo

With Go 1.26 or newer, install the preferred native language server directly from
[Microsoft's typescript-go repository](https://github.com/microsoft/typescript-go):

```bash
go install github.com/microsoft/typescript-go/cmd/tsgo@latest
```

Go installs the executable in `GOBIN`, or in `GOPATH/bin` when `GOBIN` is unset. Ensure that
directory is on `PATH` so `k6 lsp` can find `tsgo`.

Alternatively, install the preview tsgo package through npm:

```bash
npm install --save-dev @typescript/native-preview
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
exchanges LSP messages over stdin and stdout. k6 discovers type information, generates a TypeScript
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

## Direct TypeScript from JSR compared with jslib TOTP

The JSR package exports `./src/totp.ts`. The package directory URL ending in `/src` is not itself a
module, so the complete pinned runtime import is:

```javascript
import {
  generateTOTP,
  verifyTOTP,
} from "https://jsr.io/@rabbit-company/totp/1.0.1/src/totp.ts";
```

Unlike a JavaScript response that needs `X-TypeScript-Types`, this response has content type
`text/typescript` and contains its exported interfaces and function signatures inline. `k6 lsp`
caches the source with its `.ts` extension and maps the unchanged URL to it. Generate and inspect the
project with:

```bash
go run . typecheck --generate-only examples/typecheck/totp-jsr.js
```

The generated mapping has this shape:

```json
{
  "compilerOptions": {
    "paths": {
      "https://jsr.io/@rabbit-company/totp/1.0.1/src/totp.ts": [
        "/tmp/k6-types-755373299/types/remotes/jsr.io/@rabbit-company/totp/1.0.1/src/totp.ts"
      ]
    }
  }
}
```

Open [`totp-jsr.js`](totp-jsr.js) through the Neovim setup and hover over `generateTOTP`. Completion
should include the typed `digits`, `algorithm`, `timeStep`, `timestamp`, and `window` options. For a
quick diagnostic, temporarily change `{ digits: 6 }` to `{ algorithm: "MD5" }`; tsgo should reject
it because the package permits only SHA-1, SHA-256, or SHA-512.

[`totp-jslib.js`](totp-jslib.js) performs the same generate-and-verify operation through the current
jslib module:

```javascript
import { TOTP } from "https://jslib.k6.io/totp/1.0.0/index.js";
```

The runtime APIs differ as follows:

| Capability | JSR `@rabbit-company/totp` | `jslib.k6.io/totp` |
| --- | --- | --- |
| API shape | Typed functions and option objects | `TOTP` constructor with `gen()` and `verify()` methods |
| Algorithms | SHA-1, SHA-256, and SHA-512 | SHA-1 |
| Verification window | Configurable; one step on either side by default | Current time step only |
| Secret and URI helpers | `generateTOTPSecret()` and `generateTOTPURI()` | Not provided |
| Type delivery | Inline, versioned TypeScript source | Bundled JavaScript without `X-TypeScript-Types` or `index.d.ts` |

Both examples execute against the same fixed Base32 secret and check that a six-digit code can be
generated and verified. Run them independently:

```bash
go run . run --iterations 1 examples/typecheck/totp-jsr.js
go run . run --iterations 1 examples/typecheck/totp-jslib.js
```

Type discovery intentionally produces different results. The JSR example receives hover,
completion, navigation, and diagnostics directly from the cached `.ts` source. The jslib example
continues to run, but TypeScript reports an unresolved module because that release does not advertise
or publish a declaration:

```bash
go run . typecheck --generate-only examples/typecheck/totp-jslib.js
```

This comparison demonstrates why declarations traveling with the module remove the need for a
separate central `@types` package while leaving the runtime import unchanged.

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

### Real xk6-icmp extension

[`xk6-icmp/extension.go`](xk6-icmp/extension.go) delegates runtime behavior to
[`grafana/xk6-icmp`](https://github.com/grafana/xk6-icmp) v0.3.3 and embeds that release's unchanged
[`index.d.ts`](xk6-icmp/index.d.ts). The adapter only adds `modules.TypeScriptTypeProvider`; the
`ping` and `pingAsync` implementations still come from the real extension.

Both the adapter and [`k6-with-icmp`](k6-with-icmp/main.go) are separate nested Go modules. This
keeps xk6-icmp and its transitive dependencies out of k6's root `go.mod` and `vendor` directory.
Because `k6-with-icmp` is a nested module, build it from its own directory:

```bash
cd examples/typecheck/k6-with-icmp
go build -o ../../../k6-with-icmp .
cd ../../..
```

Do not run `go build ./examples/typecheck/k6-with-icmp` from the repository root; the root module
does not contain packages inside that nested module. Use the resulting binary for type checking or
as the editor's language server:

```bash
./k6-with-icmp typecheck --generate-only examples/typecheck/icmp.js
./k6-with-icmp lsp --server tsgo examples/typecheck/icmp.js
```

The generated project maps `k6/x/icmp` to a declaration extracted from the custom binary:

```json
{
  "compilerOptions": {
    "paths": {
      "k6/x/icmp": [
        "/tmp/k6-lsp-755373299/types/extensions/k6-x-icmp/_devel_/index.d.ts"
      ]
    }
  }
}
```

Type checking and LSP startup do not send ICMP packets. Running [`icmp.js`](icmp.js) does, and the
host may require permission to open ICMP sockets.

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
npm install --save-dev typescript typescript-language-server
./k6 lsp --server tsserver examples/typecheck/remote.js
```

The tsserver backend creates a conventional `tsconfig.json` bridge for the lifetime of the process
because `typescript-language-server` cannot select tsgo's custom configuration filename. It refuses
to overwrite an existing `tsconfig.json`.

## Neovim

[`neovim.lua`](neovim.lua) is a standalone configuration for Neovim 0.11 or newer. It uses the
built-in LSP client and starts one `k6 lsp` process for the selected workspace directory. No
`nvim-lspconfig` plugin is required. A `FileType` autocommand starts or reuses the process;
`LspAttach` configures each buffer after the k6 client attaches.

Build k6 and install tsgo first:

```bash
go build -o ./k6 .
go install github.com/microsoft/typescript-go/cmd/tsgo@latest
```

Then start Neovim from the directory that should become the k6 workspace:

```bash
cd examples/typecheck
K6_BIN="$PWD/../../k6" nvim -u neovim.lua remote.js
```

If the intended k6 binary is already on `PATH`, omit `K6_BIN` as well.

The configuration recognizes three optional environment variables:

- `K6_BIN` selects a specific k6 executable. When unset, the configuration uses `k6` from `PATH`,
  falling back to `<workspace>/k6`.
- `K6_LSP_ROOT` overrides the workspace directory passed to `k6 lsp`. Set it only when the desired
  workspace differs from the directory in which Neovim was started.
- `K6_LSP_SERVER` optionally selects `tsgo` or `tsserver`; it defaults to `tsgo`.

Workspace selection happens inside the `FileType` callback rather than while `init.lua` is loading.
This also works when Neovim starts with a directory or session and a k6 script is opened later. All
JavaScript and TypeScript buffers below the selected directory reuse one client. The k6 wrapper
recursively discovers their import graphs and updates the generated project when scripts are added or
removed.

The callback sets both `root_dir` and `cmd_cwd` to the selected workspace, which is also passed to
`k6 lsp`. k6 searches it and its ancestors for
`node_modules/.bin/tsgo`. Starting Neovim in `examples/typecheck`, as above, avoids scanning every
JavaScript and TypeScript file in the k6 repository. Use `K6_LSP_ROOT` when Neovim must instead be
started from another directory.

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
binary and workspace-directory selection policy.

### Neovim with the typed extension

The extension example needs the extension-enabled binary, not the ordinary `./k6` build. Build it
and select it with `K6_BIN`:

```bash
go build -o ./k6-with-types ./examples/typecheck/k6-with-types
go install github.com/microsoft/typescript-go/cmd/tsgo@latest
cd examples/typecheck
K6_BIN="$PWD/../../k6-with-types" nvim -u neovim.lua extension.js
```

Once attached, hovering over `greet` should show `(name: string) => string`, and changing
`greet("k6")` to `greet(42)` should produce a TypeScript diagnostic.

## Visual Studio Code

VS Code's built-in JavaScript and TypeScript support does not launch arbitrary stdio LSP commands.
The simplest setup is therefore to have k6 generate the TypeScript project in the workspace and let
VS Code's built-in language service load that `tsconfig.json`.

For the ordinary k6 examples, build k6 from the repository root, generate the project inside the
example directory, and open that directory in VS Code:

```bash
go build -o ./k6 .
cd examples/typecheck
../../k6 typecheck --generate-only --in-place remote.js
code .
```

Open [`remote.js`](remote.js), then hover over `Ajv` or use completion on its API. The generated
`tsconfig.json` selects the script and `.k6/types/` contains k6's built-in declarations and any
discovered remote or extension declarations. VS Code notices changes to those files; rerun the
generation command after changing imports. Do not edit the generated configuration because the next
k6 invocation replaces it.

The xk6-icmp example must use the extension-enabled binary. Its build must happen inside the nested
`k6-with-icmp` module:

```bash
cd examples/typecheck/k6-with-icmp
go build -o ../../../k6-with-icmp .
cd ..
../../k6-with-icmp typecheck --generate-only --in-place icmp.js
code .
```

Open [`icmp.js`](icmp.js) and hover over `pingAsync`, request completion inside its options object,
or temporarily use an invalid `preferred_ip_version` value to confirm that VS Code loaded the
declaration extracted from the custom binary. Generating the project does not send ICMP packets.

This setup uses VS Code's built-in TypeScript service rather than the `k6 lsp` process. Connecting
VS Code to `k6 lsp` itself requires a VS Code extension built with the
[`vscode-languageclient`](https://code.visualstudio.com/api/language-extensions/language-server-extension-guide)
library. That client should start the following command with the selected workspace as its working
directory and register JavaScript and TypeScript document selectors:

```text
/absolute/path/to/k6-with-icmp lsp --server tsgo /absolute/path/to/workspace
```

Do not configure VS Code's `typescript.tsdk` or `typescript.tsserver.path` setting to point at k6;
those settings select a TypeScript SDK or tsserver implementation, while `k6 lsp` speaks the standard
Language Server Protocol.

## IntelliJ IDEA and GoLand

The [IntelliJ Platform LSP API](https://plugins.jetbrains.com/docs/intellij/language-server-protocol.html)
supports GoLand and the commercial IntelliJ-based IDEs. That API is intended for IntelliJ plugins;
GoLand does not provide a generic built-in settings page where a user can enter an arbitrary stdio
language-server command. A future dedicated k6 plugin can use the native API to recognize k6 scripts
and launch `k6 lsp` for the project directory.

For testing the prototype without developing a dedicated plugin, install the
[LSP4IJ plugin](https://plugins.jetbrains.com/plugin/23257-lsp4ij). LSP4IJ supports user-defined stdio
language servers and currently requires an IntelliJ-based IDE version 2024.2 or newer. After installing
the plugin and restarting GoLand:

1. Build k6 and install the preferred backend in the project:

   ```bash
   go build -o ./k6 .
   go install github.com/microsoft/typescript-go/cmd/tsgo@latest
   ```

2. Open **Settings | Languages & Frameworks | Language Servers**, select **+**, and create a
   user-defined language server.
3. In the **Server** tab, configure:

   ```text
   Name:    k6
   Command: "$PROJECT_DIR$/k6" lsp --server tsgo "$PROJECT_DIR$/examples/typecheck"
   ```

   Keep system environment variables enabled. LSP4IJ expands `$PROJECT_DIR$` before launch. The input
   directory becomes the k6 LSP workspace regardless of GoLand's process working directory, and k6
   searches that directory and its ancestors for `node_modules/.bin/tsgo` before searching `PATH`.
   In a normal k6 project, pass `"$PROJECT_DIR$"`; the narrower example directory keeps this repository
   demonstration focused.
4. In the **Mappings** tab, associate the JavaScript file type with language ID `javascript`. If the
   local k6 graph contains TypeScript files, also associate the TypeScript file type or the `*.ts`
   filename pattern with language ID `typescript`.
5. Apply the configuration and open a script below the configured directory. Use the
   **Language Services** status bar widget or LSP4IJ's **Language Servers** tool window to confirm that the
   `k6` process is running.

Hover over `generateTOTP` in [`totp-jsr.js`](totp-jsr.js), request completion inside its options
object, or temporarily set `algorithm: "MD5"` to verify hover, completion, and diagnostics. Note that GoLand may
treat a space as an input for completion e.g. `{␣` may fail to resolve completion options. GoLand's
own JavaScript and TypeScript support can also contribute editor results; the LSP console shows the
requests and responses handled specifically by `k6 lsp`.

Directory mode recursively discovers JavaScript and TypeScript files, combines their k6 import graphs,
and watches both file contents and directory membership. New and removed scripts therefore update the
generated TypeScript project without changing the GoLand command. It skips `node_modules`, `.git`,
`.k6`, and declaration files. A script that cannot be loaded as a k6 entry remains in the TypeScript
project while k6 logs a warning and continues processing the other scripts.

File mode remains available when a workspace should expose only one graph:

```text
"$PROJECT_DIR$/k6" lsp --server tsgo "$PROJECT_DIR$/examples/typecheck/totp-jsr.js"
```

For the typed extension example, build the extension-enabled binary and change the executable in the
directory command:

```text
"$PROJECT_DIR$/k6-with-types" lsp --server tsgo "$PROJECT_DIR$/examples/typecheck"
```

LSP4IJ's
[user-defined language server documentation](https://github.com/redhat-developer/lsp4ij/blob/main/docs/UserDefinedLanguageServer.md)
also describes environment variables, project macros, mappings, workspace folders, and protocol
tracing. No LSP initialization JSON is required for `k6 lsp`; the wrapper injects the generated tsgo
configuration after the client sends `initialized`.

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
