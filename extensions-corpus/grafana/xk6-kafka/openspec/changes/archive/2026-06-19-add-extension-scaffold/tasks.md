## 1. Go module

- [x] 1.1 Add `go.mod` for `github.com/grafana/xk6-kafka` with a supported Go version
- [x] 1.2 Depend on k6 v2 (`go.k6.io/k6` v2) and run `xk6 sync` to align versions with the target k6
- [x] 1.3 Create the `pkg/kafka` package

## 2. Module registration

- [x] 2.1 Implement the root module and register it as `k6/x/kafka` via `modules.Register`
- [x] 2.2 Expose the module's members as the default export

## 3. Constants

- [x] 3.1 Define all flat constants (codecs, SASL, TLS, balancers, group balancers, schema types, element types, subject strategies, isolation levels, start offsets + `FIRST_OFFSET`/`LAST_OFFSET`, `TIME`) with values matching `index.d.ts`
- [x] 3.2 Attach the constants to the module export as flat top-level values (no enum objects)
- [x] 3.3 Add a test asserting constant names and values match `index.d.ts`

## 4. Public symbols

- [x] 4.1 Register `Writer`, `Reader`, `Connection`, `SchemaRegistry` constructors that construct without error (no method behavior yet)
- [x] 4.2 Register the `LoadJKS` function symbol
- [x] 4.3 Add a test that imports the module and constructs each symbol

## 5. Build & CI

- [x] 5.1 Verify `xk6 build --with github.com/grafana/xk6-kafka` works with `CGO_ENABLED=0`
- [x] 5.2 Add the CI caller workflow from `grafana/k6-ci` `templates/k6-ci.yml`, pinning the `uses:` ref to a specific commit SHA and setting `k6-ci-ref` to the same SHA
- [x] 5.3 Set inputs `skip-tests: false` and `skip-extension-testing: false` so the shared workflow runs lint, the multi-version Go tests, and `xk6` extension build/lint/test
- [x] 5.4 Confirm `golangci-lint` (k6-ci config) and `xk6 lint` pass

## 6. Makefile

- [x] 6.1 Add the `grafana/k6-ci` template `Makefile` with the `lint`, `update-lint-patch`, and `clean-lint` targets (deriving the pinned ref from the CI workflow's `uses:` line; no duplicated hash)
- [x] 6.2 Gitignore the generated `.golangci-base.yml` and `.golangci.yml`
- [x] 6.3 Add convenience targets `build` (`xk6 build`), `test` (unit tests), and `it` (integration via `xk6 test`) without altering the lint mechanism
- [x] 6.4 Make the default target (running `make` with no target) print usage/help listing the available targets

## 7. Validate

- [x] 7.1 Run `openspec validate add-extension-scaffold --strict` and fix any issues
