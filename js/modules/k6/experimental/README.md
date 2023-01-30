# Experimental Modules

This folder are here as a documentation and reference point for k6's experimental modules. 

Although [accessible in k6 scripts](../../../initcontext.go) under the `k6/experimental` import path, those modules implementations live in their own repository and are not part of the k6 stable release yet:
* [`k6/experimental/k6-redis`](https://github.com/grafana/xk6-redis)
* [`k6/experimental/k6-websockets`](https://github.com/grafana/xk6-websockets)
* [`k6/experimental/k6-timers`](https://github.com/grafana/xk6-timers)
* [`k6/experimental/k6-browser`](https://github.com/grafana/xk6-browser)

While we intend to keep these modules as stable as possible, we may need to add features or introduce breaking changes. This could happen at any time until we release the module as stable. **use them at your own risk**.

## Upgrading

Experimental modules are based on xk6-extensions, and they introduce a cycle dependency between k6 and the extension. When upgrading an extension's version, it's required to run the following steps:

1. Get the feature branch ready to be merged. Note: from the next step rebasing of the feature branch should be denied; otherwise, the commit will be lost and the relative dependencies will be broken.
2. Make a commit in the feature branch that removes the extension package (and any other problematic experimental modules).
3. Make a PR in the extension's repository and merge it in its main branch that updates the k6 dependency to the commit generated at the previous point.
4. Tag the newly merged commit creating a new version.
5. Make another commit in the feature branch on k6 repository that re-adds this latest version of the extension.
6. Merge the whole feature branch in k6 master with a `Merge` commit (`Create a merge commit` button on GitHub). It will guarantee that the commit used by the extension is preserved.
