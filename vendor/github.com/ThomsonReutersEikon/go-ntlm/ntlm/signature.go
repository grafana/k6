//Copyright 2013 Thomson Reuters Global Resources. BSD License please see License file for more information

package ntlm

import (
	rc4P "crypto/rc4"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

type NtlmsspMessageSignature struct {
	ByteData []byte
	// A 32-bit unsigned integer that contains the signature version. This field MUST be 0x00000001.
	Version []byte
	// A 4-byte array that contains the random pad for the message.
	RandomPad []byte
	// A 4-byte array that contains the checksum for the message.
	CheckSum []byte
	// A 32-bit unsigned integer that contains the NTLM sequence number for this application message.
	SeqNum []byte
}

func (n *NtlmsspMessageSignature) String() string {
	return fmt.Sprintf("NtlmsspMessageSignature: %s", hex.EncodeToString(n.Bytes()))
}

func (n *NtlmsspMessageSignature) Bytes() []byte {
	if n.ByteData != nil {
		return n.ByteData
	} else {
		return concat(n.Version, n.RandomPad, n.CheckSum, n.SeqNum)
	}
	return nil
}

// Define SEAL(Handle, SigningKey, SeqNum, Message) as
func seal(negFlags uint32, handle *rc4P.Cipher, signingKey []byte, seqNum uint32, message []byte) (sealedMessage []byte, sig *NtlmsspMessageSignature) {
	sealedMessage = rc4(handle, message)
	sig = mac(negFlags, handle, signingKey, uint32(seqNum), message)
	return
}

// Define SIGN(Handle, SigningKey, SeqNum, Message) as
func sign(negFlags uint32, handle *rc4P.Cipher, signingKey []byte, seqNum uint32, message []byte) []byte {
	return concat(message, mac(negFlags, handle, signingKey, uint32(seqNum), message).Bytes())
}

func mac(negFlags uint32, handle *rc4P.Cipher, signingKey []byte, seqNum uint32, message []byte) (result *NtlmsspMessageSignature) {
	if NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY.IsSet(negFlags) {
		result = macWithExtendedSessionSecurity(negFlags, handle, signingKey, seqNum, message)
	} else {
		result = macWithoutExtendedSessionSecurity(handle, seqNum, message)
	}
	return result
}

// Define MAC(Handle, SigningKey, SeqNum, Message) as
// Set NTLMSSP_MESSAGE_SIGNATURE.Version to 0x00000001
// Set NTLMSSP_MESSAGE_SIGNATURE.Checksum to CRC32(Message)
// Set NTLMSSP_MESSAGE_SIGNATURE.RandomPad RC4(Handle, RandomPad)
// Set NTLMSSP_MESSAGE_SIGNATURE.Checksum to RC4(Handle, NTLMSSP_MESSAGE_SIGNATURE.Checksum)
// Set NTLMSSP_MESSAGE_SIGNATURE.SeqNum to RC4(Handle, 0x00000000)
// If (connection oriented)
//   Set NTLMSSP_MESSAGE_SIGNATURE.SeqNum to NTLMSSP_MESSAGE_SIGNATURE.SeqNum XOR SeqNum
//   Set SeqNum to SeqNum + 1
// Else
//   Set NTLMSSP_MESSAGE_SIGNATURE.SeqNum to NTLMSSP_MESSAGE_SIGNATURE.SeqNum XOR (application supplied SeqNum)
// EndIf
// Set NTLMSSP_MESSAGE_SIGNATURE.RandomPad to 0
// End
func macWithoutExtendedSessionSecurity(handle *rc4P.Cipher, seqNum uint32, message []byte) *NtlmsspMessageSignature {
	sig := new(NtlmsspMessageSignature)

	seqNumBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(seqNumBytes, seqNum)

	sig.Version = []byte{0x01, 0x00, 0x00, 0x00}
	sig.CheckSum = make([]byte, 4)
	binary.LittleEndian.PutUint32(sig.CheckSum, crc32(message))
	sig.RandomPad = rc4(handle, zeroBytes(4))
	sig.CheckSum = rc4(handle, sig.CheckSum)
	sig.SeqNum = rc4(handle, zeroBytes(4))
	for i := 0; i < 4; i++ {
		sig.SeqNum[i] = sig.SeqNum[i] ^ seqNumBytes[i]
	}
	sig.RandomPad = zeroBytes(4)
	return sig
}

// Define MAC(Handle, SigningKey, SeqNum, Message) as
// Set NTLMSSP_MESSAGE_SIGNATURE.Version to 0x00000001
// if Key Exchange Key Negotiated
//   Set NTLMSSP_MESSAGE_SIGNATURE.Checksum to RC4(Handle, HMAC_MD5(SigningKey, ConcatenationOf(SeqNum, Message))[0..7])
// else
//   Set NTLMSSP_MESSAGE_SIGNATURE.Checksum to HMAC_MD5(SigningKey, ConcatenationOf(SeqNum, Message))[0..7]
// end
// Set NTLMSSP_MESSAGE_SIGNATURE.SeqNum to SeqNum
// Set SeqNum to SeqNum + 1
// EndDefine
func macWithExtendedSessionSecurity(negFlags uint32, handle *rc4P.Cipher, signingKey []byte, seqNum uint32, message []byte) *NtlmsspMessageSignature {
	sig := new(NtlmsspMessageSignature)
	sig.Version = []byte{0x01, 0x00, 0x00, 0x00}
	seqNumBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(seqNumBytes, seqNum)
	sig.CheckSum = hmacMd5(signingKey, concat(seqNumBytes, message))[0:8]
	if NTLMSSP_NEGOTIATE_KEY_EXCH.IsSet(negFlags) {
		sig.CheckSum = rc4(handle, sig.CheckSum)
	}
	sig.SeqNum = seqNumBytes
	return sig
}

func reinitSealingKey(key []byte, sequenceNumber int) (handle *rc4P.Cipher, err error) {
	seqNumBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(seqNumBytes, uint32(sequenceNumber))
	newKey := md5(concat(key, seqNumBytes))
	handle, err = rc4Init(newKey)
	return handle, err
}
