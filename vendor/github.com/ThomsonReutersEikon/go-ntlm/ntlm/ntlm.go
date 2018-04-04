//Copyright 2013 Thomson Reuters Global Resources. BSD License please see License file for more information

// Package NTLM implements the interfaces used for interacting with NTLMv1 and NTLMv2.
// To create NTLM v1 or v2 sessions you would use CreateClientSession and create ClientServerSession.
package ntlm

import (
	rc4P "crypto/rc4"
	"errors"
)

type Version int

const (
	Version1 Version = 1
	Version2 Version = 2
)

type Mode int

const (
	ConnectionlessMode Mode = iota
	ConnectionOrientedMode
)

// Creates an NTLM v1 or v2 client
// mode - This must be ConnectionlessMode or ConnectionOrientedMode depending on what type of NTLM is used
// version - This must be Version1 or Version2 depending on the version of NTLM used
func CreateClientSession(version Version, mode Mode) (n ClientSession, err error) {
	switch version {
	case Version1:
		n = new(V1ClientSession)
	case Version2:
		n = new(V2ClientSession)
	default:
		return nil, errors.New("Unknown NTLM Version, must be 1 or 2")
	}

	return n, nil
}

type ClientSession interface {
	SetUserInfo(username string, password string, domain string)
	SetMode(mode Mode)

	GenerateNegotiateMessage() (*NegotiateMessage, error)
	ProcessChallengeMessage(*ChallengeMessage) error
	GenerateAuthenticateMessage() (*AuthenticateMessage, error)

	Seal(message []byte) ([]byte, error)
	Sign(message []byte) ([]byte, error)
	Mac(message []byte, sequenceNumber int) ([]byte, error)
	VerifyMac(message, expectedMac []byte, sequenceNumber int) (bool, error)
}

// Creates an NTLM v1 or v2 server
// mode - This must be ConnectionlessMode or ConnectionOrientedMode depending on what type of NTLM is used
// version - This must be Version1 or Version2 depending on the version of NTLM used
func CreateServerSession(version Version, mode Mode) (n ServerSession, err error) {
	switch version {
	case Version1:
		n = new(V1ServerSession)
	case Version2:
		n = new(V2ServerSession)
	default:
		return nil, errors.New("Unknown NTLM Version, must be 1 or 2")
	}

	n.SetMode(mode)
	return n, nil
}

type ServerSession interface {
	SetUserInfo(username string, password string, domain string)
	GetUserInfo() (string, string, string)

	SetMode(mode Mode)
	SetServerChallenge(challenge []byte)

	ProcessNegotiateMessage(*NegotiateMessage) error
	GenerateChallengeMessage() (*ChallengeMessage, error)
	ProcessAuthenticateMessage(*AuthenticateMessage) error

	GetSessionData() *SessionData

	Version() int
	Seal(message []byte) ([]byte, error)
	Sign(message []byte) ([]byte, error)
	Mac(message []byte, sequenceNumber int) ([]byte, error)
	VerifyMac(message, expectedMac []byte, sequenceNumber int) (bool, error)
}

// This struct collects NTLM data structures and keys that are used across all types of NTLM requests
type SessionData struct {
	mode Mode

	user       string
	password   string
	userDomain string

	NegotiateFlags uint32

	negotiateMessage    *NegotiateMessage
	challengeMessage    *ChallengeMessage
	authenticateMessage *AuthenticateMessage

	serverChallenge     []byte
	clientChallenge     []byte
	ntChallengeResponse []byte
	lmChallengeResponse []byte

	responseKeyLM             []byte
	responseKeyNT             []byte
	exportedSessionKey        []byte
	encryptedRandomSessionKey []byte
	keyExchangeKey            []byte
	sessionBaseKey            []byte
	mic                       []byte

	ClientSigningKey []byte
	ServerSigningKey []byte
	ClientSealingKey []byte
	ServerSealingKey []byte

	clientHandle *rc4P.Cipher
	serverHandle *rc4P.Cipher
}
