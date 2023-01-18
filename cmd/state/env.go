package state

import "strings"

// ParseEnvKeyValue splits an environment variable string into key and value.
func ParseEnvKeyValue(kv string) (string, string) {
	if idx := strings.IndexRune(kv, '='); idx != -1 {
		return kv[:idx], kv[idx+1:]
	}
	return kv, ""
}

// BuildEnvMap returns a map from raw environment values, such as returned from
// os.Environ().
func BuildEnvMap(environ []string) map[string]string {
	env := make(map[string]string, len(environ))
	for _, kv := range environ {
		k, v := ParseEnvKeyValue(kv)
		env[k] = v
	}
	return env
}
