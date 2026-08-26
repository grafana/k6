package mqtt

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/promises"
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

	promise, resolve, reject := promises.New(c.vu)

	go func() {
		if err := c.connectExecute(); err != nil {
			reject(err)

			return
		}

		resolve(nil)
	}()

	return promise, nil
}

func (c *client) reconnect() error {
	c.disconnect()

	c.log.Debug("Reconnecting to MQTT broker")

	return c.connectExecute()
}

func (c *client) reconnectAsync() (*sobek.Promise, error) {
	promise, resolve, reject := promises.New(c.vu)

	go func() {
		if err := c.reconnect(); err != nil {
			reject(err)

			return
		}

		resolve(nil)
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

	_, _, err = c.vu.State().GetAddrResolver().ResolveAddr(u.Host)

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

	if conf := c.vu.State().TLSConfig; conf != nil {
		// Overriding the NextProtos to avoid talking http2
		// @see https://github.com/grafana/xk6-mqtt/issues/20
		tlsConfig := conf.Clone()

		if strings.HasPrefix(c.url, "wss://") {
			tlsConfig.NextProtos = []string{"http/1.1"}
		}

		opts.SetTLSConfig(tlsConfig)
	}

	return paho.NewClient(opts)
}
