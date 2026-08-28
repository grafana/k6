// Package redis only exists to register the redis extension
package redis

import (
	"github.com/grafana/xk6-redis/redis"
	extensionapi "go.k6.io/k6-extension-api"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/redis".
func init() {
	extensionapi.Register("k6/x/redis", new(redis.RootModule))
}
