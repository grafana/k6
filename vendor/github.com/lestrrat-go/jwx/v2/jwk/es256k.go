//go:build jwx_es256k
// +build jwx_es256k

package jwk

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lestrrat-go/jwx/v2/internal/ecutil"
	"github.com/lestrrat-go/jwx/v2/jwa"
)

func init() {
	ecutil.RegisterCurve(secp256k1.S256(), jwa.Secp256k1)
}
