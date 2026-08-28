# Extension API migration agent prompt

Use this prompt for a focused migration of one xk6 extension to the standalone
extension API.

```text
You are migrating <EXTENSION> in .catalog-extensions/<OWNER>/<REPOSITORY> to
go.k6.io/k6-extension-api. Work only in that cloned extension, unless the task
explicitly asks you to add a small, broadly reusable API compatibility helper.

The standalone API source is ../../extension-api relative to each catalog
clone. Its public surface is intentionally small and must not import k6. Read
extension-api/api.go and docs/design/extension-api.md before changing code.

Rules:
1. Replace imports of go.k6.io/k6/js/modules (or /v2 equivalents) with
   go.k6.io/k6-extension-api. Register with extensionapi.Register(), implement
   extensionapi.Module / Instance as applicable, and return
   extensionapi.Exports.
2. Do not retain a k6 module dependency. Update go.mod and go.sum using the
   local replace `go.k6.io/k6-extension-api => ../../../extension-api`.
3. Do not add k6 dependencies to the standalone API. Only add a capability if
   it is explicitly requested and can be implemented with the Go standard
   library and Sobek.
4. Preserve the extension's existing JavaScript API and init/VU semantics.
5. Run gofmt and the extension's focused Go tests or build. Do not commit.
6. Report exact files changed, test/build commands and results, and any
   remaining blocker. Do not modify unrelated catalog clones.
```

For extensions that previously use `js/common.Throw`, prefer the temporary
`extensionapi/common.Throw(runtime, err)` compatibility helper. It must create
a JavaScript exception using Sobek and must not expose k6 types.
