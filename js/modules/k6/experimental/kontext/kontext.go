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

// FIXME: we need to centralize the json marshaling in a single place, either here, or at the module level

// ErrKontextKeyNotFound is the error returned when a key is not found in the kontext.
var ErrKontextKeyNotFound = errors.New("key not found")

// ErrKontextWrongType is the error returned when the type of the value in the kontext is not the expected one.
var ErrKontextWrongType = errors.New("wrong type")

const k6ServiceURLEnvironmentVariable = "K6_KONTEXT_SERVICE_URL"

// Getter is the interface encapsulating the action of getting a key from the kontext.
type Getter interface {
	Get(key string) (any, error)
}

// Setter is the interface encapsulating the action of setting a key in the kontext.
type Setter interface {
	Set(key string, value any) error
}

// Sizer is the interface encapsulating the action of getting the size of a key in the kontext.
type Sizer interface {
	Size(key string) (int64, error)
}

// Lister is the interface encapsulating the actions of interacting with a list in the kontext.
type Lister interface {
	Sizer
	LeftPush(key string, value any) (int64, error)
	RightPush(key string, value any) (int64, error)
	RightPop(key string) (any, error)
	LeftPop(key string) (any, error)
}

// Incrementer is the interface encapsulating the actions of adding to an integer in the kontext.
type Incrementer interface {
	Incr(key string) (int64, error)
	Decr(key string) (int64, error)
}

// Kontexter is the interface that all Kontext implementations must implement.
type Kontexter interface {
	Getter
	Setter
	Lister
	Incrementer
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
func (lk *LocalKontext) Get(key string) (any, error) {
	var value any

	// Get the value from the database within a BoltDB transaction
	err := lk.db.handle.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(DefaultLocalKontextBucket))
		if bucket == nil {
			return nil
		}

		jsonValue := bucket.Get([]byte(key))

		if err := json.Unmarshal(jsonValue, &value); err != nil {
			return fmt.Errorf("unmarshalling value failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if value == nil {
		return nil, fmt.Errorf("getting key %s failed: %w", key, ErrKontextKeyNotFound)
	}

	return value, nil
}

// Set sets a value in the local kontext database.
func (lk *LocalKontext) Set(key string, value any) error {
	err := lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshalling value to json failed: %w", err)
		}

		return bucket.Put([]byte(key), encoded)
	})
	if err != nil {
		return fmt.Errorf("setting key %s failed: %w", key, err)
	}

	return nil
}

// LeftPush pushes a value to the left of a list stored in the local kontext database.
func (lk *LocalKontext) LeftPush(key string, value any) (int64, error) {
	updatedCount := int64(0)

	err := lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Get the current value
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			// If the current value is nil, create a new slice with the new value
			newSliceBytes, err := json.Marshal([]any{value})
			if err != nil {
				return fmt.Errorf("marshalling new slice failed: %w", err)
			}

			return bucket.Put([]byte(key), newSliceBytes)
		}

		// Unmarshal the current value as a slice of any
		var currentSlice []any
		if err := json.Unmarshal(currentValue, &currentSlice); err != nil {
			return fmt.Errorf("unmarshalling current value failed: %w", err)
		}

		// Prepare the value(s) to the current slice
		currentSlice = append([]any{value}, currentSlice...)

		// Marshal the current slice
		currentSliceBytes, err := json.Marshal(currentSlice)
		if err != nil {
			return fmt.Errorf("marshalling current slice failed: %w", err)
		}

		return bucket.Put([]byte(key), currentSliceBytes)
	})
	if err != nil {
		return 0, fmt.Errorf("left pushing key %s failed: %w", key, err)
	}

	return updatedCount, nil
}

// RightPush pushes a value to the right of a list stored in the local kontext database.
func (lk *LocalKontext) RightPush(key string, value any) (int64, error) {
	updatedCount := int64(0)

	err := lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Get the current value
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			// If the current value is nil, create a new slice with the new value
			newSliceBytes, err := json.Marshal([]any{value})
			if err != nil {
				return fmt.Errorf("marshalling new slice failed: %w", err)
			}

			return bucket.Put([]byte(key), newSliceBytes)
		}

		// Unmarshal the current value as a slice of any
		var currentSlice []any
		if err := json.Unmarshal(currentValue, &currentSlice); err != nil {
			return fmt.Errorf("unmarshalling current value failed: %w", err)
		}

		// Prepare the value(s) to the current slice
		currentSlice = append(currentSlice, value)

		// Marshal the current slice
		currentSliceBytes, err := json.Marshal(currentSlice)
		if err != nil {
			return fmt.Errorf("marshalling current slice failed: %w", err)
		}

		return bucket.Put([]byte(key), currentSliceBytes)
	})
	if err != nil {
		return 0, fmt.Errorf("left pushing key %s failed: %w", key, err)
	}

	return updatedCount, nil
}

