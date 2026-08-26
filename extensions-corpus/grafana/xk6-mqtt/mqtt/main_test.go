package mqtt

import (
	"log"
	"os"
	"testing"

	"github.com/grafana/xk6-mqtt/internal/broker"
)

func TestMain(m *testing.M) {
	server := broker.Setup()

	code := m.Run()

	log.Print("Shutting down embedded MQTT broker...")

	if err := server.Close(); err != nil {
		log.Fatalf("Error shutting down embedded broker: %v", err)
	}

	os.Exit(code) //nolint:forbidigo // standard TestMain exit
}
