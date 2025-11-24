// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Protocol details from https://msdn.microsoft.com/en-us/library/cc236621.aspx,
// implementation hints from http://davenport.sourceforge.net/ntlm.html .
// This package only implements authentication, no key exchange or encryption. It
// only supports Unicode (UTF16LE) encoding of protocol strings, no OEM encoding.
// This package implements NTLMv2.
package ntlmssp

import (
	"crypto/hmac"
	"crypto/md5"
	"strings"

	"github.com/Azure/go-ntlmssp/internal/md4"
)

func getNtlmV2Hash(password, username, domain string) []byte {
	return getNtlmV2Hashed(getNtlmHash(password), username, domain)
}

func getNtlmV2Hashed(ntlmHash []byte, username, domain string) []byte {
	return hmacMd5(ntlmHash, toUnicode(strings.ToUpper(username)+domain))
}

func getNtlmHash(password string) []byte {
	hash := md4.New()
	hash.Write(toUnicode(password))
	return hash.Sum(nil)
}

func computeNtlmV2Response(ntlmV2Hash, serverChallenge, clientChallenge,
	timestamp, targetInfo []byte,
) []byte {
	temp := []byte{1, 1, 0, 0, 0, 0, 0, 0}
	temp = append(temp, timestamp...)
	temp = append(temp, clientChallenge...)
	temp = append(temp, 0, 0, 0, 0)
	temp = append(temp, targetInfo...)
	temp = append(temp, 0, 0, 0, 0)

	NTProofStr := hmacMd5(ntlmV2Hash, serverChallenge, temp)
	return append(NTProofStr, temp...)
}

func computeLmV2Response(ntlmV2Hash, serverChallenge, clientChallenge []byte) []byte {
	return append(hmacMd5(ntlmV2Hash, serverChallenge, clientChallenge), clientChallenge...)
}

func hmacMd5(key []byte, data ...[]byte) []byte {
	mac := hmac.New(md5.New, key)
	for _, d := range data {
		mac.Write(d)
	}
	return mac.Sum(nil)
}
