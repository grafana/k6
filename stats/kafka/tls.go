package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

func NewTLSConfig(clientCertFilePath, clientKeyFilePath, caCertFilePath string) (*tls.Config, error) {
	//Load client certificate
	if clientCertFilePath == "" || clientKeyFilePath == "" {
		return nil, fmt.Errorf("client cert or client key must not be empty")
	}

	cert, err := tls.LoadX509KeyPair(clientCertFilePath, clientKeyFilePath)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	//Load CA certificate
	if caCertFilePath != "" {
		caCert, err := ioutil.ReadFile(caCertFilePath)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}
