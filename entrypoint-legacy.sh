#!/bin/sh

cat >&2 <<-EOF
+------------------------------------------------------------------------------+
| WARNING: The loadimpact/k6 Docker image has been replaced by grafana/k6.     |
|          THIS IMAGE IS DEPRECATED and its support will be discontinued after |
|          Dec 31, 2023. Please update your scripts to use grafana/k6 to       |
|          continue using the latest version of k6.                            |
+------------------------------------------------------------------------------+

EOF

/usr/bin/k6 "$@"
