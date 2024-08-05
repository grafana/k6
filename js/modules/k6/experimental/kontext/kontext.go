package kontext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"

	"go.k6.io/k6/js/modules/k6/experimental/kontext/proto"

	bolt "go.etcd.io/bbolt"
	"go.k6.io/k6/js/modules"
)

// KontextKeyNotFoundError is the error returned when a key is not found in the kontext.
var KontextKeyNotFoundError = errors.New("key not found")

const k6ServiceURLEnvironmentVariable = "K6_KONTEXT_SERVICE_URL"

// Getter is the interface encapsulating the action of getting a key from the kontext.
type Getter interface {
	Get(key string) ([]byte, error)
}

// Setter is the interface encapsulating the action of setting a key in the kontext.
type Setter interface {
	Set(key string, value []byte) error
}

// Kontexter is the interface that all Kontext implementations must implement.
type Kontexter interface {
	Getter
	Setter
}

// LocalKontext is a Kontext implementation that uses a local BoltDB database to store key-value pairs.
type LocalKontext struct {
	// vu is the VU instance that this kontext instance belongs to
	vu modules.VU

	// db is the BoltDB instance that is local kontext instance uses.
	db *db

	// bucket is the name of the BoltDB bucket that this KV instance uses.
	bucket []byte
}

// Ensure that LocalKontext implements the Kontexter interface.
var _ Kontexter = &LocalKontext{}

// NewLocalKontext creates a new LocalKontext instance.
func NewLocalKontext(vu modules.VU, db *db) (*LocalKontext, error) {
	// FIXME: we probably need to close at some point?
	if err := db.open(); err != nil {
		return nil, fmt.Errorf("opening local kontext database failed: %w", err)
	}

	return &LocalKontext{vu: vu, db: db, bucket: []byte(DefaultLocalKontextBucket)}, nil
}

// Get retrieves a value from the local kontext database.
func (lk *LocalKontext) Get(key string) ([]byte, error) {
	var jsonValue []byte

	// Get the value from the database within a BoltDB transaction
	err := lk.db.handle.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(DefaultLocalKontextBucket))
		if bucket == nil {
			return nil
		}

		jsonValue = bucket.Get([]byte(key))

		return nil
	})
	if err != nil {
		return nil, err
	}

	if jsonValue == nil {
		return nil, fmt.Errorf("getting key %s failed: %w", key, KontextKeyNotFoundError)
	}

	return jsonValue, nil
}

// Set sets a value in the local kontext database.
func (lk *LocalKontext) Set(key string, value []byte) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("setting key %s failed: %w", key, err)
	}

	err = lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		return bucket.Put([]byte(key), jsonValue)
	})
	if err != nil {
		return fmt.Errorf("setting key %s failed: %w", key, err)
	}

	return nil
}

// CloudKontext is a Kontext implementation that uses the Grafana Cloud k6 service to store key-value pairs.
type CloudKontext struct {
	// vu is the VU instance that this kontext instance belongs to
	vu modules.VU

	// client holds the gRPC client that communicates with the Grafana Cloud k6 service.
	client proto.KontextKVClient
}

// Ensure that CloudKontext implements the Kontexter interface.
var _ Kontexter = &CloudKontext{}

// NewCloudKontext creates a new CloudKontext instance.
func NewCloudKontext(vu modules.VU, serviceURL string) (*CloudKontext, error) {
	// create a gRPC connection to the server
	conn, err := grpc.NewClient(serviceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC server: %w", err)
	}

	client := proto.NewKontextKVClient(conn)

	return &CloudKontext{vu: vu, client: client}, nil
}

// Get retrieves a value from the Grafana Cloud k6 service.
func (c CloudKontext) Get(key string) ([]byte, error) {
	ctx := context.Background()
	response, err := c.client.Get(ctx, &proto.GetRequest{Key: key})
	if err != nil {
		return nil, fmt.Errorf("getting key %s from kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Ok {
		return nil, fmt.Errorf("getting key %s from kontext grpc service failed", key)
	}

	return response.GetData(), nil
}

// Set sets a value in the Grafana Cloud k6 service.
func (c CloudKontext) Set(key string, value []byte) error {
	ctx := context.Background()

	response, err := c.client.Set(ctx, &proto.SetRequest{Key: key, Data: value})
	if err != nil {
		return fmt.Errorf("setting key %s in kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Ok {
		return fmt.Errorf("setting key %s in kontext grpc service failed", key)
	}

	return nil
}
