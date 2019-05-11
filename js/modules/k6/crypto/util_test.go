package crypto

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/crypto/x509"
)

func bytes(encoded string) []byte {
	decoded, _ := hex.DecodeString(encoded)
	return decoded
}

func enhex(value []byte) string {
	return hex.EncodeToString(value)
}

func stringify(value string) string {
	return fmt.Sprintf(`"%s"`, value)
}

func template(value string) string {
	return fmt.Sprintf("`%s`", value)
}

func makeRuntime() *goja.Runtime {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("crypto", common.Bind(rt, New(), &ctx))
	rt.Set("x509", common.Bind(rt, x509.New(), &ctx))
	return rt
}
