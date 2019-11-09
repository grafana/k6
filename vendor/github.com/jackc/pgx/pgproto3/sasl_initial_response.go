package pgproto3

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/pgio"
)

type SASLInitialResponse struct {
	AuthMechanism string
	Data          []byte
}

func (*SASLInitialResponse) Frontend() {}

func (dst *SASLInitialResponse) Decode(src []byte) error {
	*dst = SASLInitialResponse{}

	rp := 0

	idx := bytes.IndexByte(src, 0)
	if idx < 0 {
		return errors.New("invalid SASLInitialResponse")
	}

	dst.AuthMechanism = string(src[rp:idx])
	rp = idx + 1

	rp += 4 // The rest of the message is data so we can just skip the size
	dst.Data = src[rp:]

	return nil
}

func (src *SASLInitialResponse) Encode(dst []byte) []byte {
	dst = append(dst, 'p')
	sp := len(dst)
	dst = pgio.AppendInt32(dst, -1)

	dst = append(dst, []byte(src.AuthMechanism)...)
	dst = append(dst, 0)

	dst = pgio.AppendInt32(dst, int32(len(src.Data)))
	dst = append(dst, src.Data...)

	pgio.SetInt32(dst[sp:], int32(len(dst[sp:])))

	return dst
}

func (src *SASLInitialResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type          string
		AuthMechanism string
		Data          string
	}{
		Type:          "SASLInitialResponse",
		AuthMechanism: src.AuthMechanism,
		Data:          hex.EncodeToString(src.Data),
	})
}
