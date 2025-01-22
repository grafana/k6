// Package pbcloud contains the Protobuf definitions used
// for the metrics flush RPCs.
package pbcloud

//go:generate protoc --go_out=. --go_opt=paths=source_relative ./metric.proto
