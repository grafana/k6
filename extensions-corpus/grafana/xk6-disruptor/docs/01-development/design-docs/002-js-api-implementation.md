
# Design Doc: Design of a JS API implementation framework

|                       |               | 
|-----------------------|---------------|
|**Author(s)**:         | @pablochacin |
|**Created**:           | 2023-03-22 |
|**Status**:            | Accepted |
|**Last status change**:| 2023-04-21 |
|**Approver(s)**:       | @pablochacin |
|**Related**| N/A |
|**Replaces**| N/A |
|**Superseded by** | N/A |


## Background

In the following description we define the `JS API` as the API as seen by the JS code and the `API` as its implementation in the extension. 

xk6-disruptor is built around the concept disruptors that inject faults. Disruptors implement one method for each type of fault they inject and different disruptors can implement the same method if they are able to inject the same type of faults. For example, `ServiceDisruptor` and `PodDisruptor` both implement the `InjectHTTPFault` method for injecting `HttpFaults`.

```go
type PodDisruptor { ... }

func (d *PodDisruptor)InjectHTTPFault(fault HTTPFault) error { ... }

type ServiceDisruptor { ... }

func (d *ServiceDisruptor)InjectHTTPFault(fault HTTPFault) error { ... }
```

This design is convenient because it allows reusing fault definitions across different disruptors, while each disruptor can modify or enhance the behavior of their fault injection methods. For example for the `PodDisruptor` the `port` parameter defined in the `HTTPFault` refers to the port exposed by the Pod that is the target for the fault injection, while for the `ServiceDisruptor` it indicates the service port - which is mapped to the corresponding pod port.

The JS API follows the same model than the extension. For example, the code below creates a `PodDisruptor` in JS and invokes its `injectHTTPFaults` method:

```javascript
const d = new PodDisruptor(...)
d.injectHTTPFaults(...)
```

