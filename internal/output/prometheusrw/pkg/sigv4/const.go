package sigv4

const (
	// Amazon Managed Service for Prometheus
	awsServiceName = "aps"

	signingAlgorithm = "AWS4-HMAC-SHA256"

	authorizationHeaderKey = "Authorization"
	amzDateKey             = "X-Amz-Date"

	// emptyStringSHA256 is the hex encoded sha256 value of an empty string
	emptyStringSHA256 = `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`

	// timeFormat is the time format to be used in the X-Amz-Date header or query parameter
	timeFormat = "20060102T150405Z"

	// shortTimeFormat is the shorten time format used in the credential scope
	shortTimeFormat = "20060102"

	// contentSHAKey is the SHA256 of request body
	contentSHAKey = "X-Amz-Content-Sha256"
)
