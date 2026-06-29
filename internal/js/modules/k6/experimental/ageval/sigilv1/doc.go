// Package sigilv1 contains the protobuf/gRPC types for Grafana's Sigil
// (AI observability) generation-ingest API, vendored from
// github.com/grafana/sigil-sdk (go/proto/sigil/v1).
//
// They are copied in (rather than depended on as a module) so the experimental
// ageval module can run an in-process GenerationIngestService server without
// pulling the full sigil-sdk and its OTel SDK dependencies into k6's go.mod.
// The two *.pb.go files are generated code copied verbatim; re-vendor them if
// the upstream proto changes.
package sigilv1