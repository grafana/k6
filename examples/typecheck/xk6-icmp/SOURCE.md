# xk6-icmp example source

`index.d.ts` is copied without modification from
[`grafana/xk6-icmp` v0.3.3](https://github.com/grafana/xk6-icmp/blob/v0.3.3/index.d.ts).

The adapter in `extension.go` delegates runtime behavior to that release's `icmp` package and adds
the `modules.TypeScriptTypeProvider` implementation needed by this prototype. The declaration and
upstream module are licensed under the repository's AGPL-3.0 license.
