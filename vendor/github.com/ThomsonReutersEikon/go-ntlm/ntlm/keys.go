//Copyright 2013 Thomson Reuters Global Resources. BSD License please see License file for more information

package ntlm

// Define KXKEY(SessionBaseKey, LmChallengeResponse, ServerChallenge) as
func kxKey(flags uint32, sessionBaseKey []byte, lmChallengeResponse []byte, serverChallenge []byte, lmnowf []byte) (keyExchangeKey []byte, err error) {
	if NTLMSSP_NEGOTIATE_LM_KEY.IsSet(flags) {
		var part1, part2 []byte
		part1, err = des(lmnowf[0:7], lmChallengeResponse[0:8])
		if err != nil {
			return nil, err
		}

		key := append([]byte{lmnowf[7]}, []byte{0xBD, 0xBD, 0xBD, 0xBD, 0xBD, 0xBD}...)
		part2, err = des(key, lmChallengeResponse[0:8])
		if err != nil {
			return nil, err
		}

		keyExchangeKey = concat(part1, part2)
	} else if NTLMSSP_REQUEST_NON_NT_SESSION_KEY.IsSet(flags) {
		keyExchangeKey = concat(lmnowf[0:8], zeroBytes(8))
	} else {
		keyExchangeKey = sessionBaseKey
	}

	return
}

// Define SIGNKEY(NegFlg, RandomSessionKey, Mode) as
func signKey(flags uint32, randomSessionKey []byte, mode string) (signKey []byte) {
	if NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY.IsSet(flags) {
		if mode == "Client" {
			signKey = md5(concat(randomSessionKey, []byte("session key to client-to-server signing key magic constant\x00")))
		} else {
			signKey = md5(concat(randomSessionKey, []byte("session key to server-to-client signing key magic constant\x00")))
		}
	} else {
		signKey = nil
	}
	return
}

// 	Define SEALKEY(NegotiateFlags, RandomSessionKey, Mode) as
func sealKey(flags uint32, randomSessionKey []byte, mode string) (sealKey []byte) {
	if NTLMSSP_NEGOTIATE_EXTENDED_SESSIONSECURITY.IsSet(flags) {
		if NTLMSSP_NEGOTIATE_128.IsSet(flags) {
			sealKey = randomSessionKey
		} else if NTLMSSP_NEGOTIATE_56.IsSet(flags) {
			sealKey = randomSessionKey[0:7]
		} else {
			sealKey = randomSessionKey[0:5]
		}
		if mode == "Client" {
			sealKey = md5(concat(sealKey, []byte("session key to client-to-server sealing key magic constant\x00")))
		} else {
			sealKey = md5(concat(sealKey, []byte("session key to server-to-client sealing key magic constant\x00")))
		}
	} else if NTLMSSP_NEGOTIATE_LM_KEY.IsSet(flags) {
		if NTLMSSP_NEGOTIATE_56.IsSet(flags) {
			sealKey = concat(randomSessionKey[0:7], []byte{0xA0})
		} else {
			sealKey = concat(randomSessionKey[0:5], []byte{0xE5, 0x38, 0xB0})
		}
	} else {
		sealKey = randomSessionKey
	}

	return
}
