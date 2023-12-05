/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package main implements a simple gRPC server that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It implements the route guide service whose definition can be found in routeguide/route_guide.proto.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"google.golang.org/grpc"

	"go.k6.io/k6/lib/testutils/grpcservice"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"

	"google.golang.org/grpc/reflection"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "", "A json file containing a list of features")
	port       = flag.Int("port", 10000, "The server port")
)

func main() {
	flag.Parse()

	log.Printf("gRPC server starting on localhost:%d\n", *port)

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		if *certFile == "" {
			*certFile = testdata.Path("server1.pem")
		}
		if *keyFile == "" {
			*keyFile = testdata.Path("server1.key")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	features := grpcservice.LoadFeatures(*jsonDBFile)
	grpcServer := grpc.NewServer(opts...)
	grpcservice.RegisterRouteGuideServer(grpcServer, grpcservice.NewRouteGuideServer(features...))
	grpcservice.RegisterFeatureExplorerServer(grpcServer, grpcservice.NewFeatureExplorerServer(features...))
	reflection.Register(grpcServer)
	grpcServer.Serve(lis)
}
