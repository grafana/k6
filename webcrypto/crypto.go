package webcrypto

import "go.k6.io/k6/js/modules"

type Crypto struct {
	vu modules.VU

	Subtle *SubtleCrypto
}

func (c *Crypto) GetRandomValues() {
	// TODO
}

func (c *Crypto) RandomUUID() {
	// TODO
}
