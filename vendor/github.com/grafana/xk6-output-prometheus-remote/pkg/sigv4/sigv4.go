// Package sigv4 is responsible to for aws sigv4 signing of requests
package sigv4

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type signer interface {
	sign(req *http.Request) error
}

type defaultSigner struct {
	config *Config

	// noEscape represents the characters that AWS doesn't escape
	noEscape [256]bool

	ignoredHeaders map[string]struct{}
}

func newDefaultSigner(config *Config) signer {
	ds := &defaultSigner{
		config:   config,
		noEscape: buildAwsNoEscape(),
		ignoredHeaders: map[string]struct{}{
			"Authorization":   {},
			"User-Agent":      {},
			"X-Amzn-Trace-Id": {},
			"Expect":          {},
		},
	}

	return ds
}

func (d *defaultSigner) sign(req *http.Request) error {
	now := time.Now().UTC()
	iSO8601Date := now.Format(timeFormat)

	credentialScope := buildCredentialScope(now, d.config.Region)

	payloadHash, err := d.getPayloadHash(req)
	if err != nil {
		return err
	}

	req.Header.Set("Host", req.Host)
	req.Header.Set(amzDateKey, iSO8601Date)
	req.Header.Set(contentSHAKey, payloadHash)

	signedHeadersStr, canonicalHeaderStr := buildCanonicalHeaders(req, d.ignoredHeaders)

	canonicalQueryString := getCanonicalQueryString(req.URL)
	canonicalReq := buildCanonicalString(
		req.Method,
		getCanonicalURI(req.URL, d.noEscape),
		canonicalQueryString,
		canonicalHeaderStr,
		signedHeadersStr,
		payloadHash,
	)

	signature := sign(
		deriveKey(d.config.AwsSecretAccessKey, d.config.Region),
		buildStringToSign(iSO8601Date, credentialScope, canonicalReq),
	)

	authorizationHeader := fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		signingAlgorithm,
		d.config.AwsAccessKeyID,
		credentialScope,
		signedHeadersStr,
		signature,
	)

	req.URL.RawQuery = canonicalQueryString
	req.Header.Set(authorizationHeaderKey, authorizationHeader)
	return nil
}

func (d *defaultSigner) getPayloadHash(req *http.Request) (string, error) {
	if req.Body == nil {
		return emptyStringSHA256, nil
	}

	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	reqBodyBuffer := bytes.NewReader(reqBody)

	hash := sha256.New()
	if _, err := io.Copy(hash, reqBodyBuffer); err != nil {
		return "", err
	}

	payloadHash := hex.EncodeToString(hash.Sum(nil))

	// ensuring that we keep the request body intact for next tripper
	req.Body = io.NopCloser(bytes.NewReader(reqBody))

	return payloadHash, nil
}

func buildCredentialScope(signingTime time.Time, region string) string {
	return fmt.Sprintf(
		"%s/%s/%s/aws4_request",
		signingTime.UTC().Format(shortTimeFormat),
		region,
		awsServiceName,
	)
}

func buildCanonicalString(method, uri, query, canonicalHeaders, signedHeaders, payloadHash string) string {
	return strings.Join([]string{
		method,
		uri,
		query,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
}

// buildCanonicalHeaders is mostly ported from https://github.com/aws/aws-sdk-go-v2/aws/signer/v4 buildCanonicalHeaders
func buildCanonicalHeaders(
	req *http.Request,
	ignoredHeaders map[string]struct{},
) (signedHeaders, canonicalHeadersStr string) {
	const hostHeader, contentLengthHeader = "host", "content-length"
	host, header, length := req.Host, req.Header, req.ContentLength

	signed := make(http.Header)
	headers := append([]string{}, hostHeader)
	signed[hostHeader] = append(signed[hostHeader], host)

	if length > 0 {
		headers = append(headers, contentLengthHeader)
		signed[contentLengthHeader] = append(signed[contentLengthHeader], strconv.FormatInt(length, 10))
	}

	for k, v := range header {
		if _, ok := ignoredHeaders[k]; ok {
			continue
		}

		if strings.EqualFold(k, contentLengthHeader) {
			// prevent signing already handled content-length header.
			continue
		}

		lowerCaseKey := strings.ToLower(k)
		if _, ok := signed[lowerCaseKey]; ok {
			// include additional values
			signed[lowerCaseKey] = append(signed[lowerCaseKey], v...)
			continue
		}

		headers = append(headers, lowerCaseKey)
		signed[lowerCaseKey] = v
	}

	// aws requires headers to keys to be sorted
	sort.Strings(headers)
	signedHeaders = strings.Join(headers, ";")

	var canonicalHeaders strings.Builder
	for _, h := range headers {
		if h == hostHeader {
			canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", hostHeader, stripExcessSpaces(host)))
			continue
		}

		canonicalHeaders.WriteString(fmt.Sprintf("%s:", h))
		values := signed[h]
		for j, v := range values {
			cleanedValue := strings.TrimSpace(stripExcessSpaces(v))
			canonicalHeaders.WriteString(cleanedValue)
			if j < len(values)-1 {
				canonicalHeaders.WriteRune(',')
			}
		}
		canonicalHeaders.WriteRune('\n')
	}
	canonicalHeadersStr = canonicalHeaders.String()
	return signedHeaders, canonicalHeadersStr
}

func getCanonicalURI(u *url.URL, noEscape [256]bool) string {
	return escapePath(getURIPath(u), noEscape)
}

func getCanonicalQueryString(u *url.URL) string {
	query := u.Query()

	// Sort Each Query Key's Values
	for key := range query {
		sort.Strings(query[key])
	}

	var rawQuery strings.Builder
	rawQuery.WriteString(strings.ReplaceAll(query.Encode(), "+", "%20"))
	return rawQuery.String()
}

func buildStringToSign(amzDate, credentialScope, canonicalRequestString string) string {
	hash := sha256.New()
	hash.Write([]byte(canonicalRequestString))
	return strings.Join([]string{
		signingAlgorithm,
		amzDate,
		credentialScope,
		hex.EncodeToString(hash.Sum(nil)),
	}, "\n")
}

func deriveKey(secretKey, region string) string {
	signingDate := time.Now().UTC().Format(shortTimeFormat)
	hmacDate := hmacSHA256([]byte("AWS4"+secretKey), signingDate)
	hmacRegion := hmacSHA256(hmacDate, region)
	hmacService := hmacSHA256(hmacRegion, awsServiceName)
	signingKey := hmacSHA256(hmacService, "aws4_request")
	return string(signingKey)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sign(signingKey string, strToSign string) string {
	h := hmac.New(sha256.New, []byte(signingKey))
	h.Write([]byte(strToSign))
	sig := hex.EncodeToString(h.Sum(nil))
	return sig
}
