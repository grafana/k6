package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/grafana/sobek"
	"github.com/redis/go-redis/v9"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/promises"
	"go.k6.io/k6/lib"
)

// Client represents the Client constructor (i.e. `new redis.Client()`) and
// returns a new Redis client object.
type Client struct {
	vu           modules.VU
	redisOptions *redis.UniversalOptions
	redisClient  redis.UniversalClient
}

// Set the given key with the given value.
//
// If the provided value is not a supported type, the promise is rejected with an error.
//
// The value for `expiration` is interpreted as seconds.
func (c *Client) Set(key string, value interface{}, expiration int) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, value); err != nil {
		reject(err)
		return promise
	}

	go func() {
		result, err := c.redisClient.Set(c.vu.Context(), key, value, time.Duration(expiration)*time.Second).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(result)
	}()

	return promise
}

// Get returns the value for the given key.
//
// If the key does not exist, the promise is rejected with an error.
//
// If the key does not exist, the promise is rejected with an error.
func (c *Client) Get(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.Get(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// GetSet sets the value of key to value and returns the old value stored
//
// If the provided value is not a supported type, the promise is rejected with an error.
func (c *Client) GetSet(key string, value interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, value); err != nil {
		reject(err)
		return promise
	}

	go func() {
		oldValue, err := c.redisClient.GetSet(c.vu.Context(), key, value).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(oldValue)
	}()

	return promise
}

// Del removes the specified keys. A key is ignored if it does not exist
func (c *Client) Del(keys ...string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.Del(c.vu.Context(), keys...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// GetDel gets the value of key and deletes the key.
//
// If the key does not exist, the promise is rejected with an error.
func (c *Client) GetDel(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.GetDel(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Exists returns the number of key arguments that exist.
// Note that if the same existing key is mentioned in the argument
// multiple times, it will be counted multiple times.
func (c *Client) Exists(keys ...string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.Exists(c.vu.Context(), keys...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Incr increments the number stored at `key` by one. If the key does
// not exist, it is set to zero before performing the operation. An
// error is returned if the key contains a value of the wrong type, or
// contains a string that cannot be represented as an integer.
func (c *Client) Incr(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		newValue, err := c.redisClient.Incr(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(newValue)
	}()

	return promise
}

// IncrBy increments the number stored at `key` by `increment`. If the key does
// not exist, it is set to zero before performing the operation. An
// error is returned if the key contains a value of the wrong type, or
// contains a string that cannot be represented as an integer.
func (c *Client) IncrBy(key string, increment int64) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		newValue, err := c.redisClient.IncrBy(c.vu.Context(), key, increment).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(newValue)
	}()

	return promise
}

// Decr decrements the number stored at `key` by one. If the key does
// not exist, it is set to zero before performing the operation. An
// error is returned if the key contains a value of the wrong type, or
// contains a string that cannot be represented as an integer.
func (c *Client) Decr(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		newValue, err := c.redisClient.Decr(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(newValue)
	}()

	return promise
}

// DecrBy decrements the number stored at `key` by `decrement`. If the key does
// not exist, it is set to zero before performing the operation. An
// error is returned if the key contains a value of the wrong type, or
// contains a string that cannot be represented as an integer.
func (c *Client) DecrBy(key string, decrement int64) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		newValue, err := c.redisClient.DecrBy(c.vu.Context(), key, decrement).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(newValue)
	}()

	return promise
}

// RandomKey returns a random key.
//
// If the database is empty, the promise is rejected with an error.
func (c *Client) RandomKey() *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		key, err := c.redisClient.RandomKey(c.vu.Context()).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(key)
	}()

	return promise
}

// Mget returns the values associated with the specified keys.
func (c *Client) Mget(keys ...string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		values, err := c.redisClient.MGet(c.vu.Context(), keys...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(values)
	}()

	return promise
}

// Expire sets a timeout on key, after which the key will automatically
// be deleted.
// Note that calling Expire with a non-positive timeout will result in
// the key being deleted rather than expired.
func (c *Client) Expire(key string, seconds int) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		ok, err := c.redisClient.Expire(c.vu.Context(), key, time.Duration(seconds)*time.Second).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(ok)
	}()

	return promise
}

