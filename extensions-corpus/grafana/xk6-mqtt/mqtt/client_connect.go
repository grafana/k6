package mqtt

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

var (
	errNotConnected = errors.New("not connected")
	errCredProvider = errors.New("credentials provider failed")
)

type connectOptions struct {
	Keepalive      sobek.Value
	ConnectTimeout sobek.Value
	CleanSession   bool
	Servers        []string
	Tags           map[string]string
}

func (co *connectOptions) toPaho(opts *paho.ClientOptions) {
	if sobek.IsNumber(co.Keepalive) {
		opts.SetKeepAlive(time.Second * time.Duration(co.Keepalive.ToInteger()))
	}

	if sobek.IsNumber(co.ConnectTimeout) && co.ConnectTimeout.ToInteger() >= 0 {
		opts.SetConnectTimeout(time.Millisecond * time.Duration(co.ConnectTimeout.ToInteger()))
	}

	if co.CleanSession {
		opts.SetCleanSession(true)
	}

	for _, server := range co.Servers {
		opts.AddBroker(server)
	}
}

func (c *client) connect(urlOrOpts sobek.Value, optsOrEmpty sobek.Value) error {
	err := c.connectPrepare(urlOrOpts, optsOrEmpty)
	if err != nil {
		return err
	}

	return c.connectExecute()
}

func (c *client) connectAsync(urlOrOpts sobek.Value, optsOrEmpty sobek.Value) (*sobek.Promise, error) {
	err := c.connectPrepare(urlOrOpts, optsOrEmpty)
	if err != nil {
		return nil, err
	}

	promise, resolver := newPromise(c.vu)

	go func() {
		if err := c.connectExecute(); err != nil {
			resolver.Reject(err)

			return
		}

		resolver.Resolve(nil)
	}()

	return promise, nil
}

func (c *client) reconnect() error {
	c.disconnect()

	c.log.Debug("Reconnecting to MQTT broker")

	return c.connectExecute()
}

func (c *client) reconnectAsync() (*sobek.Promise, error) {
	promise, resolver := newPromise(c.vu)

	go func() {
		if err := c.reconnect(); err != nil {
			resolver.Reject(err)

			return
		}

		resolver.Resolve(nil)
	}()

	return promise, nil
}

func (c *client) isConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.pahoClient == nil {
		return false
	}

	return c.pahoClient.IsConnected()
}

func (c *client) connectPrepare(urlOrOpts sobek.Value, optsOrEmpty sobek.Value) error {
	c.disconnect()

	var (
		urlStr string
		opts   *connectOptions
	)

	switch urlOrOpts.ExportType() {
	case reflect.TypeFor[string]():
		urlStr = urlOrOpts.String()
		urlOrOpts = optsOrEmpty

	case reflect.TypeFor[map[string]any]():

	default:
		return fmt.Errorf("%w: expected string or object", errInvalidType)
	}

	if urlOrOpts != nil && !sobek.IsUndefined(urlOrOpts) && !sobek.IsNull(urlOrOpts) {
		if urlOrOpts.ExportType() != reflect.TypeFor[map[string]any]() {
			return fmt.Errorf("%w: expected object", errInvalidType)
		}

		if err := c.vu.Runtime().ExportTo(urlOrOpts, &opts); err != nil {
			return err
		}
	} else {
		opts = new(connectOptions)
	}

	c.connOpts = opts

	err := c.validateAddress(urlStr)
	if err != nil {
		return err
	}

	c.url = urlStr

	return nil
}

func (c *client) validateAddress(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	network, err := networkFor(c.vu)
	if err != nil {
		return err
	}
	_, err = network.LookupHost(c.vu.Context(), u.Hostname())
	return err
}

func (c *client) connectExecute() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.log.Debug("Connecting to MQTT broker")

	c.pahoClient = c.newPahoClient()

	if token := c.pahoClient.Connect(); token.Wait() && token.Error() != nil {
		if err := c.handleError(token.Error(), "connect", c.connOpts.Tags, "url", c.url); err != nil {
			return err
		}

		return nil
	}

	c.addCallMetrics("connect", nil)

	return nil
}

func (c *client) disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pahoClient == nil {
		return
	}

	c.stopLoop()

	if c.pahoClient.IsConnected() {
		c.pahoClient.Disconnect(0)
	}

	c.pahoClient = nil
}

func (c *client) newPahoClient() paho.Client {
	opts := paho.NewClientOptions()

	c.clientOpts.toPaho(opts, c.vu.Runtime())

	if len(c.url) != 0 {
		opts.AddBroker(c.url)
	}

	c.connOpts.toPaho(opts)

	opts.SetDefaultPublishHandler(c.messageHandler)
	opts.SetOnConnectHandler(c.connectHandler)
	opts.SetReconnectingHandler(c.reconnectHandler)

	opts.SetCustomOpenConnectionFn(c.openConnection)

	return paho.NewClient(opts)
}

func (c *client) openConnection(uri *url.URL, opts paho.ClientOptions) (net.Conn, error) {
	network, err := networkFor(c.vu)
	if err != nil {
		return nil, err
	}
	conn, err := network.DialContext(c.vu.Context(), "tcp", uri.Host)
	if err != nil {
		return nil, err
	}
	switch uri.Scheme {
	case "mqtt", "tcp":
		return conn, nil
	case "ssl", "tls", "mqtts", "mqtt+ssl", "tcps":
		tlsCapability, ok := c.vu.(extensionapi.TLS)
		if !ok {
			_ = conn.Close()
			return nil, fmt.Errorf("MQTT TLS connections are not allowed in the init context")
		}
		config := opts.TLSConfig
		if config == nil {
			config = &tls.Config{}
		}
		if config.ServerName == "" {
			config = config.Clone()
			config.ServerName = uri.Hostname()
		}
		return tlsCapability.TLSClient(c.vu.Context(), conn, config)
	default:
		_ = conn.Close()
		return nil, fmt.Errorf("MQTT protocol %q is not supported by the extension API network capability", uri.Scheme)
	}
}
