package redis

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type singleNodeOptions struct {
	Socket          *socketOptions `json:"socket,omitempty"`
	Username        string         `json:"username,omitempty"`
	Password        string         `json:"password,omitempty"`
	ClientName      string         `json:"clientName,omitempty"`
	Database        int            `json:"database,omitempty"`
	MaxRetries      int            `json:"maxRetries,omitempty"`
	MinRetryBackoff int64          `json:"minRetryBackoff,omitempty"`
	MaxRetryBackoff int64          `json:"maxRetryBackoff,omitempty"`
}

func (opts singleNodeOptions) toRedisOptions() (*redis.Options, error) {
	ropts := &redis.Options{}
	sopts := opts.Socket
	if err := setSocketOptions(ropts, sopts); err != nil {
		return nil, err
	}

	ropts.DB = opts.Database
	ropts.Username = opts.Username
	ropts.Password = opts.Password
	ropts.MaxRetries = opts.MaxRetries
	ropts.MinRetryBackoff = time.Duration(opts.MinRetryBackoff)
	ropts.MaxRetryBackoff = time.Duration(opts.MaxRetryBackoff)

	return ropts, nil
}

type socketOptions struct {
	Host               string      `json:"host,omitempty"`
	Port               int         `json:"port,omitempty"`
	TLS                *tlsOptions `json:"tls,omitempty"`
	DialTimeout        int64       `json:"dialTimeout,omitempty"`
	ReadTimeout        int64       `json:"readTimeout,omitempty"`
	WriteTimeout       int64       `json:"writeTimeout,omitempty"`
	PoolSize           int         `json:"poolSize,omitempty"`
	MinIdleConns       int         `json:"minIdleConns,omitempty"`
	MaxConnAge         int64       `json:"maxConnAge,omitempty"`
	PoolTimeout        int64       `json:"poolTimeout,omitempty"`
	IdleTimeout        int64       `json:"idleTimeout,omitempty"`
	IdleCheckFrequency int64       `json:"idleCheckFrequency,omitempty"`
}

type tlsOptions struct {
	// TODO: Handle binary data (ArrayBuffer) for all these as well.
	CA   []string `json:"ca,omitempty"`
	Cert string   `json:"cert,omitempty"`
	Key  string   `json:"key,omitempty"`
}

type commonClusterOptions struct {
	MaxRedirects   int  `json:"maxRedirects,omitempty"`
	ReadOnly       bool `json:"readOnly,omitempty"`
	RouteByLatency bool `json:"routeByLatency,omitempty"`
	RouteRandomly  bool `json:"routeRandomly,omitempty"`
}

type clusterNodesMapOptions struct {
	commonClusterOptions
	Nodes []*singleNodeOptions `json:"nodes,omitempty"`
}

type clusterNodesStringOptions struct {
	commonClusterOptions
	Nodes []string `json:"nodes,omitempty"`
}

type sentinelOptions struct {
	singleNodeOptions
	MasterName       string `json:"masterName,omitempty"`
	SentinelUsername string `json:"sentinelUsername,omitempty"`
	SentinelPassword string `json:"sentinelPassword,omitempty"`
}

// newOptionsFromObject validates and instantiates an options struct from its
// map representation as exported from sobek.Runtime.
func newOptionsFromObject(obj map[string]interface{}) (*redis.UniversalOptions, error) {
	var options interface{}
	if cluster, ok := obj["cluster"].(map[string]interface{}); ok {
		obj = cluster
		nodes, ok := cluster["nodes"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("cluster nodes property must be an array; got %T", cluster["nodes"])
		}
		if len(nodes) == 0 {
			return nil, errors.New("cluster nodes property cannot be empty")
		}
		switch nodes[0].(type) {
		case map[string]interface{}:
			options = &clusterNodesMapOptions{}
		case string:
			options = &clusterNodesStringOptions{}
		default:
			return nil, fmt.Errorf("cluster nodes array must contain string or object elements; got %T", nodes[0])
		}
	} else if _, ok := obj["masterName"]; ok {
		options = &sentinelOptions{}
	} else {
		options = &singleNodeOptions{}
	}

	jsonStr, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize options to JSON %w", err)
	}

	// Instantiate a JSON decoder which will error on unknown
	// fields. As a result, if the input map contains an unknown
	// option, this function will produce an error.
	decoder := json.NewDecoder(bytes.NewReader(jsonStr))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&options)
	if err != nil {
		return nil, err
	}

	return toUniversalOptions(options)
}

// newOptionsFromString parses the expected URL into redis.UniversalOptions.
func newOptionsFromString(url string) (*redis.UniversalOptions, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	return toUniversalOptions(opts)
}