The JS API is implemented by the [API layer](https://github.com/grafana/xk6-disruptor/blob/main/pkg/api/api.go) as a series of adapters between Javascript code and the disruptors.

Each adapter implements the same fault injection methods than the disruptor it "wraps". These methods validate and convert the objects received from Javascript, delegate the execution to the corresponding disruptor and return the result or rises an error. 

For example, consider the `JsPodDisruptor`, the adapter for the `PodDisruptor`:

```go
type JsPodDisruptor struct {
        PodDisruptor
}

func (j *JsPodDisruptor)InjectHTTPFaults(args ...goja.Value) {
        // validate arguments from JavaScript

        // transform arguments to a HTTPFault object
         := HTTPFault{}

        // delegate call to disruptor
        err := j.PodDisruptor.InjectHTTPFaults(f)

        // handle errors and return values
}
```
Notice that the adapter for `ServiceDisruptor` must also implement the `InjectHTTPFaults` method:

```go
type JsServiceDisruptor struct {
        ServiceDisruptor
}

func (j *JsServiceDisruptor)InjectHTTPFaults(args ...goja.Value) {
        // validate arguments from JavaScript

        // transform arguments to a HTTPFault object
        f := HTTPFault{}

        // delegate call to disruptor
        err := j.ServoceDisruptor.InjectHTTPFaults(f)

        // handle errors and return values
}
```

## Problem statement

Each adapter must implement the methods of the disruptor it adapts, creating significant duplication in the implementation of the API. Following the example above, the adapters for `PodDisruptor` and `ServiceDisruptor` must both implement the method `InjectHTTPFault`.

In the reminder of this document we make more emphasis on the existing duplicity between the `ServiceDisruptor` and the `PodDisruptor`, but the same duplicity may occur between other unrelated disruptors. For example, both a `PodDisruptor` and a `NodeDisruptor` can implement an `InjectNetworkFault` method for injecting network-level disruptions.

## Goals

1. Reduce redundancies in the implementation of the JS API
2. Facilitate extending the API by adding more faults and disruptors following a well-defined programming model

### Non-goals (optional)

1. Redesign the JS API as seen by the JS code.
2. Decouple the JS API from the API implemented by the disruptors. In the reminder of this document, we assume there is a one-to-one correspondence.

### Out of scope

1. Consider a redesign of the API around other concepts than disruptors and faults.

## Proposal: Create fault injection interfaces

The duplicity can be mitigated by creating individual interfaces for each fault injection methods and create adapters for each interface, instead of for each disruptor. In this way, the adapters of multiple disruptors can reuse the adapters of the interfaces they implement.

For example, the `ProtocolFaultInjector` interface defines the `InjectHTTPFaults` method. `PodDisruptor` and `ServiceDisruptor` both implement this interface.

Similarly `PodDisruptor` and `NodeDisruptor` would both implement a `NetworkFaultInjector` interface that defines the `InjectNetworkFault` method.

Notice that all disruptors should implement the `Disruptor` interface that defines the `Targets` method, which returns the list of targets of the disruptor (pods for pod disruptor, nodes for NodeDisruptor).

The JS API implements an adapter for each interface that validate and convert the arguments received from JavaScript and delegates the execution to any disruptor that implements this type of fault.

For example, for the `ProtocolFaultInjector`:

```go
type JsProtocolFaultInjector struct {
        injector: ProtocolFaultInjector
}

func (j JsHTTPFaultInjector)InjectHTTPFaults(args ...goja.Value) {
        // validate arguments from JavaScript

        // transform arguments to a HTTPFault object
        f := HTTPFault{}

        // delegate call to fault injector
        err := j.injector.InjectHTTPFaults(f)

        // handle errors
}
```
We would have a similar adapter for the `NetworkFaultInjector` interface.

The adapters for the disruptors are implemented by composing the adapters for each interface. In this way, the interfaces can be reused across disruptors without any additional implementation code.

For example, the adapter for `PodDisruptor` will be defined as follows:

```go
type JsPodDisruptor struct {
        JsDisruptor
        JsProtocolFaultInjector
        JsNetworkDisruptor
}
```

Similarly, the adapter for `ServiceDisruptor` will be defined as follows:

```go
type JsServiceDisruptor struct {
        JsDisruptor
        JsProtocolFaultInjector
}
```

See annex [Code Example](#annex-code-example) for a detailed example.

#### Advantages

1. Reduce code duplicity between wrappers that implement the same fault injection methods
2. Extending the fault injection and disruptor catalog follows a well-known pattern.

#### Disadvantages

1. Ties the JS API to the API implemented by the disruptors. If they diverge, the wrappers would require more logic.


## Annex: Code Example

This annex shows an example 

### API

The API is the implementation of xk6-disruptor capabilities in golang as a series of disruptors that can inject faults into their targets.

#### Disruptor 

Disruptor is a generic disruptor interface that all disruptors must implement.

```go
type Disruptor interface {
	Targets() []string
}
```

#### Protocol Fault Injection


```go
type HTTPFault struct{}

type ProtocolFaultInjector interface {
	InjectHttpFaults(HttpFault) error
}
```

#### Network Fault Injection

```go
type NetworkFault struct{}

type NetworkFaultInjector interface {
	InjectNetworkFaults(NetworkFault) error
}
```

#### PodDisruptor

```go
type PodDisruptor struct{}

func (p PodDisruptor) Targets() []string {
	return []string{}
}

func (p PodDisruptor) InjectHttpFaults(HttpFault) error {
	return nil
}

func (p PodDisruptor) InjectNetworkFaults(NetworkFault) error {
	return nil
}
```

#### ServiceDisruptor

```go
type ServiceDisruptor struct{}

func (s ServiceDisruptor) Targets() []string {
	return []string{}
}

func (s ServiceDisruptor) InjectHttpFaults(HttpFault) error {
	return nil
}
```

#### NodeDisruptor

```go
type NodeDisruptor struct{}

func (n NodeDisruptor) Targets() []string {
	return []string{}
}

func (n NodeDisruptor) InjectNetworkFaults(NetworkFault) error {
	return nil
}
```

### JS API

The JS API implements the API between JS and the disruptors.

In defines the type `JsDisruptor` that implement the generic `Disruptor` interface:

```go
type JsDisruptor struct {
	Disruptor
}

func (d JsDisruptor) Targets() []string {
	return d.Disruptor.Targets()
}
```

It also defines an adapter that implements the JS API for each type of fault injection. For example, `JsHttpFaultInjector` implements `HttpFaultInjector`. 

```go
type JsProtocolFaultInjector struct {
        ProtocolFaultInjector
}

func (j JsProtocolFaultInjector)InjectHTTPFaults(args ...goja.Value) {
        // validate arguments from JavaScript

        // transform arguments to a HTTPFault object
        f := HTTPFault{}

        // delegate call to disruptor
        err := j.ProtocolFaultInjector.InjectHTTPFaults(f)

        // handle errors
}
```

Similarly, we can define `JsDisruptor` for `Disruptor` interface and `JsNetworkFaultInjector` for `NetworkFaultInjector` interface.

Finally, we implement an adapter for each type of disruptor. Each wrapper embeds the types that implement the methods required by the wrapper:

```go
type JsPodDisruptor struct {
        JsDisruptor
        JsHttpFaultInjector
        JsNetworkDisruptor
}

type JsServiceDisruptor struct {
        JsDisruptor
        JsHttpFaultInjector
}

type JsNodeDisruptor struct {
        JsDisruptor
        JsNetworkDisruptor
}
```

We can create a `JsPodDisruptor` using an instance of `PodDisruptor` as follows:

```go
d := PodDisruptor{...}
j := JsPodDistuptor{
        JsDisruptor:         JsDisruptor{d},
        JsHttpFaultInjector: JsHttpFaultInjector{d},
        JsNetworkDisruptor:  JsNetworkDisruptor{d},
}
```

The `JsPodDisruptor` implements the `InjectHTTPFaults`, `InjectNetworkFaults` and `Targets` functions. 

Similarly, we can create a `JsServiceDisruptor` using an instance of `ServiceDisruptor` as follows:

```go
d := ServiceDisruptor{...}
j := JsServiceDistuptor{
        JsDisruptor:         JsDisruptor{d},
        JsHttpFaultInjector: JsHttpFaultInjector{d},
}
```

The `JsServiceDisruptor` implements the `InjectHTTPFaults`, and `Targets` functions.

Finally, we can implement a `JsNodeDisruptor`:

```go
d := NodeDisruptor{...}
j := JsNodeDistuptor{
        JsDisruptor:        JsDisruptor{d},
        JsNetworkDisruptor: JsNetworkDisruptor{d},
}
```

The `JsNodeDisruptor` implements the `InjectNetworkFaults` and `Targets` functions. 
