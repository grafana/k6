// Package broker provides an embedded MQTT broker for testing purposes.
package broker

import (
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// EnvBrokerAddress is the environment variable used to set the MQTT broker address.
// It is used by tests to connect to the embedded broker.
const EnvBrokerAddress = "MQTT_BROKER_ADDRESS"

// New creates a new embedded MQTT broker.
func New(open bool) *mochi.Server {
	log.Print("Creating new embedded MQTT broker")

	broker := mochi.New(&mochi.Options{InlineClient: true})

	var (
		hook mochi.Hook = &auth.AllowHook{}
		opts *auth.Options
	)

	if !open {
		opts = &auth.Options{
			Ledger: &auth.Ledger{
				ACL: auth.ACLRules{auth.ACLRule{Remote: "*"}},
				Auth: auth.AuthRules{
					auth.AuthRule{Username: "test-user", Password: "test-password", Allow: true},
				},
			},
		}

		hook = new(auth.Hook)
	}

	if err := broker.AddHook(hook, opts); err != nil {
		log.Fatal("Failed to add auth hook:", err)
	}

	const brokerHost = "127.0.0.1"

	tcpListener := listeners.NewTCP(listeners.Config{ID: "tcp", Address: brokerHost + ":0"})
	if err := broker.AddListener(tcpListener); err != nil {
		log.Fatal("Failed to add TCP listener:", err)
	}

	go func() {
		log.Print("Starting embedded MQTT broker...")

		err := broker.Serve()
		if err != nil && !errors.Is(err, mochi.ErrConnectionClosed) {
			log.Fatal("Embedded broker server error:", err)
		}
	}()

	// Ensure the listeners are indeed listening before continuing
	var tcpPort string

	for range 10 { // Retry a few times
		if _, port, err := net.SplitHostPort(tcpListener.Address()); err == nil && port != "0" {
			tcpPort = port

			break
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond) //nolint:mnd
	}

	if tcpPort == "" {
		log.Fatal("Failed to get assigned port for embedded broker")
	}

	must(broker.Subscribe("test/#", 0, echoHandler(broker)), "Failed to subscribe to echo topic")

	return broker
}

// Setup initializes the embedded MQTT broker for testing purposes.
func Setup() *mochi.Server {
	log.Print("Setting up embedded MQTT broker for tests")

	broker := New(true)

	tcpListener, ok := broker.Listeners.Get("tcp")
	if !ok {
		log.Fatal("Failed to get TCP listener")
	}

	address := "mqtt://" + tcpListener.Address()

	//nolint:forbidigo // embedded test broker exports its address via env
	must(os.Setenv(EnvBrokerAddress, address), "Failed to set environment variable for MQTT broker address")
	log.Println("MQTT broker address set to", address)

	return broker
}

func must(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func echoHandler(server *mochi.Server) func(cl *mochi.Client, sub packets.Subscription, pk packets.Packet) {
	const suffix = "/echo"

	return func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		if !strings.HasSuffix(pk.TopicName, suffix) {
			_ = server.Publish(pk.TopicName+suffix, pk.Payload, false, 0)
		}
	}
}
