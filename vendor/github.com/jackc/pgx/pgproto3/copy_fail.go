package pgproto3

import (
	"bytes"
	"encoding/json"

	"github.com/jackc/pgx/pgio"
)

type CopyFail struct {
	Message string
}

func (*CopyFail) Backend() {}

func (dst *CopyFail) Decode(src []byte) error {
	idx := bytes.IndexByte(src, 0)
	if idx != len(src)-1 {
		return &invalidMessageFormatErr{messageType: "CopyFail"}
	}

	dst.Message = string(src[:idx])

	return nil
}

func (src *CopyFail) Encode(dst []byte) []byte {
	dst = append(dst, 'f')
	sp := len(dst)
	dst = pgio.AppendInt32(dst, -1)

	dst = append(dst, src.Message...)
	dst = append(dst, 0)

	pgio.SetInt32(dst[sp:], int32(len(dst[sp:])))

	return dst
}

func (src *CopyFail) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type       string
		Message string
	}{
		Type:       "CopyFail",
		Message: src.Message,
	})
}
