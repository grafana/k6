package grpc

import (
	"crypto/tls"
	"database/sql"
	"net"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/types"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"gopkg.in/guregu/null.v3"
)

func doReflect(
	client reflectpb.ServerReflection_ServerReflectionInfoClient,
	req *reflectpb.ServerReflectionRequest) (*reflectpb.ServerReflectionResponse, error) {
	if err := client.Send(req); err != nil {
		return nil, err
	}
	return client.Recv()
}

func reflectState() *lib.State {
	return &lib.State{
		TLSConfig: &tls.Config{},
		Options: lib.Options{UserAgent: null.String{NullString: sql.NullString{
			String: "k6",
			Valid:  true,
		}}, HTTPDebug: null.String{}},
		Dialer: netext.NewDialer(
			net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 10 * time.Second,
			}, netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4)),
	}
}
