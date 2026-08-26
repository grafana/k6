package kafka

// Config structs decoded from the JS-side objects declared in index.d.ts. JSON
// tags match the contract's field names; the k6 field-name mapper uses them when
// exporting sobek values into these structs.

// SASLConfig configures SASL authentication.
type SASLConfig struct {
	Username   string `js:"username"`
	Password   string `js:"password"` //nolint:gosec // config field name, not a hardcoded credential
	Algorithm  string `js:"algorithm"`
	AWSProfile string `js:"awsProfile"`
}

// TLSConfig configures a TLS connection.
type TLSConfig struct {
	EnableTLS             bool   `js:"enableTls"`
	InsecureSkipTLSVerify bool   `js:"insecureSkipTlsVerify"`
	MinVersion            string `js:"minVersion"`
	ClientCertPem         string `js:"clientCertPem"`
	ClientKeyPem          string `js:"clientKeyPem"`
	ServerCaPem           string `js:"serverCaPem"`
}

// ConnectionConfig configures a Connection.
type ConnectionConfig struct {
	Address string      `js:"address"`
	SASL    *SASLConfig `js:"sasl"`
	TLS     *TLSConfig  `js:"tls"`
}

// JKSConfig configures loading a Java KeyStore.
type JKSConfig struct {
	Path     string `js:"path"`
	Password string `js:"password"` //nolint:gosec // config field name, not a hardcoded credential
	// ClientCertAlias is accepted for contract compatibility but not used: the
	// client certificate chain comes from the private-key entry (ClientKeyAlias).
	ClientCertAlias   string `js:"clientCertAlias"`
	ClientKeyAlias    string `js:"clientKeyAlias"`
	ClientKeyPassword string `js:"clientKeyPassword"`
	ServerCaAlias     string `js:"serverCaAlias"`
}

// JKS is the PEM material extracted from a Java KeyStore.
type JKS struct {
	ClientCertsPem []string `js:"clientCertsPem"`
	ClientKeyPem   string   `js:"clientKeyPem"`
	ServerCaPem    string   `js:"serverCaPem"`
}
