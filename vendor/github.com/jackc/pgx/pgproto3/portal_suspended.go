package pgproto3

import (
	"encoding/json"
)

type PortalSuspended struct{}

func (*PortalSuspended) Backend() {}

func (dst *PortalSuspended) Decode(src []byte) error {
	if len(src) != 0 {
		return &invalidMessageLenErr{messageType: "PortalSuspended", expectedLen: 0, actualLen: len(src)}
	}

	return nil
}

func (src *PortalSuspended) Encode(dst []byte) []byte {
	return append(dst, 's', 0, 0, 0, 4)
}

func (src *PortalSuspended) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string
	}{
		Type: "PortalSuspended",
	})
}
