package pgproto3

import (
	"bytes"
	"encoding/binary"

	"github.com/jackc/pgx/pgio"
	"github.com/pkg/errors"
)

const (
	AuthTypeOk                = 0
	AuthTypeCleartextPassword = 3
	AuthTypeMD5Password       = 5
	AuthTypeSASL              = 10
	AuthTypeSASLContinue      = 11
	AuthTypeSASLFinal         = 12
)

type Authentication struct {
	Type uint32

	// MD5Password fields
	Salt [4]byte

	// SASL fields
	SASLAuthMechanisms []string

	// SASLContinue and SASLFinal data
	SASLData []byte
}

func (*Authentication) Backend() {}

func (dst *Authentication) Decode(src []byte) error {
	*dst = Authentication{Type: binary.BigEndian.Uint32(src[:4])}

	switch dst.Type {
	case AuthTypeOk:
	case AuthTypeCleartextPassword:
	case AuthTypeMD5Password:
		copy(dst.Salt[:], src[4:8])
	case AuthTypeSASL:
		authMechanisms := src[4:]
		for len(authMechanisms) > 1 {
			idx := bytes.IndexByte(authMechanisms, 0)
			if idx > 0 {
				dst.SASLAuthMechanisms = append(dst.SASLAuthMechanisms, string(authMechanisms[:idx]))
				authMechanisms = authMechanisms[idx+1:]
			}
		}
	case AuthTypeSASLContinue, AuthTypeSASLFinal:
		dst.SASLData = src[4:]
	default:
		return errors.Errorf("unknown authentication type: %d", dst.Type)
	}

	return nil
}

func (src *Authentication) Encode(dst []byte) []byte {
	dst = append(dst, 'R')
	sp := len(dst)
	dst = pgio.AppendInt32(dst, -1)
	dst = pgio.AppendUint32(dst, src.Type)

	switch src.Type {
	case AuthTypeMD5Password:
		dst = append(dst, src.Salt[:]...)
	case AuthTypeSASL:
		for _, s := range src.SASLAuthMechanisms {
			dst = append(dst, []byte(s)...)
			dst = append(dst, 0)
		}
		dst = append(dst, 0)
	case AuthTypeSASLContinue:
		dst = pgio.AppendInt32(dst, int32(len(src.SASLData)))
		dst = append(dst, src.SASLData...)
	}

	pgio.SetInt32(dst[sp:], int32(len(dst[sp:])))

	return dst
}
