package redis

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
)

func TestClientConstructor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, arg, expErr string
	}{
		{
			name: "ok/url/tcp",
			arg:  "'redis://user:pass@localhost:6379/0'",
		},
		{
			name: "ok/url/tls",
			arg:  "'rediss://somesecurehost'",
		},
		{
			name: "ok/object/single",
			arg: `{
				clientName: 'myclient',
				username: 'user',
				password: 'pass',
				socket: {
					host: 'localhost',
					port: 6379,
				}
			}`,
		},
		{
			name: "ok/object/single_tls",
			arg: `{
				socket: {
					host: 'localhost',
					port: 6379,
					tls: {
						ca: ['...'],
					}
				}
			}`,
		},
		{
			name: "ok/object/cluster_urls",
			arg: `{
				cluster: {
					maxRedirects: 3,
					readOnly: true,
					routeByLatency: true,
					routeRandomly: true,
					nodes: ['redis://host1:6379', 'redis://host2:6379']
				}
			}`,
		},
		{
			name: "ok/object/cluster_objects",
			arg: `{
				cluster: {
					nodes: [
						{
							username: 'user',
							password: 'pass',
							socket: {
								host: 'host1',
								port: 6379,
							},
						},
						{
							username: 'user',
							password: 'pass',
							socket: {
								host: 'host2',
								port: 6379,
							},
						}
					]
				}
			}`,
		},
		{
			name: "ok/object/sentinel",
			arg: `{
				username: 'user',
				password: 'pass',
				socket: {
					host: 'localhost',
					port: 6379,
				},
				masterName: 'masterhost',
				sentinelUsername: 'sentineluser',
				sentinelPassword: 'sentinelpass',
			}`,
		},
		{
			name:   "err/empty",
			arg:    "",
			expErr: "must specify one argument",
		},
		{
			name:   "err/url/missing_scheme",
			arg:    "'localhost:6379'",
			expErr: "invalid URL scheme",
		},
		{
			name:   "err/url/invalid_scheme",
			arg:    "'https://localhost:6379'",
			expErr: "invalid options; reason: redis: invalid URL scheme: https",
		},
		{
			name:   "err/object/unknown_field",
			arg:    "{addrs: ['localhost:6379']}",
			expErr: `invalid options; reason: json: unknown field "addrs"`,
		},
		{
			name: "err/object/empty_socket",
			arg: `{
				username: 'user',
				password: 'pass',
			}`,
			expErr: "invalid options; reason: empty socket options",
		},
		{
			name: "err/object/cluster_wrong_type",
			arg: `{
				cluster: {
					nodes: 1,
				}
			}`,
			expErr: `invalid options; reason: cluster nodes property must be an array; got int64`,
		},
		{
			name: "err/object/cluster_wrong_type_internal",
			arg: `{
				cluster: {
					nodes: [1, 2],
				}
			}`,
			expErr: `invalid options; reason: cluster nodes array must contain string or object elements; got int64`,
		},
		{
			name: "err/object/cluster_empty",
			arg: `{
				cluster: {
					nodes: []
				}
			}`,
			expErr: `invalid options; reason: cluster nodes property cannot be empty`,
		},
		{
			name: "err/object/cluster_inconsistent_option",
			arg: `{
				cluster: {
					nodes: [
						{
							username: 'user1',
							password: 'pass',
							socket: {
								host: 'host1',
								port: 6379,
							},
						},
						{
							username: 'user2',
							password: 'pass',
							socket: {
								host: 'host2',
								port: 6379,
							},
						}
					]
				}
			}`,
			expErr: `invalid options; reason: inconsistent username option: user1 != user2`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestSetup(t)
			script := fmt.Sprintf("new Client(%s);", tc.arg)
			gotScriptErr := ts.runtime.EventLoop.Start(func() error {
				_, err := ts.rt.RunString(script)
				return err
			})
			if tc.expErr != "" {
				require.Error(t, gotScriptErr)
				assert.Contains(t, gotScriptErr.Error(), tc.expErr)
			} else {
				assert.NoError(t, gotScriptErr)
			}
		})
	}
}