// LeftPop pops the first element from a list stored in the local kontext database.
func (lk *LocalKontext) LeftPop(key string) (any, error) {
	var poppedValue any

	err := lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Get the current value
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			return ErrKontextKeyNotFound
		}

		// Unmarshal the current value as a slice of any
		var currentSlice []interface{}
		if err := json.Unmarshal(currentValue, &currentSlice); err != nil {
			// FIXME: we should probably return a "wrong type" error here
			return fmt.Errorf("unmarshalling current value failed: %w", err)
		}

		// If the list is empty leave the popped value set to nil and return
		if len(currentSlice) == 0 {
			return nil
		}

		// Otherwise pop the last element from the slice
		poppedValue = currentSlice[0]
		currentSlice = currentSlice[1:]

		// Marshal the current slice
		encodedCurrentSlice, err := json.Marshal(currentSlice)
		if err != nil {
			return fmt.Errorf("marshalling current slice failed: %w", err)
		}

		if err := bucket.Put([]byte(key), encodedCurrentSlice); err != nil {
			return fmt.Errorf("putting updated value failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("right popping key %s failed: %w", key, err)
	}

	return poppedValue, nil
}

// RightPop pops the last element from a list stored in the local kontext database.
func (lk *LocalKontext) RightPop(key string) (any, error) {
	var poppedValue any

	err := lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Get the current value
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			return ErrKontextKeyNotFound
		}

		// Unmarshal the current value as a slice of any
		var currentSlice []interface{}
		if err := json.Unmarshal(currentValue, &currentSlice); err != nil {
			// FIXME: we should probably return a "wrong type" error here
			return fmt.Errorf("unmarshalling current value failed: %w", err)
		}

		// If the list is empty leave the popped value set to nil and return
		if len(currentSlice) == 0 {
			return nil
		}

		// Otherwise pop the last element from the slice
		poppedValue = currentSlice[len(currentSlice)-1]
		currentSlice = currentSlice[:len(currentSlice)-1]

		// Marshal the current slice
		encodedCurrentSlice, err := json.Marshal(currentSlice)
		if err != nil {
			return fmt.Errorf("marshalling current slice failed: %w", err)
		}

		if err := bucket.Put([]byte(key), encodedCurrentSlice); err != nil {
			return fmt.Errorf("putting updated value failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("right popping key %s failed: %w", key, err)
	}

	return poppedValue, nil
}

func (lk *LocalKontext) Size(key string) (int64, error) {
	var size int64

	err := lk.db.handle.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Get the current value
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			return ErrKontextKeyNotFound
		}

		// TODO: we currently only support lists, but we could support any other type really.
		// Unmarshal the current value as a slice of any
		var currentSlice []any
		if err := json.Unmarshal(currentValue, &currentSlice); err != nil {
			return fmt.Errorf("unmarshalling value as list failed: %w: %w", ErrKontextWrongType, err)
		}

		size = int64(len(currentSlice))

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("getting size of key %s failed: %w", key, err)
	}

	return size, nil
}

func (lk *LocalKontext) Incr(key string) (int64, error) {
	return lk.incrBy(key, 1)
}

func (lk *LocalKontext) Decr(key string) (int64, error) {
	return lk.incrBy(key, -1)
}

