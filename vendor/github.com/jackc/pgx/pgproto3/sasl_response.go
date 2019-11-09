package pgproto3

import (
	"encoding/hex"
	"encoding/json"

	"github.com/jackc/pgx/pgio"
)

type SASLResponse struct {
	Data []byte
}

func (*SASLResponse) Frontend() {}

func (dst *SASLResponse) Decode(src []byte) error {
	*dst = SASLResponse{Data: src}
	return nil
}

func (src *SASLResponse) Encode(dst []byte) []byte {
	dst = append(dst, 'p')
	dst = pgio.AppendInt32(dst, int32(4+len(src.Data)))

	dst = append(dst, src.Data...)

	return dst
}

func (src *SASLResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string
		Data string
	}{
		Type: "SASLResponse",
		Data: hex.EncodeToString(src.Data),
	})
}
