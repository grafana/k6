/*
Package dns provides DNS client and server implementations.

A client can handle queries for a net.Dialer:

	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,

			Dial: new(dns.Client).Dial,
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")


It can also query a remote DNS server directly:

	client := new(dns.Client)
	query := &dns.Query{
		RemoteAddr: &net.TCPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 53},

		Message: &dns.Message{
			Questions: []dns.Question{
				{
					Name:  "example.com.",
					Type:  dns.TypeA,
					Class: dns.ClassIN,
				},
				{
					Name:  "example.com.",
					Type:  dns.TypeAAAA,
					Class: dns.ClassIN,
				},
			},
		},
	}

	msg, err := client.Do(ctx, query)

A handler answers queries for a server or a local resolver for a client:

	zone := &dns.Zone{
		Origin: "localhost.",
		TTL:    5 * time.Minute,
		RRs: dns.RRSet{
			"alpha": []dns.Record{
				&dns.A{net.IPv4(127, 0, 0, 42).To4()},
				&dns.AAAA{net.ParseIP("::42")},
			},
		},
	}

	srv := &dns.Server{
		Addr:    ":53",
		Handler: zone,
	}

	go srv.ListenAndServe(ctx)

	mux := new(dns.ResolveMux)
	mux.Handle(dns.TypeANY, zone.Origin, zone)

	client := &dns.Client{
		Resolver: mux,
	}

	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial:     client.Dial,
	}

	addrs, err := net.LookupHost("alpha.localhost")

*/
package dns
