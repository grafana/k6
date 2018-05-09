// +build !static
// +build static_all

package kafka

// #cgo pkg-config: rdkafka-static
// #cgo LDFLAGS: -static
import "C"
