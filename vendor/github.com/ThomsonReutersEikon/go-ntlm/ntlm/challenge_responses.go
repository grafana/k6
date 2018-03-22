//Copyright 2013 Thomson Reuters Global Resources. BSD License please see License file for more information

package ntlm

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
)

// NTLMv1
// ******
type NtlmV1Response struct {
	// 24 byte array
	Response []byte
}

func (n *NtlmV1Response) String() string {
	return fmt.Sprintf("NtlmV1Response: %s", hex.EncodeToString(n.Response))
}

func ReadNtlmV1Response(bytes []byte) (*NtlmV1Response, error) {
	r := new(NtlmV1Response)
	r.Response = bytes[0:24]
	return r, nil
}

// *** NTLMv2
// The NTLMv2_CLIENT_CHALLENGE structure defines the client challenge in the AUTHENTICATE_MESSAGE.
// This structure is used only when NTLM v2 authentication is configured.
type NtlmV2ClientChallenge struct {
	// An 8-bit unsigned char that contains the current version of the challenge response type.
	// This field MUST be 0x01.
	RespType byte
	// An 8-bit unsigned char that contains the maximum supported version of the challenge response type.
	// This field MUST be 0x01.
	HiRespType byte
	// A 16-bit unsigned integer that SHOULD be 0x0000 and MUST be ignored on receipt.
	Reserved1 uint16
	// A 32-bit unsigned integer that SHOULD be 0x00000000 and MUST be ignored on receipt.
	Reserved2 uint32
	// A 64-bit unsigned integer that contains the current system time, represented as the number of 100 nanosecond
	// ticks elapsed since midnight of January 1, 1601 (UTC).
	TimeStamp []byte
	// An 8-byte array of unsigned char that contains the client's ClientChallenge (section 3.1.5.1.2).
	ChallengeFromClient []byte
	// A 32-bit unsigned integer that SHOULD be 0x00000000 and MUST be ignored on receipt.
	Reserved3 uint32
	AvPairs   *AvPairs
}

func (n *NtlmV2ClientChallenge) String() string {
	var buffer bytes.Buffer

	buffer.WriteString("NTLM v2 ClientChallenge\n")
	buffer.WriteString(fmt.Sprintf("Timestamp: %s\n", hex.EncodeToString(n.TimeStamp)))
	buffer.WriteString(fmt.Sprintf("ChallengeFromClient: %s\n", hex.EncodeToString(n.ChallengeFromClient)))
	buffer.WriteString("AvPairs\n")
	buffer.WriteString(n.AvPairs.String())

	return buffer.String()
}

// The NTLMv2_RESPONSE structure defines the NTLMv2 authentication NtChallengeResponse in the AUTHENTICATE_MESSAGE.
// This response is used only when NTLMv2 authentication is configured.
type NtlmV2Response struct {
	// A 16-byte array of unsigned char that contains the client's NT challenge- response as defined in section 3.3.2.
	// Response corresponds to the NTProofStr variable from section 3.3.2.
	Response []byte
	// A variable-length byte array that contains the ClientChallenge as defined in section 3.3.2.
	// ChallengeFromClient corresponds to the temp variable from section 3.3.2.
	NtlmV2ClientChallenge *NtlmV2ClientChallenge
}

func (n *NtlmV2Response) String() string {
	var buffer bytes.Buffer

	buffer.WriteString("NTLM v2 Response\n")
	buffer.WriteString(fmt.Sprintf("Response: %s\n", hex.EncodeToString(n.Response)))
	buffer.WriteString(n.NtlmV2ClientChallenge.String())

	return buffer.String()
}

func ReadNtlmV2Response(bytes []byte) (*NtlmV2Response, error) {
	r := new(NtlmV2Response)
	r.Response = bytes[0:16]
	r.NtlmV2ClientChallenge = new(NtlmV2ClientChallenge)
	c := r.NtlmV2ClientChallenge
	c.RespType = bytes[16]
	c.HiRespType = bytes[17]

	if c.RespType != 1 || c.HiRespType != 1 {
		return nil, errors.New("Does not contain a valid NTLM v2 client challenge - could be NTLMv1.")
	}

	// Ignoring - 2 bytes reserved
	// c.Reserved1
	// Ignoring - 4 bytes reserved
	// c.Reserved2
	c.TimeStamp = bytes[24:32]
	c.ChallengeFromClient = bytes[32:40]
	// Ignoring - 4 bytes reserved
	// c.Reserved3
	c.AvPairs = ReadAvPairs(bytes[44:])
	return r, nil
}

// LMv1
// ****
type LmV1Response struct {
	// 24 bytes
	Response []byte
}

func ReadLmV1Response(bytes []byte) *LmV1Response {
	r := new(LmV1Response)
	r.Response = bytes[0:24]
	return r
}

func (l *LmV1Response) String() string {
	return fmt.Sprintf("LmV1Response: %s", hex.EncodeToString(l.Response))
}

// *** LMv2
type LmV2Response struct {
	// A 16-byte array of unsigned char that contains the client's LM challenge-response.
	// This is the portion of the LmChallengeResponse field to which the HMAC_MD5 algorithm
	/// has been applied, as defined in section 3.3.2. Specifically, Response corresponds
	// to the result of applying the HMAC_MD5 algorithm, using the key ResponseKeyLM, to a
	// message consisting of the concatenation of the ResponseKeyLM, ServerChallenge and ClientChallenge.
	Response []byte
	// An 8-byte array of unsigned char that contains the client's ClientChallenge, as defined in section 3.1.5.1.2.
	ChallengeFromClient []byte
}

func ReadLmV2Response(bytes []byte) *LmV2Response {
	r := new(LmV2Response)
	r.Response = bytes[0:16]
	r.ChallengeFromClient = bytes[16:24]
	return r
}

func (l *LmV2Response) String() string {
	var buffer bytes.Buffer

	buffer.WriteString("LM v2 Response\n")
	buffer.WriteString(fmt.Sprintf("Response: %s\n", hex.EncodeToString(l.Response)))
	buffer.WriteString(fmt.Sprintf("ChallengeFromClient: %s\n", hex.EncodeToString(l.ChallengeFromClient)))

	return buffer.String()
}
