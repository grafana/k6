package platform

// SanitizeSeparator sanitizes the path separator in the given buffer.
func SanitizeSeparator(in []byte) {
	for i := range in {
		if in[i] == '\\' {
			in[i] = '/'
		}
	}
}
