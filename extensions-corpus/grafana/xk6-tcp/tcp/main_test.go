package tcp

import (
	"os"
	"testing"

	"github.com/grafana/xk6-tcp/internal/echo"
)

func TestMain(m *testing.M) {
	server := echo.Setup()

	code := m.Run()

	server.Stop()

	os.Exit(code) //nolint:forbidigo // standard TestMain exit
}