func (lk *LocalKontext) incrBy(key string, n int64) (int64, error) {
	var newN int64

	err := lk.db.handle.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(lk.bucket)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		// Get the current value
		currentValue := bucket.Get([]byte(key))
		if currentValue == nil {
			n = 0
			encoded, err := json.Marshal(&n)
			if err != nil {
				return fmt.Errorf("marshalling value to json failed: %w", err)
			}
			err = bucket.Put([]byte(key), encoded)
			if err != nil {
				return fmt.Errorf("putting updated value failed: %w", err)
			}

			return nil
		}

		var prev int64
		err := json.Unmarshal(currentValue, &prev)
		if err != nil {
			return fmt.Errorf("unmarshalling value to json failed: %w", err)
		}

		prev += n

		encoded, err := json.Marshal(&prev)
		if err != nil {
			return fmt.Errorf("marshalling value to json failed: %w", err)
		}
		err = bucket.Put([]byte(key), encoded)
		if err != nil {
			return fmt.Errorf("putting updated value failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("incrby of key %s failed: %w", key, err)
	}

	return newN, nil
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
func (c CloudKontext) Get(key string) (any, error) {
	ctx := context.Background()
	response, err := c.client.Get(ctx, &proto.GetRequest{Key: key})
	if err != nil {
		return nil, fmt.Errorf("getting key %s from kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Nil {
		return nil, fmt.Errorf("getting key %s from kontext grpc service failed", key)
	}

	var value any
	if err := json.Unmarshal(response.GetData(), &value); err != nil {
		return nil, fmt.Errorf("unmarshalling value failed: %w", err)
	}

	return value, nil
}

// Set sets a value in the Grafana Cloud k6 service.
func (c CloudKontext) Set(key string, value any) error {
	ctx := context.Background()

	encodedValue, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshalling value to json failed: %w", err)
	}

	response, err := c.client.Set(ctx, &proto.SetRequest{Key: key, Data: encodedValue})
	if err != nil {
		return fmt.Errorf("setting key %s in kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Nil {
		return fmt.Errorf("setting key %s in kontext grpc service failed", key)
	}

	return nil
}

// LeftPush pushes a value to the left of a list stored in the Grafana Cloud k6 service.
func (c CloudKontext) LeftPush(key string, value any) (int64, error) {
	ctx := context.Background()

	encoded, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("marshalling value to json failed: %w", err)
	}

	response, err := c.client.Lpush(ctx, &proto.PushRequest{Key: key, Data: encoded})
	if err != nil {
		return 0, fmt.Errorf("left pushing key %s in kontext grpc service failed: %w", key, err)
	}

	return response.Count, nil
}

// RightPush pushes a value to the right of a list stored in the Grafana Cloud k6 service.
func (c CloudKontext) RightPush(key string, value any) (int64, error) {
	ctx := context.Background()

	encoded, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("marshalling value to json failed: %w", err)
	}

	response, err := c.client.Rpush(ctx, &proto.PushRequest{Key: key, Data: encoded})
	if err != nil {
		return 0, fmt.Errorf("left pushing key %s in kontext grpc service failed: %w", key, err)
	}

	return response.Count, nil
}

// LeftPop pops the first element from a list stored in the Grafana Cloud k6 service.
func (c CloudKontext) LeftPop(key string) (any, error) {
	ctx := context.Background()

	response, err := c.client.Lpop(ctx, &proto.PopRequest{Key: key})
	if err != nil {
		return nil, fmt.Errorf("right popping key %s in kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Nil {
		return nil, fmt.Errorf(
			"right popping key %s from kontext grpc service failed; reason: cannot "+
				"rpop from nil value", key)
	}

	if response.GetCode() != proto.StatusCode_WrongType {
		return nil, fmt.Errorf(
			"right popping key %s from kontext grpc service failed; reason: value "+
				"is of wrong type", key)
	}

	var value any
	if err := json.Unmarshal(response.GetData(), &value); err != nil {
		return nil, fmt.Errorf(
			"right popping key %s from kontext grpc service failed; reason: unmarshalling value failed: %w",
			key,
			err,
		)
	}

	return value, nil
}

// RightPop pops the last element from a list stored in the Grafana Cloud k6 service.
func (c CloudKontext) RightPop(key string) (any, error) {
	ctx := context.Background()

	response, err := c.client.Rpop(ctx, &proto.PopRequest{Key: key})
	if err != nil {
		return nil, fmt.Errorf("right popping key %s in kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Nil {
		return nil, fmt.Errorf(
			"right popping key %s from kontext grpc service failed; reason: cannot rpop from nil value",
			key,
		)
	}

	if response.GetCode() != proto.StatusCode_WrongType {
		return nil, fmt.Errorf("right popping key %s from kontext grpc service failed; reason: value is of wrong type", key)
	}

	var value any
	if err := json.Unmarshal(response.GetData(), &value); err != nil {
		return nil, fmt.Errorf(
			"right popping key %s from kontext grpc service failed; reason: unmarshalling value failed: %w",
			key,
			err,
		)
	}

	return value, nil
}

func (c CloudKontext) Size(key string) (int64, error) {
	ctx := context.Background()

	response, err := c.client.Size(ctx, &proto.SizeRequest{Key: key})
	if err != nil {
		return 0, fmt.Errorf("getting size of key %s in kontext grpc service failed: %w", key, err)
	}

	if response.GetCode() != proto.StatusCode_Nil {
		return 0, fmt.Errorf("getting size of key %s in kontext grpc service failed", key)
	}

	return response.Count, nil

}

func (c CloudKontext) Incr(key string) (int64, error) {
	return c.incrBy(key, 1)
}

func (c CloudKontext) Decr(key string) (int64, error) {
	return c.incrBy(key, -1)
}

func (c CloudKontext) incrBy(key string, n int64) (int64, error) {
	ctx := context.Background()

	response, err := c.client.IncrBy(ctx, &proto.IncrByRequest{Key: key, Count: n})

	switch {
	case err != nil:
		return 0, fmt.Errorf("incr key %s in kontext grpc service failed: %w", key, err)
	case response.Code == proto.StatusCode_Ok:
	case response.Code == proto.StatusCode_WrongType:
		return 0, fmt.Errorf("incr key %s from kontext grpc service failed; reason: value is of wrong type", key)
	default:
		return 0, fmt.Errorf("incr key %s in kontext grpc got unexpected status: %v", key, response.Code)
	}

	return response.Count, nil
}
