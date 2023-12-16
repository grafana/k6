//go:build go1.20 && !go1.21

package qtls

import (
	"crypto/tls"
	"fmt"
	"unsafe"

	"github.com/quic-go/quic-go/internal/protocol"

	"github.com/quic-go/qtls-go1-20"
)

type (
	QUICConn            = qtls.QUICConn
	QUICConfig          = qtls.QUICConfig
	QUICEvent           = qtls.QUICEvent
	QUICEventKind       = qtls.QUICEventKind
	QUICEncryptionLevel = qtls.QUICEncryptionLevel
	AlertError          = qtls.AlertError
)

const (
	QUICEncryptionLevelInitial     = qtls.QUICEncryptionLevelInitial
	QUICEncryptionLevelEarly       = qtls.QUICEncryptionLevelEarly
	QUICEncryptionLevelHandshake   = qtls.QUICEncryptionLevelHandshake
	QUICEncryptionLevelApplication = qtls.QUICEncryptionLevelApplication
)

const (
	QUICNoEvent                     = qtls.QUICNoEvent
	QUICSetReadSecret               = qtls.QUICSetReadSecret
	QUICSetWriteSecret              = qtls.QUICSetWriteSecret
	QUICWriteData                   = qtls.QUICWriteData
	QUICTransportParameters         = qtls.QUICTransportParameters
	QUICTransportParametersRequired = qtls.QUICTransportParametersRequired
	QUICRejectedEarlyData           = qtls.QUICRejectedEarlyData
	QUICHandshakeDone               = qtls.QUICHandshakeDone
)

func SetupConfigForServer(conf *QUICConfig, enable0RTT bool, getDataForSessionTicket func() []byte, handleSessionTicket func([]byte, bool) bool) {
	qtls.InitSessionTicketKeys(conf.TLSConfig)
	conf.TLSConfig = conf.TLSConfig.Clone()
	conf.TLSConfig.MinVersion = tls.VersionTLS13
	conf.ExtraConfig = &qtls.ExtraConfig{
		Enable0RTT: enable0RTT,
		Accept0RTT: func(data []byte) bool {
			return handleSessionTicket(data, true)
		},
		GetAppDataForSessionTicket: getDataForSessionTicket,
	}
}

func SetupConfigForClient(conf *QUICConfig, getDataForSessionState func() []byte, setDataFromSessionState func([]byte) bool) {
	conf.ExtraConfig = &qtls.ExtraConfig{
		GetAppDataForSessionState:  getDataForSessionState,
		SetAppDataFromSessionState: setDataFromSessionState,
	}
}

func QUICServer(config *QUICConfig) *QUICConn {
	return qtls.QUICServer(config)
}

func QUICClient(config *QUICConfig) *QUICConn {
	return qtls.QUICClient(config)
}

func ToTLSEncryptionLevel(e protocol.EncryptionLevel) qtls.QUICEncryptionLevel {
	switch e {
	case protocol.EncryptionInitial:
		return qtls.QUICEncryptionLevelInitial
	case protocol.EncryptionHandshake:
		return qtls.QUICEncryptionLevelHandshake
	case protocol.Encryption1RTT:
		return qtls.QUICEncryptionLevelApplication
	case protocol.Encryption0RTT:
		return qtls.QUICEncryptionLevelEarly
	default:
		panic(fmt.Sprintf("unexpected encryption level: %s", e))
	}
}

func FromTLSEncryptionLevel(e qtls.QUICEncryptionLevel) protocol.EncryptionLevel {
	switch e {
	case qtls.QUICEncryptionLevelInitial:
		return protocol.EncryptionInitial
	case qtls.QUICEncryptionLevelHandshake:
		return protocol.EncryptionHandshake
	case qtls.QUICEncryptionLevelApplication:
		return protocol.Encryption1RTT
	case qtls.QUICEncryptionLevelEarly:
		return protocol.Encryption0RTT
	default:
		panic(fmt.Sprintf("unexpect encryption level: %s", e))
	}
}

//go:linkname cipherSuitesTLS13 github.com/quic-go/qtls-go1-20.cipherSuitesTLS13
var cipherSuitesTLS13 []unsafe.Pointer

//go:linkname defaultCipherSuitesTLS13 github.com/quic-go/qtls-go1-20.defaultCipherSuitesTLS13
var defaultCipherSuitesTLS13 []uint16

//go:linkname defaultCipherSuitesTLS13NoAES github.com/quic-go/qtls-go1-20.defaultCipherSuitesTLS13NoAES
var defaultCipherSuitesTLS13NoAES []uint16

var cipherSuitesModified bool

// SetCipherSuite modifies the cipherSuiteTLS13 slice of cipher suites inside qtls
// such that it only contains the cipher suite with the chosen id.
// The reset function returned resets them back to the original value.
func SetCipherSuite(id uint16) (reset func()) {
	if cipherSuitesModified {
		panic("cipher suites modified multiple times without resetting")
	}
	cipherSuitesModified = true

	origCipherSuitesTLS13 := append([]unsafe.Pointer{}, cipherSuitesTLS13...)
	origDefaultCipherSuitesTLS13 := append([]uint16{}, defaultCipherSuitesTLS13...)
	origDefaultCipherSuitesTLS13NoAES := append([]uint16{}, defaultCipherSuitesTLS13NoAES...)
	// The order is given by the order of the slice elements in cipherSuitesTLS13 in qtls.
	switch id {
	case tls.TLS_AES_128_GCM_SHA256:
		cipherSuitesTLS13 = cipherSuitesTLS13[:1]
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		cipherSuitesTLS13 = cipherSuitesTLS13[1:2]
	case tls.TLS_AES_256_GCM_SHA384:
		cipherSuitesTLS13 = cipherSuitesTLS13[2:]
	default:
		panic(fmt.Sprintf("unexpected cipher suite: %d", id))
	}
	defaultCipherSuitesTLS13 = []uint16{id}
	defaultCipherSuitesTLS13NoAES = []uint16{id}

	return func() {
		cipherSuitesTLS13 = origCipherSuitesTLS13
		defaultCipherSuitesTLS13 = origDefaultCipherSuitesTLS13
		defaultCipherSuitesTLS13NoAES = origDefaultCipherSuitesTLS13NoAES
		cipherSuitesModified = false
	}
}

func SendSessionTicket(c *QUICConn, allow0RTT bool) error {
	return c.SendSessionTicket(allow0RTT)
}
