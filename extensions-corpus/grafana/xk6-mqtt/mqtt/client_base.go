package mqtt

import (
	"fmt"
	"sync"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

type will struct {
	Topic   string
	Payload string
	Qos     byte
	Retain  bool
}

type credentials struct {
	Username string
	Password string //nolint:gosec // user-supplied connection credential field, not a hardcoded secret
}

type clientOptions struct {
	ClientId            sobek.Value //nolint:revive
	Username            sobek.Value
	Password            sobek.Value
	CredentialsProvider sobek.Callable
	Will                *will
	Tags                map[string]string
}

func (co *clientOptions) toPaho(opts *paho.ClientOptions, runtime *sobek.Runtime) {
	if sobek.IsString(co.ClientId) {
		opts.SetClientID(co.ClientId.String())
	}

	if sobek.IsString(co.Username) {
		opts.SetUsername(co.Username.String())
	}

	if sobek.IsString(co.Password) {
		opts.SetPassword(co.Password.String())
	}

	cp := co.getCredentialsProvider(runtime)
	if cp != nil {
		opts.SetCredentialsProvider(cp)
	}

	if co.Will != nil {
		opts.SetWill(co.Will.Topic, co.Will.Payload, co.Will.Qos, co.Will.Retain)
	}
}

func (co *clientOptions) getCredentialsProvider(runtime *sobek.Runtime) paho.CredentialsProvider {
	if co.CredentialsProvider == nil {
		return nil
	}

	return func() (string, string) {
		credsValue, err := co.CredentialsProvider(sobek.Undefined())
		if err != nil {
			common.Throw(runtime, fmt.Errorf("%w: %s", errCredProvider, err.Error()))
		}

		var creds credentials

		err = runtime.ExportTo(credsValue, &creds)
		if err != nil {
			common.Throw(runtime, fmt.Errorf("%w: %s", errCredProvider, err.Error()))
		}

		return creds.Username, creds.Password
	}
}

type client struct {
	pahoClient paho.Client

	url string
	log logrus.FieldLogger

	clientOpts *clientOptions
	connOpts   *connectOptions

	handlers sync.Map

	vu       modules.VU
	callChan chan func() error
	stop     chan struct{}
	stopOnce sync.Once

	metrics *mqttMetrics

	mu sync.RWMutex
}

func newClient(log logrus.FieldLogger, vu modules.VU, metrics *mqttMetrics) *client {
	c := new(client)
	c.log = log
	c.vu = vu
	c.callChan = make(chan func() error)
	c.stop = make(chan struct{})
	c.connOpts = new(connectOptions)

	c.metrics = metrics

	return c
}

func (c *client) stopLoop() {
	c.stopOnce.Do(func() { close(c.stop) })
}

func (m *module) client(call sobek.ConstructorCall) *sobek.Object {
	toValue := m.vu.Runtime().ToValue

	c := newClient(m.log, m.vu, m.metrics)
	this := call.This
	must := func(err error) {
		if err != nil {
			common.Throw(m.vu.Runtime(), err)
		}
	}

	c.clientOpts = new(clientOptions)

	if len(call.Arguments) > 0 {
		must(m.vu.Runtime().ExportTo(call.Arguments[0], &c.clientOpts))
	}

	must(this.Set("connect", toValue(c.connect)))
	must(this.Set("connectAsync", toValue(c.connectAsync)))
	must(this.Set("end", toValue(c.end)))
	must(this.Set("endAsync", toValue(c.endAsync)))
	must(this.Set("reconnect", toValue(c.reconnect)))
	must(this.Set("reconnectAsync", toValue(c.reconnectAsync)))
	must(this.Set("publish", toValue(c.publish)))
	must(this.Set("publishAsync", toValue(c.publishAsync)))
	must(this.Set("subscribe", toValue(c.subscribe)))
	must(this.Set("subscribeAsync", toValue(c.subscribeAsync)))
	must(this.Set("unsubscribe", toValue(c.unsubscribe)))
	must(this.Set("unsubscribeAsync", toValue(c.unsubscribeAsync)))
	must(this.Set("on", toValue(c.on)))

	must(this.DefineAccessorProperty("connected", toValue(c.isConnected), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))

	go c.loop()

	return nil
}
