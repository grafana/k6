/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package crypto

import (
	"fmt"
	"testing"

	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
)

func TestDecrypt(t *testing.T) {
	if testing.Short() {
		return
	}
	rt := makeRuntime()

	t.Run("InvalidKey", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const ciphertext = %s;
		const recipient = { type: "HyperQuantumAlgorithm" };
		crypto.decrypt(recipient, ciphertext);
		`, material.pkcsCiphertext))
		assert.EqualError(t, err, "GoError: invalid private key")
	})

	t.Run("RSA-PKCS", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const recipient = x509.parsePrivateKey(%s);
		const ciphertext = %s;
		const message = crypto.decrypt(recipient, ciphertext, "hex");
		const expected = %s;
		if (message !== expected) {
			throw new Error("Decrypted incorrect message");
		}`,
			material.rsaPrivateKey,
			material.pkcsCiphertext,
			material.messageHex,
		))
		assert.NoError(t, err)
	})

	t.Run("RSA-OAEP", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const recipient = x509.parsePrivateKey(%s);
		const ciphertext = %s;
		const options = { type: "oaep" };
		const message = crypto.decrypt(recipient, ciphertext, "hex", options);
		const expected = %s;
		if (message !== expected) {
			throw new Error("Decrypted incorrect message");
		}`,
			material.rsaPrivateKey,
			material.oaepCiphertext,
			material.messageHex,
		))
		assert.NoError(t, err)
	})

	t.Run("RSA-OAEP-Label", func(t *testing.T) {
		_, err := common.RunString(rt, fmt.Sprintf(`
		const recipient = x509.parsePrivateKey(%s);
		const ciphertext = %s
		const options = { type: "oaep", label: %s };
		const message = crypto.decrypt(recipient, ciphertext, "hex", options);
		const expected = %s;
		if (message !== expected) {
			throw new Error("Decrypted incorrect message");
		}`,
			material.rsaPrivateKey,
			material.oaepLabeledCiphertext,
			material.labelString,
			material.messageHex,
		))
		assert.NoError(t, err)
	})
}