// Ttl returns the remaining time to live of a key that has a timeout.
//
//nolint:revive
func (c *Client) Ttl(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		duration, err := c.redisClient.TTL(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(duration.Seconds())
	}()

	return promise
}

// Persist removes the existing timeout on key.
func (c *Client) Persist(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		ok, err := c.redisClient.Persist(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(ok)
	}()

	return promise
}

// Lpush inserts all the specified values at the head of the list stored
// at `key`. If `key` does not exist, it is created as empty list before
// performing the push operations. When `key` holds a value that is not
// a list, and error is returned.
func (c *Client) Lpush(key string, values ...interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, values...); err != nil {
		reject(err)
		return promise
	}

	go func() {
		listLength, err := c.redisClient.LPush(c.vu.Context(), key, values...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(listLength)
	}()

	return promise
}

// Rpush inserts all the specified values at the tail of the list stored
// at `key`. If `key` does not exist, it is created as empty list before
// performing the push operations.
func (c *Client) Rpush(key string, values ...interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, values...); err != nil {
		reject(err)
		return promise
	}

	go func() {
		listLength, err := c.redisClient.RPush(c.vu.Context(), key, values...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(listLength)
	}()

	return promise
}

// Lpop removes and returns the first element of the list stored at `key`.
//
// If the list does not exist, this command rejects the promise with an error.
func (c *Client) Lpop(key string) *sobek.Promise {
	// TODO: redis supports indicating the amount of values to pop
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.LPop(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Rpop removes and returns the last element of the list stored at `key`.
//
// If the list does not exist, this command rejects the promise with an error.
func (c *Client) Rpop(key string) *sobek.Promise {
	// TODO: redis supports indicating the amount of values to pop
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.RPop(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Lrange returns the specified elements of the list stored at `key`. The
// offsets start and stop are zero-based indexes. These offsets can be
// negative numbers, where they indicate offsets starting at the end of
// the list.
func (c *Client) Lrange(key string, start, stop int64) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		values, err := c.redisClient.LRange(c.vu.Context(), key, start, stop).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(values)
	}()

	return promise
}

// Lindex returns the specified element of the list stored at `key`.
// The index is zero-based. Negative indices can be used to designate
// elements starting at the tail of the list.
//
// If the list does not exist, this command rejects the promise with an error.
func (c *Client) Lindex(key string, index int64) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.LIndex(c.vu.Context(), key, index).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Lset sets the list element at `index` to `element`.
//
// If the list does not exist, this command rejects the promise with an error.
func (c *Client) Lset(key string, index int64, element string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.LSet(c.vu.Context(), key, index, element).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Lrem removes the first `count` occurrences of `value` from the list stored
// at `key`. If `count` is positive, elements are removed from the beginning of the list.
// If `count` is negative, elements are removed from the end of the list.
// If `count` is zero, all elements matching `value` are removed.
//
// If the list does not exist, this command rejects the promise with an error.
func (c *Client) Lrem(key string, count int64, value string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.LRem(c.vu.Context(), key, count, value).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Llen returns the length of the list stored at `key`. If `key`
// does not exist, it is interpreted as an empty list and 0 is returned.
//
// If the list does not exist, this command rejects the promise with an error.
func (c *Client) Llen(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		length, err := c.redisClient.LLen(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(length)
	}()

	return promise
}

// Hset sets the specified field in the hash stored at `key` to `value`.
// If the `key` does not exist, a new key holding a hash is created.
// If `field` already exists in the hash, it is overwritten.
//
// If the hash does not exist, this command rejects the promise with an error.
func (c *Client) Hset(key string, field string, value interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(2, value); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.HSet(c.vu.Context(), key, field, value).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Hsetnx sets the specified field in the hash stored at `key` to `value`,
// only if `field` does not yet exist. If `key` does not exist, a new key
// holding a hash is created. If `field` already exists, this operation
// has no effect.
func (c *Client) Hsetnx(key, field, value string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		ok, err := c.redisClient.HSetNX(c.vu.Context(), key, field, value).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(ok)
	}()

	return promise
}

// Hget returns the value associated with `field` in the hash stored at `key`.
//
// If the hash does not exist, this command rejects the promise with an error.
func (c *Client) Hget(key, field string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		value, err := c.redisClient.HGet(c.vu.Context(), key, field).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(value)
	}()

	return promise
}

// Hdel deletes the specified fields from the hash stored at `key`.
func (c *Client) Hdel(key string, fields ...string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.HDel(c.vu.Context(), key, fields...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Hgetall returns all fields and values of the hash stored at `key`.
//
// If the hash does not exist, this command rejects the promise with an error.
func (c *Client) Hgetall(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		hashMap, err := c.redisClient.HGetAll(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(hashMap)
	}()

	return promise
}

// Hkeys returns all fields of the hash stored at `key`.
//
// If the hash does not exist, this command rejects the promise with an error.
func (c *Client) Hkeys(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		keys, err := c.redisClient.HKeys(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(keys)
	}()

	return promise
}

// Hvals returns all values of the hash stored at `key`.
//
// If the hash does not exist, this command rejects the promise with an error.
func (c *Client) Hvals(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		values, err := c.redisClient.HVals(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(values)
	}()

	return promise
}

// Hlen returns the number of fields in the hash stored at `key`.
//
// If the hash does not exist, this command rejects the promise with an error.
func (c *Client) Hlen(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.HLen(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Hincrby increments the integer value of `field` in the hash stored at `key`
// by `increment`. If `key` does not exist, a new key holding a hash is created.
// If `field` does not exist the value is set to 0 before the operation is
// set to 0 before the operation is performed.
func (c *Client) Hincrby(key, field string, increment int64) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		newValue, err := c.redisClient.HIncrBy(c.vu.Context(), key, field, increment).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(newValue)
	}()

	return promise
}

// Sadd adds the specified members to the set stored at key.
// Specified members that are already a member of this set are ignored.
// If key does not exist, a new set is created before adding the specified members.
func (c *Client) Sadd(key string, members ...interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, members...); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.SAdd(c.vu.Context(), key, members...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Srem removes the specified members from the set stored at key.
// Specified members that are not a member of this set are ignored.
// If key does not exist, it is treated as an empty set and this command returns 0.
func (c *Client) Srem(key string, members ...interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, members...); err != nil {
		reject(err)
		return promise
	}

	go func() {
		n, err := c.redisClient.SRem(c.vu.Context(), key, members...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(n)
	}()

	return promise
}

// Sismember returns if member is a member of the set stored at key.
func (c *Client) Sismember(key string, member interface{}) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, member); err != nil {
		reject(err)
		return promise
	}

	go func() {
		ok, err := c.redisClient.SIsMember(c.vu.Context(), key, member).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(ok)
	}()

	return promise
}

// Smembers returns all members of the set stored at key.
func (c *Client) Smembers(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		members, err := c.redisClient.SMembers(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(members)
	}()

	return promise
}

// Srandmember returns a random element from the set value stored at key.
//
// If the set does not exist, the promise is rejected with an error.
func (c *Client) Srandmember(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		element, err := c.redisClient.SRandMember(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(element)
	}()

	return promise
}

// Spop removes and returns a random element from the set value stored at key.
//
// If the set does not exist, the promise is rejected with an error.
func (c *Client) Spop(key string) *sobek.Promise {
	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	go func() {
		element, err := c.redisClient.SPop(c.vu.Context(), key).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(element)
	}()

	return promise
}

// SendCommand sends a command to the redis server.
func (c *Client) SendCommand(command string, args ...interface{}) *sobek.Promise {
	var doArgs []interface{}
	doArgs = append(doArgs, command)
	doArgs = append(doArgs, args...)

	promise, resolve, reject := promises.New(c.vu)

	if err := c.connect(); err != nil {
		reject(err)
		return promise
	}

	if err := c.isSupportedType(1, args...); err != nil {
		reject(err)
		return promise
	}

	go func() {
		cmd, err := c.redisClient.Do(c.vu.Context(), doArgs...).Result()
		if err != nil {
			reject(err)
			return
		}

		resolve(cmd)
	}()

	return promise
}

// connect establishes the client's connection to the target
// redis instance(s).
func (c *Client) connect() error {
	// A nil VU state indicates we are in the init context.
	// As a general convention, k6 should not perform IO in the
	// init context. Thus, the Connect method will error if
	// called in the init context.
	vuState := c.vu.State()
	if vuState == nil {
		return common.NewInitContextError("connecting to a redis server in the init context is not supported")
	}

	// If the redisClient is already instantiated, it is safe
	// to assume that the connection is already established.
	if c.redisClient != nil {
		return nil
	}

	tlsCfg := c.redisOptions.TLSConfig
	if tlsCfg != nil && vuState.TLSConfig != nil {
		// Merge k6 TLS configuration with the one we received from the
		// Client constructor. This will need adjusting depending on which
		// options we want to expose in the Redis module, and how we want
		// the override to work.
		tlsCfg.InsecureSkipVerify = vuState.TLSConfig.InsecureSkipVerify
		tlsCfg.CipherSuites = vuState.TLSConfig.CipherSuites
		tlsCfg.MinVersion = vuState.TLSConfig.MinVersion
		tlsCfg.MaxVersion = vuState.TLSConfig.MaxVersion
		tlsCfg.Renegotiation = vuState.TLSConfig.Renegotiation
		tlsCfg.KeyLogWriter = vuState.TLSConfig.KeyLogWriter
		tlsCfg.Certificates = append(tlsCfg.Certificates, vuState.TLSConfig.Certificates...)

		// TODO: Merge vuState.TLSConfig.RootCAs with
		// c.redisOptions.TLSConfig. k6 currently doesn't allow setting
		// this, so it doesn't matter right now, but these should be merged.
		// I couldn't find a way to do this with the x509.CertPool API
		// though...

		// In order to preserve the underlying effects of the [netext.Dialer], such
		// as handling blocked hostnames, or handling hostname resolution, we override
		// the redis client's dialer with our own function which uses the VU's [netext.Dialer]
		// and manually upgrades the connection to TLS.
		//
		// See Pull Request's #17 [discussion] for more details.
		//
		// [discussion]: https://github.com/grafana/xk6-redis/pull/17#discussion_r1369707388
		c.redisOptions.Dialer = c.upgradeDialerToTLS(vuState.Dialer, tlsCfg)
	} else {
		c.redisOptions.Dialer = vuState.Dialer.DialContext
	}

	// Replace the internal redis client instance with a new
	// one using our custom options.
	c.redisClient = redis.NewUniversalClient(c.redisOptions)

	return nil
}

// IsConnected returns true if the client is connected to redis.
func (c *Client) IsConnected() bool {
	return c.redisClient != nil
}

// isSupportedType returns whether the provided arguments are of a type
// supported by the redis client.
//
// Errors will indicate the zero-indexed position of the argument of
// an unsuppoprted type.
//
// isSupportedType should report type errors with arguments in the correct
// position. To be able to accurately report the argument position in the larger
// context of a call to a redis function, the `offset` argument allows to indicate
// the amount of arguments present in front of the ones we provide to `isSupportedType`.
// For instance, when calling `set`, which takes a key, and a value argument,
// isSupportedType applied to the value should eventually report an error with
// the argument in position 1.
func (c *Client) isSupportedType(offset int, args ...interface{}) error {
	for idx, arg := range args {
		switch arg.(type) {
		case string, int, int64, float64, bool:
			continue
		default:
			return fmt.Errorf(
				"unsupported type provided for argument at index %d, "+
					"supported types are string, number, and boolean", idx+offset)
		}
	}

	return nil
}

// DialContextFunc is a function that can be used to dial a connection to a redis server.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// upgradeDialerToTLS returns a DialContextFunc that uses the provided dialer to
// establish a connection, and then upgrades it to TLS using the provided config.
//
// We use this function to make sure the k6 [netext.Dialer], our redis module uses to establish
// the connection and handle network-related options such as blocked hostnames,
// or hostname resolution, but we also want to use the TLS configuration provided
// by the user.
func (c *Client) upgradeDialerToTLS(dialer lib.DialContexter, config *tls.Config) DialContextFunc {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		// Use netext.Dialer to establish the connection
		rawConn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		// Upgrade the connection to TLS if needed
		tlsConn := tls.Client(rawConn, config)
		err = tlsConn.Handshake()
		if err != nil {
			if closeErr := rawConn.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to close connection after TLS handshake error: %w", closeErr)
			}

			return nil, err
		}

		// Overwrite rawConn with the TLS connection
		rawConn = tlsConn

		return rawConn, nil
	}
}
