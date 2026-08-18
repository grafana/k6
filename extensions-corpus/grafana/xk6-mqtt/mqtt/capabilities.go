package mqtt

import (
	"fmt"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

func newPromise(vu extensionapi.VU) (*sobek.Promise, extensionapi.PromiseResolver) {
	promises, ok := vu.(extensionapi.Promises)
	if !ok {
		panic("extension API promise capability is unavailable")
	}
	return promises.NewPromise()
}

func callbackRegistrar(vu extensionapi.VU) func() func(func() error) {
	scheduler, ok := vu.(extensionapi.Scheduler)
	if !ok {
		panic("extension API scheduler capability is unavailable")
	}
	return func() func(func() error) {
		callback := scheduler.RegisterCallback()
		return func(task func() error) { callback(extensionapi.Task(task)) }
	}
}

func networkFor(vu extensionapi.VU) (extensionapi.Network, error) {
	network, ok := vu.(extensionapi.Network)
	if !ok {
		return nil, fmt.Errorf("MQTT connections are not allowed in the init context")
	}
	return network, nil
}
