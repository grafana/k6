package webcrypto

type bitsDeriver func(CryptoKey, CryptoKey) ([]byte, error)

func newBitsDeriver(algName string) (bitsDeriver, error) {
	if algName == ECDH {
		return deriveBitsECDH, nil
	}

	return nil, NewError(NotSupportedError, "unsupported algorithm for derive bits: "+algName)
}
