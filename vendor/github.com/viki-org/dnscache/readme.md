### A DNS cache for Go
CGO is used to lookup domain names. Given enough concurrent requests and the slightest hiccup in name resolution, it's quite easy to end up with blocked/leaking goroutines.

The issue is documented at <https://code.google.com/p/go/issues/detail?id=5625>

The Go team's singleflight solution (which isn't in stable yet) is rather elegant. However, it only eliminates concurrent lookups (thundering herd problems). Many systems can live with slightly stale resolve names, which means we can cacne DNS lookups and refresh them in the background.

### Installation
Install using the "go get" command:

    go get github.com/viki-org/dnscache

### Usage
The cache is thread safe. Create a new instance by specifying how long each entry should be cached (in seconds). Items will be refreshed in the background.

    //refresh items every 5 minutes
    resolver := dnscache.New(time.Minute * 5)

    //get an array of net.IP
    ips, _ := resolver.Fetch("api.viki.io")

    //get the first net.IP
    ip, _ := resolver.FetchOne("api.viki.io")

    //get the first net.IP as string
    ip, _ := resolver.FetchOneString("api.viki.io")

If you are using an `http.Transport`, you can use this cache by speficifying a
`Dial` function:

    transport := &http.Transport {
      MaxIdleConnsPerHost: 64,
      Dial: func(network string, address string) (net.Conn, error) {
        separator := strings.LastIndex(address, ":")
        ip, _ := dnscache.FetchString(address[:separator])
        return net.Dial("tcp", ip + address[separator:])
      },
    }
