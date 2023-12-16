//go:build !go1.20

package qtls

var _ int = "The version of quic-go you're using can't be built using outdated Go versions. For more details, please see https://github.com/quic-go/quic-go/wiki/quic-go-and-Go-versions."
