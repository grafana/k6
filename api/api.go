package api

import (
	"github.com/loadimpact/speedboat/api/console"
	"github.com/loadimpact/speedboat/api/global"
	"github.com/loadimpact/speedboat/api/http"
	"github.com/loadimpact/speedboat/api/test"
)

func New() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"global":  global.Members,
		"console": console.Members,
		"test":    test.Members,
		"http":    http.New(),
	}
}