func readOptions(options interface{}) (*redis.UniversalOptions, error) {
	var (
		opts *redis.UniversalOptions
		err  error
	)
	switch val := options.(type) {
	case string:
		opts, err = newOptionsFromString(val)
	case map[string]interface{}:
		opts, err = newOptionsFromObject(val)
	default:
		return nil, fmt.Errorf("invalid options type: %T; expected string or object", val)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid options; reason: %w", err)
	}

	return opts, nil
}

func toUniversalOptions(options interface{}) (*redis.UniversalOptions, error) {
	uopts := &redis.UniversalOptions{Protocol: 2}

	switch o := options.(type) {
	case *clusterNodesMapOptions:
		setClusterOptions(uopts, &o.commonClusterOptions)

		for _, n := range o.Nodes {
			ropts, err := n.toRedisOptions()
			if err != nil {
				return nil, err
			}
			if err = setConsistentOptions(uopts, ropts); err != nil {
				return nil, err
			}
		}
	case *clusterNodesStringOptions:
		setClusterOptions(uopts, &o.commonClusterOptions)

		for _, n := range o.Nodes {
			ropts, err := redis.ParseURL(n)
			if err != nil {
				return nil, err
			}
			if err = setConsistentOptions(uopts, ropts); err != nil {
				return nil, err
			}
		}
	case *sentinelOptions:
		uopts.MasterName = o.MasterName
		uopts.SentinelUsername = o.SentinelUsername
		uopts.SentinelPassword = o.SentinelPassword

		ropts, err := o.toRedisOptions()
		if err != nil {
			return nil, err
		}
		if err = setConsistentOptions(uopts, ropts); err != nil {
			return nil, err
		}
	case *singleNodeOptions:
		ropts, err := o.toRedisOptions()
		if err != nil {
			return nil, err
		}
		if err = setConsistentOptions(uopts, ropts); err != nil {
			return nil, err
		}
	case *redis.Options:
		if err := setConsistentOptions(uopts, o); err != nil {
			return nil, err
		}
	default:
		panic(fmt.Sprintf("unexpected options type %T", options))
	}

	return uopts, nil
}

// Set UniversalOptions values from single-node options, ensuring that any
// previously set values are consistent with the new values. This validates that
// multiple node options set when using cluster mode are consistent with each other.
// TODO: Break apart, simplify?
//
//nolint:gocognit,cyclop,funlen
func setConsistentOptions(uopts *redis.UniversalOptions, opts *redis.Options) error {
	uopts.Addrs = append(uopts.Addrs, opts.Addr)

	// Only set the TLS config once. Note that this assumes the same config is
	// used in other single-node options, since doing the consistency check we
	// use for the other options would be tedious.
	if uopts.TLSConfig == nil && opts.TLSConfig != nil {
		uopts.TLSConfig = opts.TLSConfig
	}

	if uopts.DB != 0 && opts.DB != 0 && uopts.DB != opts.DB {
		return fmt.Errorf("inconsistent db option: %d != %d", uopts.DB, opts.DB)
	}
	uopts.DB = opts.DB

	if uopts.Username != "" && opts.Username != "" && uopts.Username != opts.Username {
		return fmt.Errorf("inconsistent username option: %s != %s", uopts.Username, opts.Username)
	}
	uopts.Username = opts.Username

	if uopts.Password != "" && opts.Password != "" && uopts.Password != opts.Password {
		return fmt.Errorf("inconsistent password option")
	}
	uopts.Password = opts.Password

	if uopts.ClientName != "" && opts.ClientName != "" && uopts.ClientName != opts.ClientName {
		return fmt.Errorf("inconsistent clientName option: %s != %s", uopts.ClientName, opts.ClientName)
	}
	uopts.ClientName = opts.ClientName

	if uopts.MaxRetries != 0 && opts.MaxRetries != 0 && uopts.MaxRetries != opts.MaxRetries {
		return fmt.Errorf("inconsistent maxRetries option: %d != %d", uopts.MaxRetries, opts.MaxRetries)
	}
	uopts.MaxRetries = opts.MaxRetries

	if uopts.MinRetryBackoff != 0 && opts.MinRetryBackoff != 0 && uopts.MinRetryBackoff != opts.MinRetryBackoff {
		return fmt.Errorf("inconsistent minRetryBackoff option: %d != %d", uopts.MinRetryBackoff, opts.MinRetryBackoff)
	}
	uopts.MinRetryBackoff = opts.MinRetryBackoff

	if uopts.MaxRetryBackoff != 0 && opts.MaxRetryBackoff != 0 && uopts.MaxRetryBackoff != opts.MaxRetryBackoff {
		return fmt.Errorf("inconsistent maxRetryBackoff option: %d != %d", uopts.MaxRetryBackoff, opts.MaxRetryBackoff)
	}
	uopts.MaxRetryBackoff = opts.MaxRetryBackoff

	if uopts.DialTimeout != 0 && opts.DialTimeout != 0 && uopts.DialTimeout != opts.DialTimeout {
		return fmt.Errorf("inconsistent dialTimeout option: %d != %d", uopts.DialTimeout, opts.DialTimeout)
	}
	uopts.DialTimeout = opts.DialTimeout

	if uopts.ReadTimeout != 0 && opts.ReadTimeout != 0 && uopts.ReadTimeout != opts.ReadTimeout {
		return fmt.Errorf("inconsistent readTimeout option: %d != %d", uopts.ReadTimeout, opts.ReadTimeout)
	}
	uopts.ReadTimeout = opts.ReadTimeout

	if uopts.WriteTimeout != 0 && opts.WriteTimeout != 0 && uopts.WriteTimeout != opts.WriteTimeout {
		return fmt.Errorf("inconsistent writeTimeout option: %d != %d", uopts.WriteTimeout, opts.WriteTimeout)
	}
	uopts.WriteTimeout = opts.WriteTimeout

	if uopts.PoolSize != 0 && opts.PoolSize != 0 && uopts.PoolSize != opts.PoolSize {
		return fmt.Errorf("inconsistent poolSize option: %d != %d", uopts.PoolSize, opts.PoolSize)
	}
	uopts.PoolSize = opts.PoolSize

	if uopts.MinIdleConns != 0 && opts.MinIdleConns != 0 && uopts.MinIdleConns != opts.MinIdleConns {
		return fmt.Errorf("inconsistent minIdleConns option: %d != %d", uopts.MinIdleConns, opts.MinIdleConns)
	}
	uopts.MinIdleConns = opts.MinIdleConns

	if uopts.ConnMaxLifetime != 0 && opts.ConnMaxLifetime != 0 && uopts.ConnMaxLifetime != opts.ConnMaxLifetime {
		return fmt.Errorf("inconsistent maxConnAge option: %d != %d", uopts.ConnMaxLifetime, opts.ConnMaxLifetime)
	}
	uopts.ConnMaxLifetime = opts.ConnMaxLifetime

	if uopts.PoolTimeout != 0 && opts.PoolTimeout != 0 && uopts.PoolTimeout != opts.PoolTimeout {
		return fmt.Errorf("inconsistent poolTimeout option: %d != %d", uopts.PoolTimeout, opts.PoolTimeout)
	}
	uopts.PoolTimeout = opts.PoolTimeout

	if uopts.ConnMaxIdleTime != 0 && opts.ConnMaxIdleTime != 0 && uopts.ConnMaxIdleTime != opts.ConnMaxIdleTime {
		return fmt.Errorf("inconsistent idleTimeout option: %d != %d", uopts.ConnMaxIdleTime, opts.ConnMaxIdleTime)
	}
	uopts.ConnMaxIdleTime = opts.ConnMaxIdleTime

	return nil
}

