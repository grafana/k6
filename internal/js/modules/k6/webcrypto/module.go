// Package webcrypto exports the webcrypto API.
package webcrypto

import (
	"fmt"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

const cryptoGlobalIdentifier = "crypto"

type (
	// RootModule is the global module instance that will create Client
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the JS module.
	ModuleInstance struct {
		vu modules.VU
	}
)

// Ensure the interfaces are implemented correctly
var (
	_ modules.Instance = &ModuleInstance{}
	_ modules.Module   = &RootModule{}
)

// New returns a pointer to a new RootModule instance
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface and returns
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{
		vu: vu,
	}
}

// Exports implements the modules.Instance interface and returns
// the exports of the JS module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]interface{}{
		"crypto": mi.vu.Runtime().GlobalObject().Get(cryptoGlobalIdentifier),
	}}
}

// SetupGlobally sets the crypto object globally.
func SetupGlobally(vu modules.VU) error {
	if err := vu.Runtime().Set(cryptoGlobalIdentifier, newCryptoObject(vu)); err != nil {
		return fmt.Errorf("unable to set crypto object globally; reason: %w", err)
	}

	return nil
}

func newCryptoObject(vu modules.VU) *sobek.Object {
	rt := vu.Runtime()

	obj := rt.NewObject()

	crypto := &Crypto{
		vu:        vu,
		Subtle:    &SubtleCrypto{vu: vu},
		CryptoKey: &CryptoKey{},
	}

	if err := setReadOnlyPropertyOf(obj, "getRandomValues", rt.ToValue(crypto.GetRandomValues)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "randomUUID", rt.ToValue(crypto.RandomUUID)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "subtle", rt.ToValue(newSubtleCryptoObject(vu))); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "CryptoKey", rt.ToValue(crypto.CryptoKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	return obj
}

func newSubtleCryptoObject(vu modules.VU) *sobek.Object {
	rt := vu.Runtime()

	obj := rt.NewObject()

	subtleCrypto := &SubtleCrypto{vu: vu}

	if err := setReadOnlyPropertyOf(obj, "decrypt", rt.ToValue(subtleCrypto.Decrypt)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "deriveBits", rt.ToValue(subtleCrypto.DeriveBits)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "deriveKey", rt.ToValue(subtleCrypto.DeriveKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "digest", rt.ToValue(subtleCrypto.Digest)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "encrypt", rt.ToValue(subtleCrypto.Encrypt)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "exportKey", rt.ToValue(subtleCrypto.ExportKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "generateKey", rt.ToValue(subtleCrypto.GenerateKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "importKey", rt.ToValue(subtleCrypto.ImportKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "sign", rt.ToValue(subtleCrypto.Sign)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "unwrapKey", rt.ToValue(subtleCrypto.UnwrapKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "verify", rt.ToValue(subtleCrypto.Verify)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	if err := setReadOnlyPropertyOf(obj, "wrapKey", rt.ToValue(subtleCrypto.WrapKey)); err != nil {
		common.Throw(rt, NewError(ImplementationError, err.Error()))
	}

	return obj
}

// setReadOnlyPropertyOf sets a read-only property on the given [sobek.Object].
func setReadOnlyPropertyOf(obj *sobek.Object, name string, value sobek.Value) error {
	err := obj.DefineDataProperty(name,
		value,
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	)
	if err != nil {
		return fmt.Errorf("unable to define %s read-only property on TextEncoder object; reason: %w", name, err)
	}

	return nil
}
