package api

import (
	"github.com/loadimpact/speedboat/api/console"
	"github.com/loadimpact/speedboat/api/global"
	"github.com/loadimpact/speedboat/api/http"
)

func New() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"global":  global.Members,
		"console": console.Members,
		"http":    http.New(),
	}
}
