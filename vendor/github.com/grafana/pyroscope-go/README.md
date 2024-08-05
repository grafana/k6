# Pyroscope Golang Client

This is a golang integration for Pyroscope — open source continuous profiling platform.

For more information, please visit our [golang integration documentation](https://grafana.com/docs/pyroscope/latest/configure-client/language-sdks/go_push/).

### Profiling Go applications

To start profiling a Go application, you need to include our go module in your app:

```
go get github.com/grafana/pyroscope-go
```

Then add the following code to your application:

```go
package main

import "github.com/grafana/pyroscope-go"

func main() {
  pyroscope.Start(pyroscope.Config{
    ApplicationName: "simple.golang.app",

    // replace this with the address of pyroscope server
    ServerAddress:   "http://pyroscope-server:4040",

    // you can disable logging by setting this to nil
    Logger:          pyroscope.StandardLogger,

    // Optional HTTP Basic authentication (Grafana Cloud)
    BasicAuthUser:     "<User>",
    BasicAuthPassword: "<Password>",
    // Optional Pyroscope tenant ID (only needed if using multi-tenancy). Not needed for Grafana Cloud.
    // TenantID:          "<TenantID>",

    // by default all profilers are enabled,
    // but you can select the ones you want to use:
    ProfileTypes: []pyroscope.ProfileType{
      pyroscope.ProfileCPU,
      pyroscope.ProfileAllocObjects,
      pyroscope.ProfileAllocSpace,
      pyroscope.ProfileInuseObjects,
      pyroscope.ProfileInuseSpace,
    },
  })

  // your code goes here
}
```

### Tags

It is possible to add tags (labels) to the profiling data. These tags can be used to filter the data in the UI.

```go
// these two ways of adding tags are equivalent:
pyroscope.TagWrapper(context.Background(), pyroscope.Labels("controller", "slow_controller"), func(c context.Context) {
  slowCode()
})

pprof.Do(context.Background(), pprof.Labels("controller", "slow_controller"), func(c context.Context) {
  slowCode()
})
```

### Pull Mode

Go integration supports pull mode, which means that you can profile applications without adding any extra code. For that to work you will need to make sure you have profiling routes (`/debug/pprof`) enabled in your http server. Generally, that means that you need to add `net/http/pprof` package:

```go
import _ "net/http/pprof"
```

### Examples

Check out the [examples](https://grafana.com/docs/pyroscope/latest/configure-client/grafana-agent/go_pull/) directory in our repository to learn more 🔥
