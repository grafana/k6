package webcrypto

// okpKeyImporters holds the functions importing an Octet Key Pair (OKP) key, as used by the
// Ed25519 and X25519 algorithms, from each of the supported formats.
type okpKeyImporters struct {
	spki  func(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error)
	pkcs8 func(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error)
	jwk   func(keyData []byte, keyUsages []CryptoKeyUsage, extractable bool) (any, CryptoKeyType, error)
	raw   func(keyData []byte, keyUsages []CryptoKeyUsage) (any, CryptoKeyType, error)
}

// importOKPKey imports an OKP key in the given format, using the matching importer.
func importOKPKey(
	algorithm Algorithm,
	importers okpKeyImporters,
	format KeyFormat,
	keyData []byte,
	extractable bool,
	keyUsages []CryptoKeyUsage,
) (*CryptoKey, error) {
	var (
		handle  any
		keyType CryptoKeyType
		err     error
	)

	switch format {
	case SpkiKeyFormat:
		handle, keyType, err = importers.spki(keyData, keyUsages)
	case Pkcs8KeyFormat:
		handle, keyType, err = importers.pkcs8(keyData, keyUsages)
	case JwkKeyFormat:
		handle, keyType, err = importers.jwk(keyData, keyUsages, extractable)
	case RawKeyFormat:
		handle, keyType, err = importers.raw(keyData, keyUsages)
	default:
		return nil, NewError(NotSupportedError, unsupportedKeyFormatErrorMsg+" "+format+" for algorithm "+algorithm.Name)
	}

	if err != nil {
		return nil, err
	}

	return &CryptoKey{
		Algorithm:   algorithm,
		Extractable: extractable,
		Usages:      keyUsages,
		handle:      handle,
		Type:        keyType,
	}, nil
}
