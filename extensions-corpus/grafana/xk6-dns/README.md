# xk6-dns

This is a DNS resolver extension for [k6](https://go.k6.io/k6). It allows you to resolve DNS names to IP addresses
in your k6 scripts. 

This extension was designed with the following goals in mind:
- Assert the performance of custom DNS servers under load.
- Provide a way to resolve DNS names to IP addresses using a specific DNS server in k6 scripts.

## Features

This extension provides two functions:
- [`dns.resolve()`](#dnsresolvequery-recordtype-options) - resolves a DNS name to an IP address using the provided DNS server.
- [`dns.lookup()`](#dnslookuphost) - resolves a DNS name to an IP address using the system's default DNS server.

## Usage

### Using the extension

To use this extension requires using the xk6 tool to build a custom k6 binary that includes the extension. You can install the extension using the following command:

```bash
xk6 build --with github.com/grafana/xk6-dns
```

### Example k6 script

From there we can use the bustom built binary to run the following example script:

```javascript
import dns from 'k6/x/dns';

export const options = {
    vus: 10,
    duration: 200,
};

export default async function() {
    // Request the IP address of k6.io from the selected nameserver A records.
    const resolvedIP = await dns.resolve('k6.io', 'A', '9.9.9.9:53');
    console.log(`k6.io IPs as resolved against the 9.9.9.9 nameserver: ${resolvedIP}`);

    // Request the TXT records for k6.io from the selected nameserver.
    const txtRecords = await dns.resolve('k6.io', 'TXT', '9.9.9.9:53');
    console.log(`k6.io TXT records: ${txtRecords}`);
    
    // Lookup the IP address of k6.io using the system's default DNS server.
    const lookupIP = await dns.lookup('k6.io');
    console.log(`k6.io IP as looked up by the system's default DNS server: ${lookupIP}`);
}
```

### Example results

```
          /\      |‾‾| /‾‾/   /‾‾/
     /\  /  \     |  |/  /   /  /
    /  \/    \    |     (   /   ‾‾\
   /          \   |  |\  \ |  (‾)  |
  / __________ \  |__| \__\ \_____/ .io

     execution: local
        script: example.js
        output: -

     scenarios: (100.00%) 1 scenario, 10 max VUs, 10m30s max duration (incl. graceful stop):
              * default: 200 iterations shared among 10 VUs (maxDuration: 10m0s, gracefulStop: 30s)


     data_received.............: 0 B 0 B/s
     data_sent.................: 0 B 0 B/s
     dns_lookup_duration.......: avg=5.9ms  min=0s    med=3.5ms  max=44ms    p(90)=9.1ms   p(95)=11.69ms
     dns_lookups...............: 20  84.316677/s
     dns_resolution_duration...: avg=4.95ms min=2ms   med=3ms    max=31ms    p(90)=6.6ms   p(95)=12.94ms
     dns_resolutions...........: 20  84.316677/s
     iteration_duration........: avg=11.8ms min=2.1ms med=8.45ms max=76.46ms p(90)=17.21ms p(95)=23.16ms
     iterations................: 200 843.166766/s


running (00m00.2s), 00/10 VUs, 200 complete and 0 interrupted iterations
default ✓ [======================================] 10 VUs  00m00.2s/10m0s  200/200 shared iters
```

## API

### `dns.resolve(query, recordType, options)`

Resolves a DNS query using the provided DNS server. It returns an array of results: IP addresses for `A`/`AAAA` records, or strings for `TXT` records.

The `query` parameter is the DNS name to resolve, the `recordType` parameter is the type of DNS record to query for (e.g. 'A', 'AAAA', 'TXT'), and the `options` parameter is an object that can contain the following properties:
- `nameserver` - the IP address and port of the DNS server to query. It should be in the format `ip:port`. If not provided, the system's default DNS server will be used.

Using the `dns.resolve()` operation will emit the following metrics:
- `dns_resolutions`: a [**Counter**](https://grafana.com/docs/k6/latest/using-k6/metrics/) metric tracking the number of DNS resolutions performed.
- `dns_resolution_duration`: a [**Trend**](https://grafana.com/docs/k6/latest/using-k6/metrics/) metric tracking the time taken to resolve the DNS.

### `dns.lookup(host)`

Lookups a host name using the system's default DNS server. It returns an array of IP addresses.

The `host` parameter is the DNS name to resolve.

Using the `dns.lookup()` operation will emit the following metrics:
- `dns_lookups`: a [**Counter**](https://grafana.com/docs/k6/latest/using-k6/metrics/) metric tracking the number of DNS lookups performed.
- `dns_lookup_duration`: A [**Trend**](https://grafana.com/docs/k6/latest/using-k6/metrics/) metric tracking the time taken to lookup the DNS.

## Contributing

Contributions are welcome! If the module is missing a feature you need, or if you find a bug, please open an issue or a pull request. If you are not sure about something, feel free to open an issue and ask.
