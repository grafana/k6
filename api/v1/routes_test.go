package v1

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/api/common"
	"go.k6.io/k6/core"
)

func newRequestWithEngine(engine *core.Engine, method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	return r.WithContext(common.WithEngine(r.Context(), engine))
}

func TestNewHandler(t *testing.T) {
	assert.NotNil(t, NewHandler())
}
