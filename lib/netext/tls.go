package netext

import (
	"crypto/tls"

	"github.com/loadimpact/k6/lib"
	"golang.org/x/crypto/ocsp"
)

const (
	OCSP_STATUS_GOOD                   = "good"
	OCSP_STATUS_REVOKED                = "revoked"
	OCSP_STATUS_SERVER_FAILED          = "server_failed"
	OCSP_STATUS_UNKNOWN                = "unknown"
	OCSP_REASON_UNSPECIFIED            = "unspecified"
	OCSP_REASON_KEY_COMPROMISE         = "key_compromise"
	OCSP_REASON_CA_COMPROMISE          = "ca_compromise"
	OCSP_REASON_AFFILIATION_CHANGED    = "affiliation_changed"
	OCSP_REASON_SUPERSEDED             = "superseded"
	OCSP_REASON_CESSATION_OF_OPERATION = "cessation_of_operation"
	OCSP_REASON_CERTIFICATE_HOLD       = "certificate_hold"
	OCSP_REASON_REMOVE_FROM_CRL        = "remove_from_crl"
	OCSP_REASON_PRIVILEGE_WITHDRAWN    = "privilege_withdrawn"
	OCSP_REASON_AA_COMPROMISE          = "aa_compromise"
	SSL_3_0                            = "ssl3.0"
	TLS_1_0                            = "tls1.0"
	TLS_1_1                            = "tls1.1"
	TLS_1_2                            = "tls1.2"
)

type TLSInfo struct {
	Version     string
	CipherSuite string
}
type OCSP struct {
	ProducedAt, ThisUpdate, NextUpdate, RevokedAt int64
	RevocationReason                              string
	Status                                        string
}

func ParseTLSConnState(tlsState *tls.ConnectionState) (TLSInfo, OCSP) {
	tlsInfo := TLSInfo{}
	switch tlsState.Version {
	case tls.VersionSSL30:
		tlsInfo.Version = SSL_3_0
	case tls.VersionTLS10:
		tlsInfo.Version = TLS_1_0
	case tls.VersionTLS11:
		tlsInfo.Version = TLS_1_1
	case tls.VersionTLS12:
		tlsInfo.Version = TLS_1_2
	}

	tlsInfo.CipherSuite = lib.SupportedTLSCipherSuitesToString[tlsState.CipherSuite]
	ocspStapledRes := OCSP{Status: OCSP_STATUS_UNKNOWN}

	if ocspRes, err := ocsp.ParseResponse(tlsState.OCSPResponse, nil); err == nil {
		switch ocspRes.Status {
		case ocsp.Good:
			ocspStapledRes.Status = OCSP_STATUS_GOOD
		case ocsp.Revoked:
			ocspStapledRes.Status = OCSP_STATUS_REVOKED
		case ocsp.ServerFailed:
			ocspStapledRes.Status = OCSP_STATUS_SERVER_FAILED
		case ocsp.Unknown:
			ocspStapledRes.Status = OCSP_STATUS_UNKNOWN
		}
		switch ocspRes.RevocationReason {
		case ocsp.Unspecified:
			ocspStapledRes.RevocationReason = OCSP_REASON_UNSPECIFIED
		case ocsp.KeyCompromise:
			ocspStapledRes.RevocationReason = OCSP_REASON_KEY_COMPROMISE
		case ocsp.CACompromise:
			ocspStapledRes.RevocationReason = OCSP_REASON_CA_COMPROMISE
		case ocsp.AffiliationChanged:
			ocspStapledRes.RevocationReason = OCSP_REASON_AFFILIATION_CHANGED
		case ocsp.Superseded:
			ocspStapledRes.RevocationReason = OCSP_REASON_SUPERSEDED
		case ocsp.CessationOfOperation:
			ocspStapledRes.RevocationReason = OCSP_REASON_CESSATION_OF_OPERATION
		case ocsp.CertificateHold:
			ocspStapledRes.RevocationReason = OCSP_REASON_CERTIFICATE_HOLD
		case ocsp.RemoveFromCRL:
			ocspStapledRes.RevocationReason = OCSP_REASON_REMOVE_FROM_CRL
		case ocsp.PrivilegeWithdrawn:
			ocspStapledRes.RevocationReason = OCSP_REASON_PRIVILEGE_WITHDRAWN
		case ocsp.AACompromise:
			ocspStapledRes.RevocationReason = OCSP_REASON_AA_COMPROMISE
		}
		ocspStapledRes.ProducedAt = ocspRes.ProducedAt.Unix()
		ocspStapledRes.ThisUpdate = ocspRes.ThisUpdate.Unix()
		ocspStapledRes.NextUpdate = ocspRes.NextUpdate.Unix()
		ocspStapledRes.RevokedAt = ocspRes.RevokedAt.Unix()
	}

	return tlsInfo, ocspStapledRes
}
