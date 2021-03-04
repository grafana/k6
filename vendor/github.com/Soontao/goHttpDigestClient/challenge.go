package goHttpDigestClient

import (
	"fmt"
	"strings"
)

const (
	KEY_DIGEST           = "Digest"
	KEY_AUTH_SCHEMA      = "auth_schema"
	KEY_QOP              = "qop"
	KEY_NONCE            = "nonce"
	KEY_CNONCE           = "cnonce"
	KEY_USERNAME         = "username"
	KEY_NONCE_COUNT      = "nc"
	KEY_OPAQUE           = "opaque"
	KEY_RESPONSE         = "response"
	KEY_REALM            = "realm"
	KEY_AUTHORIZATION    = "Authorization"
	KEY_URI              = "uri"
	KEY_WWW_Authenticate = "WWW-Authenticate"
)

//The 401 (Unauthorized) response message is used by an origin server
//to challenge the authorization of a user agent.
//
// And the CHALLENGE will include informations about auth
type Challenge map[string]string

func NewChallenge(wwwAuthHeader string) Challenge {
	r := Challenge{}
	wwwAuthArr := strings.Split(strings.Replace(wwwAuthHeader, ",", "", -1), " ")
	wwwAuthArrLen := len(wwwAuthArr)
	if wwwAuthArrLen > 1 {
		r[KEY_AUTH_SCHEMA] = wwwAuthArr[0]
		for i := 1; i < wwwAuthArrLen; i++ {
			itemArr := strings.Split(wwwAuthArr[i], "=")
			r.SetChallengeItem(itemArr[0], itemArr[1])
		}
	}
	return r
}

func (info Challenge) IsDigestAuth() bool {
	return info[KEY_AUTH_SCHEMA] == KEY_DIGEST
}

func (info Challenge) SetChallengeItem(itemKey string, itemValue string) {
	info[itemKey] = itemValue
}

func (info Challenge) GetChallengeItemPure(itemKey string) string {
	return strings.Replace(info[itemKey], `"`, "", -1)
}

// some specific key, will add qutation mark
func (info Challenge) GetChallengeItemFormat(itemKey string) string {
	r := info.GetChallengeItemPure(itemKey)
	switch itemKey {
	case KEY_QOP, KEY_NONCE_COUNT:
		return r
	default:
		return fmt.Sprintf(`"%s"`, r)
	}
}

// format challenge header to authorization header
//
// MAYBE you should computeResponseFirst()
func (info Challenge) ToAuthorizationStr() string {
	auth_schema := KEY_DIGEST
	authorization_content := ""
	// how to specify the sequence
	for k, _ := range info {
		if k != KEY_AUTH_SCHEMA {
			authorization_content += fmt.Sprintf(", %s=%s", k, info.GetChallengeItemFormat(k))
		}
	}
	return auth_schema + strings.Replace(authorization_content, ",", "", 1)
}

// base challenge to compute the response, and the response will be checking by server
func (h Challenge) ComputeResponse(method, uri, entity, username, password string) Challenge {
	qop := h.GetChallengeItemPure(KEY_QOP)
	realm := h.GetChallengeItemPure(KEY_REALM)
	nonce := h.GetChallengeItemPure(KEY_NONCE)
	nonceCount := h.GetChallengeItemPure(KEY_NONCE_COUNT)
	cNonce := h.GetChallengeItemPure(KEY_CNONCE)
	response, cNonce, nonceCount := computeResponse(qop, realm, nonce, nonceCount, cNonce, method, uri, entity, username, password)
	h.SetChallengeItem(KEY_USERNAME, username)
	h.SetChallengeItem(KEY_URI, uri)
	h.SetChallengeItem(KEY_CNONCE, cNonce)
	h.SetChallengeItem(KEY_NONCE_COUNT, nonceCount)
	h.SetChallengeItem(KEY_RESPONSE, response)
	return h
}