func setClusterOptions(uopts *redis.UniversalOptions, opts *commonClusterOptions) {
	uopts.MaxRedirects = opts.MaxRedirects
	uopts.ReadOnly = opts.ReadOnly
	uopts.RouteByLatency = opts.RouteByLatency
	uopts.RouteRandomly = opts.RouteRandomly
}

func setSocketOptions(opts *redis.Options, sopts *socketOptions) error {
	if sopts == nil {
		return fmt.Errorf("empty socket options")
	}
	opts.Addr = fmt.Sprintf("%s:%d", sopts.Host, sopts.Port)
	opts.DialTimeout = time.Duration(sopts.DialTimeout) * time.Millisecond
	opts.ReadTimeout = time.Duration(sopts.ReadTimeout) * time.Millisecond
	opts.WriteTimeout = time.Duration(sopts.WriteTimeout) * time.Millisecond
	opts.PoolSize = sopts.PoolSize
	opts.MinIdleConns = sopts.MinIdleConns
	opts.ConnMaxLifetime = time.Duration(sopts.MaxConnAge) * time.Millisecond
	opts.PoolTimeout = time.Duration(sopts.PoolTimeout) * time.Millisecond
	opts.ConnMaxIdleTime = time.Duration(sopts.IdleTimeout) * time.Millisecond

	if sopts.TLS != nil {
		//#nosec G402
		tlsCfg := &tls.Config{}
		if len(sopts.TLS.CA) > 0 {
			caCertPool := x509.NewCertPool()
			for _, cert := range sopts.TLS.CA {
				caCertPool.AppendCertsFromPEM([]byte(cert))
			}
			tlsCfg.RootCAs = caCertPool
		}

		if sopts.TLS.Cert != "" && sopts.TLS.Key != "" {
			clientCertPair, err := tls.X509KeyPair([]byte(sopts.TLS.Cert), []byte(sopts.TLS.Key))
			if err != nil {
				return err
			}
			tlsCfg.Certificates = []tls.Certificate{clientCertPair}
		}

		opts.TLSConfig = tlsCfg
	}

	return nil
}
