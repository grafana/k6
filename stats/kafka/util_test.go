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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	CertificatePemTest = `-----BEGIN CERTIFICATE-----
MIIE7jCCAtYCCQC7Yg+y9BeOUjANBgkqhkiG9w0BAQsFADA5MQswCQYDVQQGEwJW
TjEOMAwGA1UECAwFSGFub2kxGjAYBgNVBAMMEXd3dy5rYWZrYS10bHMuY29tMB4X
DTIwMDQxNjA4MTAzNFoXDTIxMDQxNjA4MTAzNFowOTELMAkGA1UEBhMCVk4xDjAM
BgNVBAgMBUhhbm9pMRowGAYDVQQDDBF3d3cua2Fma2EtdGxzLmNvbTCCAiIwDQYJ
KoZIhvcNAQEBBQADggIPADCCAgoCggIBAMRfHv7aWSzpJskvgfEPLAsNt2mlMrsy
+yjQBucIX8v4FBv4SflVxoNk1epazdxMWB4SjWLSK3vSLdGcNfyoiUlwbAu7m106
m3BD80yxnyl+x1dwKfYDqLnt+KMibiU3zR2HZGFhP4HQdZ9b4DjOvME5t96b/d/m
8jNlSpvD5jH9HfCVghJLWN9dA3KHV6KGAXZFv50+KE6PNwMrDsx4eR8J4MN74NsY
narBKhnm7j6OvYkP+8e/lIHQ/vJNopcrmYtqzJNmPg9JZIjYhIl9xq53L1GsYVaD
Ef9gqV7UwsHDatE2XsvrRogGj1nPnImmoG4/ldLv/6sKS86B35OPaRshoxjoJ0P5
+3wM3Mc65FXVSHCMFts849Mj+sJ1PzmPN/w9DAxm26HepM9t2shSbvp5Rl7WdI58
G28bp29ah5WXcXok7bpmZu5hG9iS8KkpQoFji5vi5GqWX78de89sBhKriIPsKWZo
cGlU6LZvSh60keXct08mHZkoSvnWIUc+3IdMLfvMgz4JLKZChz0y+2JgPJWYBax0
6d7ip4P6mgRzmzv2pu+QxvRPZOEEYjh9thwjRZ1XIqgeHFCEM9C8iTb7ga65y5kP
fMgBFsTcPNAFaOt+bKZvTQDeErc5x2R3V4yrnFXAusXy1Y4leibIljkfjU2tJ9Zx
GGVLpkpG48j1AgMBAAEwDQYJKoZIhvcNAQELBQADggIBACwm36OvzSIUYUJ9s2x9
rfkgix7go1n8FWccaeuOm1yj+mICBnGKFbJNMaK82wvpHTwUAk7DB4nEMmxw6dmW
DqPBj1YpEd1PiWWEiLQ/RyI4dAhEsuZrQjbDeXnoSziqXF1oMUVSFhATQEYZWJQq
sqIsi5M0FfPxmQwN5ODw6OWZVV+pyl2XF4FWAGGuQLuFez8nc4x2lYoZ4F4udOdo
NpW6yojjlnC5jrG+j//3ypWTraBxkYm474ci3O9S6eJfWCrjbS8/jOFNBEfFURtf
OOcArbpc9QXcKP1izm3YsEr+pGIjUuhoi+LBbJMW0HqwXjvN0ZwOGfRAjsBql2ce
XvLNryhN8lbWFG9d3fo0rjX/sWiyPWTOXAEOEmI+fmJHvUWg7+Y5qI9UWojj+BVr
Wklvd/ejZrAufI3uOBMOIWHAXQRLAYlaElLtuKHlDonyra60Iz+fC+9qC/KwUgcU
tfiMRHKCYlX+58yzJ83Q/u/eFbTy44gGBmLx+SR8/lOcIjl525e0MTMjdCiB8zV9
gB6PhGFP0eOE27F7u3CKLUcQkLaESlK7TU0SbkvXJydXhUGgt/oreapWvb5jtC2H
LhcYyoMx3OZHorG9aeqeVUtTq0i6uTgI0VaUxYI2w+Evz/aGy/K7g0X7GYXOdF3Q
uVS8R520GTnZ1PW4RlY2Jpfp
-----END CERTIFICATE-----`

	PrivateKeyPemTest = `-----BEGIN PRIVATE KEY-----
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
EWRsC1qR8zzRIzw2b5OrE+TrZvp56tTr
-----END PRIVATE KEY-----`
)

func TestGetAbsolutelyFilePath(t *testing.T) {
	wd, err := os.Getwd()
	assert.Nil(t, err)
	// Relative path
	path, err := GetAbsolutelyFilePath("util.go")
	assert.Nil(t, err)
	want := filepath.Join(wd, "util.go")
	assert.Equal(t, want, path)
	// Absolutely path
	path, err = GetAbsolutelyFilePath(want)
	assert.Nil(t, err)
	assert.Equal(t, want, path)
}

func TestReadFile(t *testing.T) {
	f, err := os.Create("file.txt")
	defer func() {
		_ = os.Remove("file.txt")
	}()
	assert.Nil(t, err)
	_, err = f.WriteString("hello-world")
	assert.Nil(t, err)
	read, err := ReadFile("file.txt")
	assert.Nil(t, err)
	assert.Equal(t, string(read), "hello-world")
}

func TestNewTLS(t *testing.T) {
	cert, err := os.Create("cert.pem")
	assert.Nil(t, err)
	_, err = cert.WriteString(CertificatePemTest)
	assert.Nil(t, err)
	err = cert.Close()
	assert.Nil(t, err)

	private, err := os.Create("key.pem")
	assert.Nil(t, err)
	_, err = private.WriteString(PrivateKeyPemTest)
	assert.Nil(t, err)
	err = private.Close()
	assert.Nil(t, err)

	defer func() {
		_ = os.Remove("cert.pem")
		_ = os.Remove("key.pem")
	}()

	tls, err := NewTLS("cert.pem", "key.pem", "", false)
	assert.Nil(t, err)
	assert.Equal(t, tls.InsecureSkipVerify, false)
}
