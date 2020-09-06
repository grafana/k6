/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

//GetAbsolutelyFilePath Gets the local file with absolutely path
func GetAbsolutelyFilePath(path string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// If input file path is not absolutely filepath, let's join it with current working directory
	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, strings.Trim(path, "."))
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	//
	if info.IsDir() {
		return "", fmt.Errorf("%v is is not a file", path)
	}

	return path, nil
}

//ReadFile reads stream from file path
func ReadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	// validate file's size
	fileInfo, _ := f.Stat()
	size := fileInfo.Size()
	// throw error if it's a empty file
	if size == 0 {
		return nil, fmt.Errorf("file %v is empty", path)
	}
	// load file content to buffer
	buffer := make([]byte, size)
	f.Read(buffer)

	return buffer, nil
}

//NewTLS Creates TLS Certificate to authenticate with Kafka Cluster (requires PEM format)
func NewTLS(tlsClientCert, tlsClientKey, tlsClientCA string, skipVerify bool) (*tls.Config, error) {
	//Load client certificate
	if tlsClientCert == "" || tlsClientKey == "" {
		return nil, fmt.Errorf("client cert or client key must not be empty")
	}

	cert, err := tls.LoadX509KeyPair(tlsClientCert, tlsClientKey)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	//Load CA certificate
	if tlsClientCA != "" {
		caCert, err := ioutil.ReadFile(tlsClientCA)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	tlsConfig.InsecureSkipVerify = skipVerify

	return tlsConfig, nil
}
