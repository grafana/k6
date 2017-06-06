/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package netext

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/viki-org/dnscache"
)

type Dialer struct {
	net.Dialer

	Resolver *dnscache.Resolver
}

func NewDialer(dialer net.Dialer) *Dialer {
	return &Dialer{
		Dialer:   dialer,
		Resolver: dnscache.New(0),
	}
}

func (d Dialer) DialContext(ctx context.Context, proto, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip, err := d.Resolver.FetchOne(host)
	if err != nil {
		return nil, err
	}
	conn, err := d.Dialer.DialContext(ctx, proto, ip.String()+":"+port)
	if err != nil {
		return nil, err
	}
	if v := ctx.Value(ctxKeyTracer); v != nil {
		tracer := v.(*Tracer)
		return d.TraceConnection(conn, &tracer.bytesRead, &tracer.bytesWritten)
	}
	return conn, err
}

func (d Dialer) TraceConnection(conn net.Conn, bytesRead, bytesWritten *int64) (*Conn, error) {
	return &Conn{conn, bytesRead, bytesWritten}, nil
}

type Conn struct {
	net.Conn

	BytesRead, BytesWritten *int64
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	atomic.AddInt64(c.BytesRead, int64(n))
	return n, err
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	atomic.AddInt64(c.BytesWritten, int64(n))
	return n, err
}
