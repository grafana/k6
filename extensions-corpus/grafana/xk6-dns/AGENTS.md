# xk6-dns

A k6 extension that exposes a DNS resolution API to test scripts. The Go entry point is `register.go`; the implementation lives in `dns/`.

## Release process

Releases are cut from `main` as lightweight tags with an attached GitHub release. Tags follow semantic versioning (`vMAJOR.MINOR.PATCH`). Dependency, security, and CI-only updates are patch bumps; new JavaScript API surface or behavior changes warrant a minor bump.

Follow these steps to strike a release.

1. **Find the last version and the commits since.**
   ```sh
   git tag --sort=-version:refname | head
   git log --oneline <last-tag>..HEAD
   ```

2. **Pick the version.** Inspect the commits and their PR labels (`gh pr view <n> --json title,labels`). If nothing changes runtime behavior or the JS API, it is a patch release. Confirm the chosen version before publishing, since the tag is public and hard to reverse.

3. **Verify the tree.** The working tree must be clean and synced with `origin/main`. Run the quality gate and require all of it to pass:
   ```sh
   git status -sb
   go build ./...
   go vet ./...
   go test ./... -short
   ```

4. **Write the changelog.** Match the format of the previous releases (`gh release view <last-tag>`): a one-line intro, then grouped sections (`## Security`, `## Dependencies`, `## Internal`). Reference each change by its PR number, and end with a compare link:
   ```
   **Full Changelog**: https://github.com/grafana/xk6-dns/compare/<last-tag>...<new-tag>
   ```

5. **Create the tag and release.** Target the current `main` HEAD explicitly and mark it latest:
   ```sh
   gh release create <new-tag> \
     --target $(git rev-parse HEAD) \
     --title <new-tag> \
     --notes-file <notes.md> \
     --latest
   ```

6. **Sync and verify.** Pull the new tag locally and confirm the release:
   ```sh
   git fetch --tags origin
   gh release view <new-tag>
   ```

7. **Create k6-extension-registry PR**. Open a Pull request against `grafana/k6-extension-registry` adding the newly created version to the `registry-v2.yaml` file:
   ```yaml
   # Official extensions
   - module: github.com/grafana/xk6-dns
     description: DNS resolution and lookups in your tests
     tier: official
     imports:
       - k6/x/dns
     versions:
       - v1.1.0
       - v1.1.2
       - vX.Y.Z # new version goes here
   ```