func TestClientSet(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("SET", func(c *Connection, args []string) {
		if len(args) <= 2 && len(args) > 4 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'GET' command"))
			return
		}

		switch args[0] {
		case "existing_key", "non_existing_key": //nolint:goconst
			c.WriteOK()
		case "expires":
			if len(args) != 4 && args[2] != "EX" && args[3] != "0" {
				c.WriteError(errors.New("ERR unexpected number of arguments for 'SET' command"))
			}
			c.WriteOK()
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.set("existing_key", "new_value")
				.then(res => { if (res !== "OK") { throw 'unexpected value for set result: ' + res } })
				.then(() => redis.set("non_existing_key", "some_value"))
				.then(res => { if (res !== "OK") { throw 'unexpected value for set result: ' + res } })
				.then(() => redis.set("expires", "expired", 10))
				.then(res => { if (res !== "OK") { throw 'unexpected value for set result: ' + res } })
				.then(() => redis.set("unsupported_type", new Array("unsupported")))
				.then(
					res => { throw 'expected to fail setting unsupported type' },
					err => { if (!err.error().startsWith('unsupported type')) { throw 'unexpected error: ' + err } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SET", "existing_key", "new_value"},
		{"SET", "non_existing_key", "some_value"},
		{"SET", "expires", "expired", "ex", "10"},
	}, rs.GotCommands())
}

func TestClientGet(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("GET", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'GET' command"))
			return
		}

		switch args[0] {
		case "existing_key":
			c.WriteBulkString("old_value")
		case "non_existing_key":
			c.WriteNull()
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.get("existing_key")
				.then(res => { if (res !== "old_value") { throw 'unexpected value for get result: ' + res } })
				.then(() => redis.get("non_existing_key"))
				.then(
					res => { throw 'expected to fail getting non-existing key from redis' },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error: ' + err } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"GET", "existing_key"},
		{"GET", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientGetSet(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("GETSET", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'GETSET' command"))
			return
		}

		switch args[0] {
		case "existing_key":
			c.WriteBulkString("old_value")
		case "non_existing_key":
			c.WriteOK()
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.getSet("existing_key", "new_value")
				.then(res => { if (res !== "old_value") { throw 'unexpected value for getSet result: ' + res } })
				.then(() => redis.getSet("non_existing_key", "some_value"))
				.then(res => { if (res !== "OK") { throw 'unexpected value for getSet result: ' + res } })
				.then(() => redis.getSet("unsupported_type", new Array("unsupported")))
				.then(
					res => { throw 'unexpectedly resolve getset unsupported type' },
					err => { if (!err.error().startsWith('unsupported type')) { throw 'unexpected error: ' + err } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"GETSET", "existing_key", "new_value"},
		{"GETSET", "non_existing_key", "some_value"},
	}, rs.GotCommands())
}

func TestClientDel(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("DEL", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'DEL' command"))
			return
		}

		c.WriteInteger(2)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.del("key1", "key2", "nonexisting_key")
				.then(res => { if (res !== 2) { throw 'unexpected value for del result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 1, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"DEL", "key1", "key2", "nonexisting_key"},
	}, rs.GotCommands())
}

func TestClientGetDel(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("GETDEL", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'GETDEL' command"))
			return
		}

		switch args[0] {
		case "existing_key":
			c.WriteBulkString("old_value")
		case "non_existing_key":
			c.WriteNull()
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.getDel("existing_key")
				.then(res => { if (res !== "old_value") { throw 'unexpected value for getDel result: ' + res } })
				.then(() => redis.getDel("non_existing_key"))
				.then(
					res => { if (res !== null) { throw 'unexpected value for getSet result: ' + res } },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error: ' + err } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"GETDEL", "existing_key"},
		{"GETDEL", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientExists(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("EXISTS", func(c *Connection, args []string) {
		if len(args) == 0 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'EXISTS' command"))
			return
		}

		c.WriteInteger(1)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.exists("existing_key", "nonexisting_key")
				.then(res => { if (res !== 1) { throw 'unexpected value for exists result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 1, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"EXISTS", "existing_key", "nonexisting_key"},
	}, rs.GotCommands())
}

func TestClientIncr(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("INCR", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'INCR' command"))
			return
		}

		existingValue := 10

		switch args[0] {
		case "existing_key":
			c.WriteInteger(existingValue + 1)
		case "non_existing_key":
			c.WriteInteger(0 + 1)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.incr("existing_key")
				.then(res => { if (res !== 11) { throw 'unexpected value for existing key incr result: ' + res } })
				.then(() => redis.incr("non_existing_key"))
				.then(res => { if (res !== 1) { throw 'unexpected value for non existing key incr result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"INCR", "existing_key"},
		{"INCR", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientIncrBy(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("INCRBY", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'INCRBY' command"))
			return
		}

		value, err := strconv.Atoi(args[1])
		if err != nil {
			c.WriteError(err)
			return
		}

		existingValue := 10

		switch args[0] {
		case "existing_key":
			c.WriteInteger(existingValue + value)
		case "non_existing_key":
			c.WriteInteger(0 + value)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.incrBy("existing_key", 10)
				.then(res => { if (res !== 20) { throw 'unexpected value for incrBy result: ' + res } })
				.then(() => redis.incrBy("non_existing_key", 10))
				.then(res => { if (res !== 10) { throw 'unexpected value for incrBy result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"INCRBY", "existing_key", "10"},
		{"INCRBY", "non_existing_key", "10"},
	}, rs.GotCommands())
}

func TestClientDecr(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("DECR", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'DECR' command"))
			return
		}

		existingValue := 10

		switch args[0] {
		case "existing_key":
			c.WriteInteger(existingValue - 1)
		case "non_existing_key":
			c.WriteInteger(0 - 1)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.decr("existing_key")
				.then(res => { if (res !== 9) { throw 'unexpected value for decr result: ' + res } })
				.then(() => redis.decr("non_existing_key"))
				.then(res => { if (res !== -1) { throw 'unexpected value for decr result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"DECR", "existing_key"},
		{"DECR", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientDecrBy(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("DECRBY", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'DECRBY' command"))
			return
		}

		value, err := strconv.Atoi(args[1])
		if err != nil {
			c.WriteError(err)
			return
		}

		existingValue := 10

		switch args[0] {
		case "existing_key":
			c.WriteInteger(existingValue - value)
		case "non_existing_key":
			c.WriteInteger(0 - value)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.decrBy("existing_key", 2)
				.then(res => { if (res !== 8) { throw 'unexpected value for decrBy result: ' + res } })
				.then(() => redis.decrBy("non_existing_key", 2))
				.then(res => { if (res !== -2) { throw 'unexpected value for decrBy result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"DECRBY", "existing_key", "2"},
		{"DECRBY", "non_existing_key", "2"},
	}, rs.GotCommands())
}

func TestClientRandomKey(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	calledN := 0
	rs.RegisterCommandHandler("RANDOMKEY", func(c *Connection, args []string) {
		if len(args) != 0 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'RANDOMKEY' command"))
			return
		}

		if calledN == 0 {
			// let's consider the DB empty
			calledN++
			c.WriteNull()
			return
		}

		c.WriteBulkString("random_key")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.randomKey()
				.then(
					res => { throw 'unexpectedly resolved promise for randomKey command: ' + res },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error: ' + err } }
				)
				.then(() => redis.randomKey())
				.then(res => { if (res !== "random_key") { throw 'unexpected value for randomKey result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"RANDOMKEY"},
		{"RANDOMKEY"},
	}, rs.GotCommands())
}

func TestClientMget(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("MGET", func(c *Connection, args []string) {
		if len(args) < 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'MGET' command"))
			return
		}

		c.WriteArray("old_value", "")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.mget("existing_key", "non_existing_key")
				.then(
					res => {
						if (res.length !== 2 || res[0] !== "old_value" || res[1] !== null) {
							throw 'unexpected value for mget result: ' + res
						}
					}
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 1, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"MGET", "existing_key", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientExpire(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("EXPIRE", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'EXPIRE' command"))
			return
		}

		switch args[0] {
		case "expires_key":
			c.WriteInteger(1)
		case "non_existing_key":
			c.WriteInteger(0)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.expire("expires_key", 10)
				.then(res => { if (res !== true) { throw 'unexpected value for expire result: ' + res } })
				.then(() => redis.expire("non_existing_key", 1))
				.then(res => { if (res !== false) { throw 'unexpected value for expire result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"EXPIRE", "expires_key", "10"},
		{"EXPIRE", "non_existing_key", "1"},
	}, rs.GotCommands())
}

func TestClientTTL(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("TTL", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'EXPIRE' command"))
			return
		}

		switch args[0] {
		case "expires_key":
			c.WriteInteger(10)
		case "non_existing_key":
			c.WriteInteger(0)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.ttl("expires_key")
				.then(res => { if (res !== 10) { throw 'unexpected value for expire result: ' + res } })
				.then(() => redis.ttl("non_existing_key"))
				.then(res => { if (res > 0) { throw 'unexpected value for expire result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"TTL", "expires_key"},
		{"TTL", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientPersist(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("PERSIST", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'PERSIST' command"))
			return
		}

		switch args[0] {
		case "expires_key":
			c.WriteInteger(1)
		case "non_existing_key":
			c.WriteInteger(0)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.persist("expires_key")
				.then(res => { if (res !== true) { throw 'unexpected value for expire result: ' + res } })
				.then(() => redis.persist("non_existing_key"))
				.then(res => { if (res !== false) { throw 'unexpected value for expire result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"PERSIST", "expires_key"},
		{"PERSIST", "non_existing_key"},
	}, rs.GotCommands())
}

func TestClientLPush(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("LPUSH", func(c *Connection, args []string) {
		if len(args) < 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LPUSH' command"))
			return
		}

		existingList := []string{"existing_key"}

		switch args[0] {
		case "existing_list": //nolint:goconst
			existingList = append(args[1:], existingList...)
			c.WriteInteger(len(existingList))
		case "new_list":
			c.WriteInteger(1)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.lpush("existing_list", "second", "first")
				.then(res => { if (res !== 3) { throw 'unexpected value for lpush result: ' + res } })
				.then(() => redis.lpush("new_list", 1))
				.then(res => { if (res !== 1) { throw 'unexpected value for lpush result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LPUSH", "existing_list", "second", "first"},
		{"LPUSH", "new_list", "1"},
	}, rs.GotCommands())
}

func TestClientRPush(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("RPUSH", func(c *Connection, args []string) {
		if len(args) < 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'RPUSH' command"))
			return
		}

		existingList := []string{"existing_key"}

		switch args[0] {
		case "existing_list":
			existingList = append(existingList, args[1:]...)
			c.WriteInteger(len(existingList))
		case "new_list":
			c.WriteInteger(1)
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.rpush("existing_list", "second", "third")
				.then(res => { if (res !== 3) { throw 'unexpected value for rpush result: ' + res } })
				.then(() => redis.rpush("new_list", 1))
				.then(res => { if (res !== 1) { throw 'unexpected value for rpush result: ' + res } })
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"RPUSH", "existing_list", "second", "third"},
		{"RPUSH", "new_list", "1"},
	}, rs.GotCommands())
}

func TestClientLPop(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	listState := []string{"first", "second"}
	rs.RegisterCommandHandler("LPOP", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LPOP' command"))
			return
		}

		switch args[0] {
		case "existing_list":
			c.WriteBulkString(listState[0])
			listState = listState[1:]
		case "non_existing_list": //nolint:goconst
			c.WriteNull()
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.lpop("existing_list")
				.then(res => { if (res !== "first") { throw 'unexpected value for lpop first result: ' + res } })
				.then(() => redis.lpop("existing_list"))
				.then(res => { if (res !== "second") { throw 'unexpected value for lpop second result: ' + res } })
				.then(() => redis.lpop("non_existing_list"))
				.then(
					res => { if (res !== null) { throw 'unexpectedly resolved lpop promise: ' + res } },

					// An error is returned if the list does not exist
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for lpop: ' + err.error() } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LPOP", "existing_list"},
		{"LPOP", "existing_list"},
		{"LPOP", "non_existing_list"},
	}, rs.GotCommands())
}

func TestClientRPop(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	listState := []string{"first", "second"}
	rs.RegisterCommandHandler("RPOP", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'RPOP' command"))
			return
		}

		switch args[0] {
		case "existing_list":
			c.WriteBulkString(listState[len(listState)-1])
			listState = listState[:len(listState)-1]
		case "non_existing_list":
			c.WriteNull()
		}
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.rpop("existing_list")
				.then(res => { if (res !== "second") { throw 'unexpected value for rpop result: ' + res }})
				.then(() => redis.rpop("existing_list"))
				.then(res => { if (res !== "first") { throw 'unexpected value for rpop result: ' + res }})
				.then(() => redis.rpop("non_existing_list"))
				.then(
					res => { if (res !== null) { throw 'unexpectedly resolved lpop promise: ' + res } },

					// An error is returned if the list does not exist
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for rpop: ' + err.error() } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"RPOP", "existing_list"},
		{"RPOP", "existing_list"},
		{"RPOP", "non_existing_list"},
	}, rs.GotCommands())
}

func TestClientLRange(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	listState := []string{"first", "second", "third"}
	rs.RegisterCommandHandler("LRANGE", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LRANGE' command"))
			return
		}

		start, err := strconv.Atoi(args[1])
		if err != nil {
			c.WriteError(err)
			return
		}

		stop, err := strconv.Atoi(args[2])
		if err != nil {
			c.WriteError(err)
			return
		}

		if start < 0 {
			start = len(listState) + start
		}

		// This calculation is done in a way that is not 100% correct, but it is
		// good enough for the test.
		c.WriteArray(listState[start : stop+1]...)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.lrange("existing_list", 0, 0)
				.then(res => { if (res.length !== 1 || res[0] !== "first") { throw 'unexpected value for lrange result: ' + res }})
				.then(() => redis.lrange("existing_list", 0, 1))
				.then(res => { if (res.length !== 2 || res[0] !== "first" || res[1] !== "second") { throw 'unexpected value for lrange result: ' + res } })
				.then(() => redis.lrange("existing_list", -2, 2))
				.then(res => {
					if (res.length !== 2 ||
						res[0] !== "second" ||
						res[1] !== "third") {
						throw 'unexpected value for lrange result: ' + res
					}
				})
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LRANGE", "existing_list", "0", "0"},
		{"LRANGE", "existing_list", "0", "1"},
		{"LRANGE", "existing_list", "-2", "2"},
	}, rs.GotCommands())
}

func TestClientLIndex(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	listState := []string{"first", "second", "third"}
	rs.RegisterCommandHandler("LINDEX", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LINDEX' command"))
			return
		}

		if args[0] == "non_existing_list" {
			c.WriteNull()
			return
		}

		index, err := strconv.Atoi(args[1])
		if err != nil {
			c.WriteError(err)
			return
		}

		if index > len(listState)-1 {
			c.WriteNull()
			return
		}

		// This calculation is done in a way that is not 100% correct, but it is
		// good enough for the test.
		c.WriteBulkString(listState[index])
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.lindex("existing_list", 0)
				.then(res => { if (res !== "first") { throw 'unexpected value for lindex result: ' + res } })
				.then(() => redis.lindex("existing_list", 3))
				.then(
					res => { throw 'unexpectedly resolved lindex command promise: ' + res },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for lindex: ' + err.error() } }
				)
				.then(() => redis.lindex("non_existing_list", 0))
				.then(
					res => { throw 'unexpectedly resolved lindex command promise: ' + res },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for lindex: ' + err.error() } }
				)
		`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LINDEX", "existing_list", "0"},
		{"LINDEX", "existing_list", "3"},
		{"LINDEX", "non_existing_list", "0"},
	}, rs.GotCommands())
}

func TestClientClientLSet(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	listState := []string{"first"}
	rs.RegisterCommandHandler("LSET", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LSET' command"))
			return
		}

		if args[0] == "non_existing_list" {
			c.WriteError(errors.New("ERR no such key"))
			return
		}

		index, err := strconv.Atoi(args[1])
		if err != nil {
			c.WriteError(err)
			return
		}

		listState[index] = args[2]
		c.WriteOK()
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.lset("existing_list", 0, "new_first")
				.then(res => { if (res !== "OK") { throw 'unexpected value for lset result: ' + res }})
				.then(() => redis.lset("existing_list", 0, "overridden_value"))
				.then(() => redis.lset("non_existing_list", 0, "new_first"))
				.then(
					res => { if (res !== null) { throw 'unexpectedly resolved promise: ' + res } },
					err => { if (err.error() != 'ERR no such key') { throw 'unexpected error for lset: ' + err.error() } }
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LSET", "existing_list", "0", "new_first"},
		{"LSET", "existing_list", "0", "overridden_value"},
		{"LSET", "non_existing_list", "0", "new_first"},
	}, rs.GotCommands())
}

func TestClientLrem(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("LREM", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LREM' command"))
			return
		}

		if args[0] == "non_existing_list" {
			c.WriteError(errors.New("ERR no such key"))
			return
		}

		c.WriteInteger(1)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.lrem("existing_list", 1, "first")
				.then(() => redis.lrem("existing_list", 0, "second"))
				.then(() => {
					redis.lrem("non_existing_list", 2, "third")
						.then(
							res => { if (res !== null) { throw 'unexpectedly resolved promise: ' + res } },
							err => { if (err.error() != 'ERR no such key') { throw 'unexpected error for lrem: ' + err.error() } },
						)
				})
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LREM", "existing_list", "1", "first"},
		{"LREM", "existing_list", "0", "second"},
		{"LREM", "non_existing_list", "2", "third"},
	}, rs.GotCommands())
}

func TestClientLlen(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("LLEN", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LREM' command"))
			return
		}

		if args[0] == "non_existing_list" {
			c.WriteError(errors.New("ERR no such key"))
			return
		}

		c.WriteInteger(3)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.llen("existing_list")
				.then(res => { if (res !== 3) { throw 'unexpected value for llen result: ' + res } })
				.then(() => {
					redis.llen("non_existing_list")
						.then(
							res => { if (res !== null) { throw 'unexpectedly resolved promise: ' + res } },
							err => { if (err.error() != 'ERR no such key') { throw 'unexpected error for llen: ' + err.error() } }
						)
				})
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"LLEN", "existing_list"},
		{"LLEN", "non_existing_list"},
	}, rs.GotCommands())
}

func TestClientHSet(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HSET", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'LREM' command"))
			return
		}

		if args[0] == "non_existing_hash" { //nolint:goconst
			c.WriteError(errors.New("ERR no such key"))
			return
		}

		c.WriteInteger(1)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hset("existing_hash", "key", "value")
				.then(res => { if (res !== 1) { throw 'unexpected value for hset result: ' + res } })
				.then(() => redis.hset("existing_hash", "fou", "barre"))
				.then(res => { if (res !== 1) { throw 'unexpected value for hset result: ' + res } })
				.then(() => redis.hset("non_existing_hash", "cle", "valeur"))
				.then(
					res => { if (res !== null) { throw 'unexpectedly resolved promise: ' + res } },
					err => { if (err.error() != 'ERR no such key') { throw 'unexpected error for hset: ' + err.error() } },
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HSET", "existing_hash", "key", "value"},
		{"HSET", "existing_hash", "fou", "barre"},
		{"HSET", "non_existing_hash", "cle", "valeur"},
	}, rs.GotCommands())
}

func TestClientHsetnx(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HSETNX", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HSETNX' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteInteger(1) // HSET on a non existing hash creates it
			return
		}

		// key does not exist
		if args[1] == "key" {
			c.WriteInteger(1)
			return
		}

		// key already exists
		c.WriteInteger(0)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hsetnx("existing_hash", "key", "value")
				.then(res => { if (res !== true) { throw 'unexpected value for hsetnx result: ' + res } })
				.then(() => redis.hsetnx("existing_hash", "foo", "barre"))
				.then(res => { if (res !== false) { throw 'unexpected value for hsetnx result: ' + res } })
				.then(() => redis.hsetnx("non_existing_hash", "key", "value"))
				.then(res => { if (res !== true) { throw 'unexpected value for hsetnx result: ' + res } })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HSETNX", "existing_hash", "key", "value"},
		{"HSETNX", "existing_hash", "foo", "barre"},
		{"HSETNX", "non_existing_hash", "key", "value"},
	}, rs.GotCommands())
}

func TestClientHget(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HGET", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HGET' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteNull()
			return
		}

		c.WriteBulkString("bar")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hget("existing_hash", "foo")
				.then(res => { if (res !== "bar") { throw 'unexpected value for hget result: ' + res } })
				.then(() => redis.hget("non_existing_hash", "key"))
				.then(
					res => { throw 'unexpectedly resolved hget promise : ' + res },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for hget: ' + err.error() } },
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HGET", "existing_hash", "foo"},
		{"HGET", "non_existing_hash", "key"},
	}, rs.GotCommands())
}

func TestClientHdel(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HDEL", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HDEL' command"))
			return
		}

		if args[0] == "non_existing_hash" || args[1] == "non_existing_key" {
			c.WriteInteger(0)
			return
		}

		c.WriteInteger(1)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hdel("existing_hash", "foo")
				.then(res => { if (res !== 1) { throw 'unexpected value for hdel result: ' + res } })
				.then(() => redis.hdel("existing_hash", "non_existing_key"))
				.then(res => { if (res !== 0) { throw 'unexpected value for hdel result: ' + res } })
				.then(() => redis.hdel("non_existing_hash", "key"))
				.then(res => { if (res !== 0) { throw 'unexpected value for hdel result: ' + res } })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HDEL", "existing_hash", "foo"},
		{"HDEL", "existing_hash", "non_existing_key"},
		{"HDEL", "non_existing_hash", "key"},
	}, rs.GotCommands())
}

func TestClientHgetall(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HGETALL", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HGETALL' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteArray()
			return
		}

		c.WriteArray("foo", "bar")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hgetall("existing_hash")
				.then(res => { if (typeof res !== "object" || res['foo'] !== 'bar') { throw 'unexpected value for hgetall result: ' + res } })
				.then(() => redis.hgetall("non_existing_hash"))
				.then(
					res => { if (Object.keys(res).length !== 0) { throw 'unexpected value for hgetall result: ' + res} },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for hgetall: ' + err.error() } },
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HGETALL", "existing_hash"},
		{"HGETALL", "non_existing_hash"},
	}, rs.GotCommands())
}

func TestClientHkeys(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HKEYS", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HKEYS' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteArray()
			return
		}

		c.WriteArray("foo")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hkeys("existing_hash")
				.then(res => { if (res.length !== 1 || res[0] !== 'foo') { throw 'unexpected value for hkeys result: ' + res } })
				.then(() => redis.hkeys("non_existing_hash"))
				.then(
					res => { if (res.length !== 0) { throw 'unexpected value for hkeys result: ' + res} },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for hkeys: ' + err.error() } },
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HKEYS", "existing_hash"},
		{"HKEYS", "non_existing_hash"},
	}, rs.GotCommands())
}

func TestClientHvals(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HVALS", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HVALS' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteArray()
			return
		}

		c.WriteArray("bar")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hvals("existing_hash")
				.then(res => { if (res.length !== 1 || res[0] !== 'bar') { throw 'unexpected value for hvals result: ' + res } })
				.then(() => redis.hvals("non_existing_hash"))
				.then(
					res => { if (res.length !== 0) { throw 'unexpected value for hvals result: ' + res} },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for hvals: ' + err.error() } },
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HVALS", "existing_hash"},
		{"HVALS", "non_existing_hash"},
	}, rs.GotCommands())
}

func TestClientHlen(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("HLEN", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HLEN' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteInteger(0)
			return
		}

		c.WriteInteger(1)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hlen("existing_hash")
				.then(res => { if (res !== 1) { throw 'unexpected value for hlen result: ' + res } })
				.then(() => redis.hlen("non_existing_hash"))
				.then(
					res => { if (res !== 0) { throw 'unexpected value for hlen result: ' + res} },
					err => { if (err.error() != 'redis: nil') { throw 'unexpected error for hlen: ' + err.error() } },
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HLEN", "existing_hash"},
		{"HLEN", "non_existing_hash"},
	}, rs.GotCommands())
}

func TestClientHincrby(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	fooHValue := 1
	rs.RegisterCommandHandler("HINCRBY", func(c *Connection, args []string) {
		if len(args) != 3 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'HINCRBY' command"))
			return
		}

		if args[0] == "non_existing_hash" {
			c.WriteInteger(1)
			return
		}

		value, err := strconv.Atoi(args[2])
		if err != nil {
			c.WriteError(err)
			return
		}

		fooHValue += value

		c.WriteInteger(fooHValue)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.hincrby("existing_hash", "foo", 1)
				.then(res => { if (res !== 2) { throw 'unexpected value for hincrby result: ' + res } })
				.then(() => redis.hincrby("existing_hash", "foo", -1))
				.then(res => { if (res !== 1) { throw 'unexpected value for hincrby result: ' + res } })
				.then(() => redis.hincrby("non_existing_hash", "foo", 1))
				.then(res => { if (res !== 1) { throw 'unexpected value for hincrby result: ' + res } })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"HINCRBY", "existing_hash", "foo", "1"},
		{"HINCRBY", "existing_hash", "foo", "-1"},
		{"HINCRBY", "non_existing_hash", "foo", "1"},
	}, rs.GotCommands())
}

func TestClientSadd(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	barWasSet := false
	rs.RegisterCommandHandler("SADD", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SADD' command"))
			return
		}

		if args[0] == "non_existing_set" { //nolint:goconst
			c.WriteInteger(1)
			return
		}

		if barWasSet == false {
			barWasSet = true
			c.WriteInteger(1)
			return
		}

		c.WriteInteger(0)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.sadd("existing_set", "bar")
				.then(res => { if (res !== 1) { throw 'unexpected value for sadd result: ' + res } })
				.then(() => redis.sadd("existing_set", "bar"))
				.then(res => { if (res !== 0) { throw 'unexpected value for sadd result: ' + res } })
				.then(() => redis.sadd("non_existing_set", "foo"))
				.then(res => { if (res !== 1) { throw 'unexpected value for sadd result: ' + res} })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SADD", "existing_set", "bar"},
		{"SADD", "existing_set", "bar"},
		{"SADD", "non_existing_set", "foo"},
	}, rs.GotCommands())
}

func TestClientSrem(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	fooWasRemoved := false
	rs.RegisterCommandHandler("SREM", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SREM' command"))
			return
		}

		if args[0] == "non_existing_set" {
			c.WriteInteger(0)
			return
		}

		if fooWasRemoved == false {
			fooWasRemoved = true
			c.WriteInteger(1)
			return
		}

		c.WriteInteger(0)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.srem("existing_set", "foo")
				.then(res => { if (res !== 1) { throw 'unexpected value for srem result: ' + res } })
				.then(() => redis.srem("existing_set", "foo"))
				.then(res => { if (res !== 0) { throw 'unexpected value for srem result: ' + res } })
				.then(() => redis.srem("existing_set", "doesnotexist"))
				.then(res => { if (res !== 0) { throw 'unexpected value for srem result: ' + res } })
				.then(() => redis.srem("non_existing_set", "foo"))
				.then(res => { if (res !== 0) { throw 'unexpected value for srem result: ' + res} })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 4, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SREM", "existing_set", "foo"},
		{"SREM", "existing_set", "foo"},
		{"SREM", "existing_set", "doesnotexist"},
		{"SREM", "non_existing_set", "foo"},
	}, rs.GotCommands())
}

func TestClientSismember(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("SISMEMBER", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SISMEMBER' command"))
			return
		}

		if args[0] == "non_existing_set" {
			c.WriteInteger(0)
			return
		}

		if args[1] == "foo" {
			c.WriteInteger(1)
			return
		}

		c.WriteInteger(0)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.sismember("existing_set", "foo")
				.then(res => { if (res !== true) { throw 'unexpected value for sismember result: ' + res } })
				.then(() => redis.sismember("existing_set", "bar"))
				.then(res => { if (res !== false) { throw 'unexpected value for sismember result: ' + res } })
				.then(() => redis.sismember("non_existing_set", "foo"))
				.then(res => { if (res !== false) { throw 'unexpected value for sismember result: ' + res} })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 3, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SISMEMBER", "existing_set", "foo"},
		{"SISMEMBER", "existing_set", "bar"},
		{"SISMEMBER", "non_existing_set", "foo"},
	}, rs.GotCommands())
}

func TestClientSmembers(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("SMEMBERS", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SMEMBERS' command"))
			return
		}

		if args[0] == "non_existing_set" {
			c.WriteArray()
			return
		}

		c.WriteArray("foo", "bar")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.smembers("existing_set")
				.then(res => { if (res.length !== 2 || 'foo' in res || 'bar' in res) { throw 'unexpected value for smembers result: ' + res } })
				.then(() => redis.smembers("non_existing_set"))
				.then(res => { if (res.length !== 0) { throw 'unexpected value for smembers result: ' + res} })
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SMEMBERS", "existing_set"},
		{"SMEMBERS", "non_existing_set"},
	}, rs.GotCommands())
}

func TestClientSrandmember(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("SRANDMEMBER", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SRANDMEMBER' command"))
			return
		}

		if args[0] == "non_existing_set" {
			c.WriteError(errors.New("ERR no elements in set"))
			return
		}

		c.WriteBulkString("foo")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.srandmember("existing_set")
				.then(res => { if (res !== 'foo' && res !== 'bar') { throw 'unexpected value for srandmember result: ' + res} })
				.then(() => redis.srandmember("non_existing_set"))
				.then(
					res => { throw 'unexpectedly resolved promise for srandmember result: ' + res },
					err => { if (err.error() !== 'ERR no elements in set') { throw 'unexpected error for srandmember operation: ' + err.error() } }
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SRANDMEMBER", "existing_set"},
		{"SRANDMEMBER", "non_existing_set"},
	}, rs.GotCommands())
}

func TestClientSpop(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	rs.RegisterCommandHandler("SPOP", func(c *Connection, args []string) {
		if len(args) != 1 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SPOP' command"))
			return
		}

		if args[0] == "non_existing_set" {
			c.WriteError(errors.New("ERR no elements in set"))
			return
		}

		c.WriteBulkString("foo")
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.spop("existing_set")
				.then(res => { if (res !== 'foo' && res !== 'bar') { throw 'unexpected value for spop result: ' + res} })
				.then(() => redis.spop("non_existing_set"))
				.then(
					res => { throw 'unexpectedly resolved promise for spop result: ' + res },
					err => { if (err.error() !== 'ERR no elements in set') { throw 'unexpected error for srandmember operation: ' + err.error() } }
				)
			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SPOP", "existing_set"},
		{"SPOP", "non_existing_set"},
	}, rs.GotCommands())
}

func TestClientSendCommand(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	rs := RunT(t)
	fooWasSet := false
	rs.RegisterCommandHandler("SADD", func(c *Connection, args []string) {
		if len(args) != 2 {
			c.WriteError(errors.New("ERR unexpected number of arguments for 'SADD' command"))
			return
		}

		if args[1] == "foo" && !fooWasSet {
			fooWasSet = true
			c.WriteInteger(1)
			return
		}

		c.WriteInteger(0)
	})

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client('redis://%s');

			redis.sendCommand("sadd", "existing_set", "foo")
				.then(res => { if (res !== 1) { throw 'unexpected value for sadd result: ' + res } })
				.then(() => redis.sendCommand("sadd", "existing_set", "foo"))
				.then(res => { if (res !== 0) { throw 'unexpected value for sadd result: ' + res } })

			`, rs.Addr()))

		return err
	})

	assert.NoError(t, gotScriptErr)
	assert.Equal(t, 2, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"SADD", "existing_set", "foo"},
		{"SADD", "existing_set", "foo"},
	}, rs.GotCommands())
}

func TestClientCommandsInInitContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		statement string
	}{
		{
			name:      "set should fail when used in the init context",
			statement: "redis.set('should', 'fail')",
		},
		{
			name:      "get should fail when used in the init context",
			statement: "redis.get('shouldfail')",
		},
		{
			name:      "getSet should fail when used in the init context",
			statement: "redis.getSet('should', 'fail')",
		},
		{
			name:      "del should fail when used in the init context",
			statement: "redis.del('should', 'fail')",
		},
		{
			name:      "getDel should fail when used in the init context",
			statement: "redis.getDel('shouldfail')",
		},
		{
			name:      "exists should fail when used in the init context",
			statement: "redis.exists('should', 'fail')",
		},
		{
			name:      "incr should fail when used in the init context",
			statement: "redis.incr('shouldfail')",
		},
		{
			name:      "incrBy should fail when used in the init context",
			statement: "redis.incrBy('shouldfail', 10)",
		},
		{
			name:      "decr should fail when used in the init context",
			statement: "redis.decr('shouldfail')",
		},
		{
			name:      "decrBy should fail when used in the init context",
			statement: "redis.decrBy('shouldfail', 10)",
		},
		{
			name:      "randomKey should fail when used in the init context",
			statement: "redis.randomKey()",
		},
		{
			name:      "mget should fail when used in the init context",
			statement: "redis.mget('should', 'fail')",
		},
		{
			name:      "expire should fail when used in the init context",
			statement: "redis.expire('shouldfail', 10)",
		},
		{
			name:      "ttl should fail when used in the init context",
			statement: "redis.ttl('shouldfail')",
		},
		{
			name:      "persist should fail when used in the init context",
			statement: "redis.persist('shouldfail')",
		},
		{
			name:      "lpush should fail when used in the init context",
			statement: "redis.lpush('should', 'fail', 'indeed')",
		},
		{
			name:      "rpush should fail when used in the init context",
			statement: "redis.rpush('should', 'fail', 'indeed')",
		},
		{
			name:      "lpop should fail when used in the init context",
			statement: "redis.lpop('shouldfail')",
		},
		{
			name:      "rpop should fail when used in the init context",
			statement: "redis.rpop('shouldfail')",
		},
		{
			name:      "lrange should fail when used in the init context",
			statement: "redis.lrange('shouldfail', 0, 5)",
		},
		{
			name:      "lindex should fail when used in the init context",
			statement: "redis.lindex('shouldfail', 1)",
		},
		{
			name:      "lset should fail when used in the init context",
			statement: "redis.lset('shouldfail', 1, 'fail')",
		},
		{
			name:      "lrem should fail when used in the init context",
			statement: "redis.lrem('should', 1, 'fail')",
		},
		{
			name:      "llen should fail when used in the init context",
			statement: "redis.llen('shouldfail')",
		},
		{
			name:      "hset should fail when used in the init context",
			statement: "redis.hset('shouldfail', 'foo', 'bar')",
		},
		{
			name:      "hsetnx should fail when used in the init context",
			statement: "redis.hsetnx('shouldfail', 'foo', 'bar')",
		},
		{
			name:      "hget should fail when used in the init context",
			statement: "redis.hget('should', 'fail')",
		},
		{
			name:      "hdel should fail when used in the init context",
			statement: "redis.hdel('should', 'fail', 'indeed')",
		},
		{
			name:      "hgetall should fail when used in the init context",
			statement: "redis.hgetall('shouldfail')",
		},
		{
			name:      "hkeys should fail when used in the init context",
			statement: "redis.hkeys('shouldfail')",
		},
		{
			name:      "hvals should fail when used in the init context",
			statement: "redis.hvals('shouldfail')",
		},
		{
			name:      "hlen should fail when used in the init context",
			statement: "redis.hlen('shouldfail')",
		},
		{
			name:      "hincrby should fail when used in the init context",
			statement: "redis.hincrby('should', 'fail', 10)",
		},
		{
			name:      "sadd should fail when used in the init context",
			statement: "redis.sadd('should', 'fail', 'indeed')",
		},
		{
			name:      "srem should fail when used in the init context",
			statement: "redis.srem('should', 'fail', 'indeed')",
		},
		{
			name:      "sismember should fail when used in the init context",
			statement: "redis.sismember('should', 'fail')",
		},
		{
			name:      "smembers should fail when used in the init context",
			statement: "redis.smembers('shouldfail')",
		},
		{
			name:      "srandmember should fail when used in the init context",
			statement: "redis.srandmember('shouldfail')",
		},
		{
			name:      "persist should fail when used in the init context",
			statement: "redis.persist('shouldfail')",
		},
		{
			name:      "spop should fail when used in the init context",
			statement: "redis.spop('shouldfail')",
		},
		{
			name:      "sendCommand should fail when used in the init context",
			statement: "redis.sendCommand('GET', 'shouldfail')",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := newInitContextTestSetup(t)

			gotScriptErr := ts.runtime.EventLoop.Start(func() error {
				_, err := ts.rt.RunString(fmt.Sprintf(`
				const redis = new Client('redis://unreachable:42424');

				%s.then(res => { throw 'expected to fail when called in the init context' })
			`, tc.statement))

				return err
			})

			assert.Error(t, gotScriptErr)
		})
	}
}

func TestClientCommandsAgainstUnreachableServer(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		statement string
	}{
		{
			name:      "set should fail when server is unreachable",
			statement: "redis.set('should', 'fail')",
		},
		{
			name:      "get should fail when server is unreachable",
			statement: "redis.get('shouldfail')",
		},
		{
			name:      "getSet should fail when server is unreachable",
			statement: "redis.getSet('should', 'fail')",
		},
		{
			name:      "del should fail when server is unreachable",
			statement: "redis.del('should', 'fail')",
		},
		{
			name:      "getDel should fail when server is unreachable",
			statement: "redis.getDel('shouldfail')",
		},
		{
			name:      "exists should fail when server is unreachable",
			statement: "redis.exists('should', 'fail')",
		},
		{
			name:      "incr should fail when server is unreachable",
			statement: "redis.incr('shouldfail')",
		},
		{
			name:      "incrBy should fail when server is unreachable",
			statement: "redis.incrBy('shouldfail', 10)",
		},
		{
			name:      "decr should fail when server is unreachable",
			statement: "redis.decr('shouldfail')",
		},
		{
			name:      "decrBy should fail when server is unreachable",
			statement: "redis.decrBy('shouldfail', 10)",
		},
		{
			name:      "randomKey should fail when server is unreachable",
			statement: "redis.randomKey()",
		},
		{
			name:      "mget should fail when server is unreachable",
			statement: "redis.mget('should', 'fail')",
		},
		{
			name:      "expire should fail when server is unreachable",
			statement: "redis.expire('shouldfail', 10)",
		},
		{
			name:      "ttl should fail when server is unreachable",
			statement: "redis.ttl('shouldfail')",
		},
		{
			name:      "persist should fail when server is unreachable",
			statement: "redis.persist('shouldfail')",
		},
		{
			name:      "lpush should fail when server is unreachable",
			statement: "redis.lpush('should', 'fail', 'indeed')",
		},
		{
			name:      "rpush should fail when server is unreachable",
			statement: "redis.rpush('should', 'fail', 'indeed')",
		},
		{
			name:      "lpop should fail when server is unreachable",
			statement: "redis.lpop('shouldfail')",
		},
		{
			name:      "rpop should fail when server is unreachable",
			statement: "redis.rpop('shouldfail')",
		},
		{
			name:      "lrange should fail when server is unreachable",
			statement: "redis.lrange('shouldfail', 0, 5)",
		},
		{
			name:      "lindex should fail when server is unreachable",
			statement: "redis.lindex('shouldfail', 1)",
		},
		{
			name:      "lset should fail when server is unreachable",
			statement: "redis.lset('shouldfail', 1, 'fail')",
		},
		{
			name:      "lrem should fail when server is unreachable",
			statement: "redis.lrem('should', 1, 'fail')",
		},
		{
			name:      "llen should fail when server is unreachable",
			statement: "redis.llen('shouldfail')",
		},
		{
			name:      "hset should fail when server is unreachable",
			statement: "redis.hset('shouldfail', 'foo', 'bar')",
		},
		{
			name:      "hsetnx should fail when server is unreachable",
			statement: "redis.hsetnx('shouldfail', 'foo', 'bar')",
		},
		{
			name:      "hget should fail when server is unreachable",
			statement: "redis.hget('should', 'fail')",
		},
		{
			name:      "hdel should fail when server is unreachable",
			statement: "redis.hdel('should', 'fail', 'indeed')",
		},
		{
			name:      "hgetall should fail when server is unreachable",
			statement: "redis.hgetall('shouldfail')",
		},
		{
			name:      "hkeys should fail when server is unreachable",
			statement: "redis.hkeys('shouldfail')",
		},
		{
			name:      "hvals should fail when server is unreachable",
			statement: "redis.hvals('shouldfail')",
		},
		{
			name:      "hlen should fail when server is unreachable",
			statement: "redis.hlen('shouldfail')",
		},
		{
			name:      "hincrby should fail when server is unreachable",
			statement: "redis.hincrby('should', 'fail', 10)",
		},
		{
			name:      "sadd should fail when server is unreachable",
			statement: "redis.sadd('should', 'fail', 'indeed')",
		},
		{
			name:      "srem should fail when server is unreachable",
			statement: "redis.srem('should', 'fail', 'indeed')",
		},
		{
			name:      "sismember should fail when server is unreachable",
			statement: "redis.sismember('should', 'fail')",
		},
		{
			name:      "smembers should fail when server is unreachable",
			statement: "redis.smembers('shouldfail')",
		},
		{
			name:      "srandmember should fail when server is unreachable",
			statement: "redis.srandmember('shouldfail')",
		},
		{
			name:      "persist should fail when server is unreachable",
			statement: "redis.persist('shouldfail')",
		},
		{
			name:      "spop should fail when server is unreachable",
			statement: "redis.spop('shouldfail')",
		},
		{
			name:      "sendCommand should fail when server is unreachable",
			statement: "redis.sendCommand('GET', 'shouldfail')",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestSetup(t)

			gotScriptErr := ts.runtime.EventLoop.Start(func() error {
				_, err := ts.rt.RunString(fmt.Sprintf(`
				const redis = new Client('redis://unreachable:42424');

				%s.then(res => { throw 'expected to fail when server is unreachable' })
			`, tc.statement))

				return err
			})

			assert.Error(t, gotScriptErr)
		})
	}
}

func TestClientIsSupportedType(t *testing.T) {
	t.Parallel()

	t.Run("table tests", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			offset  int
			args    []any
			wantErr bool
		}{
			{
				name:    "string is a supported type",
				offset:  1,
				args:    []any{"foo"},
				wantErr: false,
			},
			{
				name:    "int is a supported type",
				offset:  1,
				args:    []any{int(123)},
				wantErr: false,
			},
			{
				name:    "int64 is a supported type",
				offset:  1,
				args:    []any{int64(123)},
				wantErr: false,
			},
			{
				name:    "float64 is a supported type",
				offset:  1,
				args:    []any{float64(123)},
				wantErr: false,
			},
			{
				name:    "bool is a supported type",
				offset:  1,
				args:    []any{bool(true)},
				wantErr: false,
			},
			{
				name:    "multiple identical types args are supported",
				offset:  1,
				args:    []any{int(123), int(456)},
				wantErr: false,
			},
			{
				name:    "multiple mixed types args are supported",
				offset:  1,
				args:    []any{int(123), "foo", bool(true)},
				wantErr: false,
			},
			{
				name:    "slice[T] is not a supported type",
				offset:  1,
				args:    []any{[]string{"1", "2", "3"}},
				wantErr: true,
			},
			{
				name:    "multiple mixed valid and invalid types args are not supported",
				offset:  1,
				args:    []any{int(123), []string{"1", "2", "3"}},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := &Client{}
				gotErr := c.isSupportedType(tt.offset, tt.args...)
				assert.Equal(t,
					tt.wantErr,
					gotErr != nil,
					"Client.isSupportedType() error = %v, wantErr %v", gotErr, tt.wantErr,
				)
			})
		}
	})

	t.Run("offset is respected in the error message", func(t *testing.T) {
		t.Parallel()

		c := &Client{}

		gotErr := c.isSupportedType(3, int(123), []string{"1", "2", "3"})

		assert.Error(t, gotErr)
		assert.Contains(t, gotErr.Error(), "argument at index 4")
	})
}

// testSetup is a helper struct holding components
// necessary to test the redis client, in the context
// of the execution of a k6 script.
type testSetup struct {
	runtime *modulestest.Runtime
	rt      *sobek.Runtime
	state   *lib.State
	samples chan metrics.SampleContainer
}

// newTestSetup initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newTestSetup(t testing.TB) testSetup {
	ts := newInitContextTestSetup(t)

	state := &lib.State{
		Dialer: netext.NewDialer(
			net.Dialer{},
			netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4),
		),
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(
				metrics.TagURL,
				metrics.TagProto,
				metrics.TagStatus,
				metrics.TagSubproto,
			),
		},
		Samples:        ts.samples,
		TLSConfig:      &tls.Config{},
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
		Tags:           lib.NewVUStateTags(metrics.NewRegistry().RootTagSet()),
	}
	ts.runtime.MoveToVUContext(state)

	ts.state = state
	return ts
}

// newInitContextTestSetup initializes a new test setup.
// It prepares a test setup with a mocked redis server and a goja runtime,
// and event loop, ready to execute scripts as if being executed in the
// main context of k6.
func newInitContextTestSetup(t testing.TB) testSetup {
	runtime := modulestest.NewRuntime(t)
	samples := make(chan metrics.SampleContainer, 1000)

	rt := runtime.VU.RuntimeField
	m := new(RootModule).NewModuleInstance(runtime.VU)
	require.NoError(t, rt.Set("Client", m.Exports().Named["Client"]))

	return testSetup{
		runtime: runtime,
		rt:      rt,
		samples: samples,
	}
}

func TestClientTLS(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)
	ts.state.TLSConfig.InsecureSkipVerify = true
	rs := RunTSecure(t, nil)

	err := ts.rt.Set("caCert", string(rs.TLSCertificate()))
	require.NoError(t, err)

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client({
				socket: {
					host: '%s',
					port: %d,
					tls: {
						ca: [caCert],
					}
				}
			});

			redis.sendCommand("PING");
		`, rs.Addr().IP.String(), rs.Addr().Port))

		return err
	})

	require.NoError(t, gotScriptErr)
	assert.Equal(t, 1, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"PING"},
	}, rs.GotCommands())
}

func TestClientTLSAuth(t *testing.T) {
	t.Parallel()

	clientCert, clientPKey, err := generateTLSCert()
	require.NoError(t, err)

	ts := newTestSetup(t)
	ts.state.TLSConfig.InsecureSkipVerify = true
	rs := RunTSecure(t, clientCert)

	err = ts.rt.Set("caCert", string(rs.TLSCertificate()))
	require.NoError(t, err)
	err = ts.rt.Set("clientCert", string(clientCert))
	require.NoError(t, err)
	err = ts.rt.Set("clientPKey", string(clientPKey))
	require.NoError(t, err)

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client({
				socket: {
					host: '%s',
					port: %d,
					tls: {
						ca: [caCert],
						cert: clientCert,
						key: clientPKey
					}
				}
			});

			redis.sendCommand("PING");
		`, rs.Addr().IP.String(), rs.Addr().Port))

		return err
	})

	require.NoError(t, gotScriptErr)
	assert.Equal(t, 1, rs.HandledCommandsCount())
	assert.Equal(t, [][]string{
		{"HELLO", "2"},
		{"PING"},
	}, rs.GotCommands())
}

func TestClientTLSRespectsNetworkOPtions(t *testing.T) {
	t.Parallel()

	clientCert, clientPKey, err := generateTLSCert()
	require.NoError(t, err)

	ts := newTestSetup(t)
	rs := RunTSecure(t, clientCert)

	err = ts.rt.Set("caCert", string(rs.TLSCertificate()))
	require.NoError(t, err)
	err = ts.rt.Set("clientCert", string(clientCert))
	require.NoError(t, err)
	err = ts.rt.Set("clientPKey", string(clientPKey))
	require.NoError(t, err)

	// Set the redis server's IP to be blacklisted.
	net, err := lib.ParseCIDR(rs.Addr().IP.String() + "/32")
	require.NoError(t, err)
	ts.state.Dialer.(*netext.Dialer).Blacklist = []*lib.IPNet{net}

	gotScriptErr := ts.runtime.EventLoop.Start(func() error {
		_, err := ts.rt.RunString(fmt.Sprintf(`
			const redis = new Client({
				socket: {
					host: '%s',
					port: %d,
					tls: {
						ca: [caCert],
						cert: clientCert,
						key: clientPKey
					}
				}
			});

			// This operation triggers a connection to the redis
			// server under the hood, and should therefore fail, since
			// the server's IP is blacklisted by k6.
			redis.sendCommand("PING")
		`, rs.Addr().IP.String(), rs.Addr().Port))

		return err
	})

	assert.Error(t, gotScriptErr)
	assert.ErrorContains(t, gotScriptErr, "IP ("+rs.Addr().IP.String()+") is in a blacklisted range")
	assert.Equal(t, 0, rs.HandledCommandsCount())
}
