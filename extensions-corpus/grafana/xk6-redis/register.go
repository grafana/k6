// Package redis only exists to register the redis extension
package redis

import (
	"github.com/grafana/xk6-redis/redis"
	"go.k6.io/k6/v2/js/modules"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/redis".
func init() {
	modules.Register("k6/x/redis", new(redis.RootModule))
}
