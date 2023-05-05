//go:build !amd64 && !arm64 && !ppc64 && !ppc64le && !s390x
// +build !amd64,!arm64,!ppc64,!ppc64le,!s390x

package fasthttp

const (
	maxHexIntChars = 7
)
