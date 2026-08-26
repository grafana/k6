package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.k6.io/k6/v2/js/modules"
)

// pingTimeout bounds the connectivity check performed when a Connection is
// constructed.
const pingTimeout = 10 * time.Second

// Connection connects to Kafka for topic administration (create/delete/list).
type Connection struct {
	vu     modules.VU
	client *kgo.Client
}

// openConnection builds a franz-go client from the config and verifies
// connectivity (eager connect). It fails if the cluster is unreachable.
func openConnection(vu modules.VU, cfg ConnectionConfig) (*Connection, error) {
	opts, err := clientOptions([]string{cfg.Address}, cfg.SASL, cfg.TLS)
	if err != nil {
		return nil, err
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Kafka client: %w", err)
	}

	// Ping uses a background context: the VU context may not exist yet, since a
	// Connection is typically constructed in the init context.
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("connecting to %s: %w", cfg.Address, err)
	}

	return &Connection{vu: vu, client: client}, nil
}

// requireVU guards the admin methods: they run in the VU context (the default
// function, or setup/teardown), not in init, and not after close.
func (c *Connection) requireVU(op string) error {
	if c.vu.State() == nil {
		return fmt.Errorf("%s must be called in the VU context (default/setup/teardown function), not in init", op)
	}
	if c.client == nil {
		return errors.New("connection is closed")
	}
	return nil
}

// Close closes the underlying client and releases its connections.
func (c *Connection) Close() {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
}
