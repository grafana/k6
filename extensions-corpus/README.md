# Extension catalog corpus

This directory embeds source snapshots of the extension catalog used while developing the
standalone extension API. The snapshots intentionally do not include Git history or nested
repositories. They are pinned below so the corpus can be reproduced or refreshed explicitly.

The snapshots were collected on 2026-08-26 from each repository's default branch.

| Repository | Origin | Branch | Commit |
| --- | --- | --- | --- |
| `grafana/xk6-client-prometheus-remote` | `https://github.com/grafana/xk6-client-prometheus-remote.git` | `main` | `a799d8b14ce3972a56e4e3fc75a99cd69e2a8b66` |
| `grafana/xk6-disruptor` | `https://github.com/grafana/xk6-disruptor.git` | `main` | `ee6fcf66a4f1195c68a316e5f800f835406cdcdd` |
| `grafana/xk6-dns` | `https://github.com/grafana/xk6-dns.git` | `main` | `755ed32872d0928eb506ba7908de6136a809ab12` |
| `grafana/xk6-faker` | `https://github.com/grafana/xk6-faker.git` | `master` | `8e921f0714b97dfbea5ba5abe0b416e7664b8676` |
| `grafana/xk6-icmp` | `https://github.com/grafana/xk6-icmp.git` | `main` | `b60b2d4d5741d9c03c0630a44ea57e871cb54316` |
| `grafana/xk6-kafka` | `https://github.com/grafana/xk6-kafka.git` | `main` | `40c5e92f72c48a926899b0f594f4d2f107995ed0` |
| `grafana/xk6-kubernetes` | `https://github.com/grafana/xk6-kubernetes.git` | `main` | `300c383ecb4ebe1e38fd4327ff95aa773c9dc03b` |
| `grafana/xk6-loki` | `https://github.com/grafana/xk6-loki.git` | `main` | `49d8455ced0716633dd3880aa2720c417314eb2c` |
| `grafana/xk6-mqtt` | `https://github.com/grafana/xk6-mqtt.git` | `main` | `c79a9e4348958d04fc31d6b31a9a965118310514` |
| `grafana/xk6-redis` | `https://github.com/grafana/xk6-redis.git` | `master` | `82eb3c1b0714797ea1df00a70374dbd42b1c72e6` |
| `grafana/xk6-sql` | `https://github.com/grafana/xk6-sql.git` | `main` | `9ca42f0b1b25db32c3883805ec71da1175fd2034` |
| `grafana/xk6-sql-driver-azuresql` | `https://github.com/grafana/xk6-sql-driver-azuresql.git` | `main` | `585dfba58e24a40d360c91549cce8b650432dc33` |
| `grafana/xk6-sql-driver-clickhouse` | `https://github.com/grafana/xk6-sql-driver-clickhouse.git` | `main` | `98d4050aa3dbd329538d69199aa9b692fa90da59` |
| `grafana/xk6-sql-driver-mysql` | `https://github.com/grafana/xk6-sql-driver-mysql.git` | `main` | `cbd164c4de29cbc493ac0ab49fcf3d0f7aab6802` |
| `grafana/xk6-sql-driver-postgres` | `https://github.com/grafana/xk6-sql-driver-postgres.git` | `main` | `84bc8e7b299496711674d75cf81433432c7bf73e` |
| `grafana/xk6-sql-driver-sqlserver` | `https://github.com/grafana/xk6-sql-driver-sqlserver.git` | `main` | `1d8905bbe544f6a96dfa3fe02ca87dcf98fb17f6` |
| `grafana/xk6-ssh` | `https://github.com/grafana/xk6-ssh.git` | `main` | `6325bd1fa4d7e8a46cf6a27ee52747f925c836b4` |
| `grafana/xk6-tcp` | `https://github.com/grafana/xk6-tcp.git` | `main` | `3690cb100fb3fd517f5b84759875316fef9850b1` |
| `grafana/xk6-tls` | `https://github.com/grafana/xk6-tls.git` | `main` | `1be64280acdf9a279dae1cd66a17947231d60706` |
| `mostafa/xk6-kafka` | `https://github.com/mostafa/xk6-kafka.git` | `main` | `0c7864ca0bc0e5e7ab4ed8ca4920c3deecf9a44d` |
| `phymbert/xk6-sse` | `https://github.com/phymbert/xk6-sse.git` | `main` | `37cc4724690d8486853de0df7ec325d04df92c5e` |
| `tango-tango/xk6-msgpack` | `https://github.com/tango-tango/xk6-msgpack.git` | `main` | `079a64e0549f15741e28e9fd63fcfddd086da751` |

The v1 catalog also referenced `xk6-client-tracing`. Its repository was unavailable when the
catalog was first analyzed, so it is deliberately recorded as unavailable rather than represented
by an incomplete snapshot.
