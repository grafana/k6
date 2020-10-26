/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package lib

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

// DefaultScenarioName is used as the default key/ID of the scenario config entries
// that were created due to the use of the shortcut execution control options (i.e. duration+vus,
// iterations+vus, or stages)
const DefaultScenarioName = "default"

// DefaultSummaryTrendStats are the default trend columns shown in the test summary output
// nolint: gochecknoglobals
var DefaultSummaryTrendStats = []string{"avg", "min", "med", "max", "p(90)", "p(95)"}

// Describes a TLS version. Serialised to/from JSON as a string, eg. "tls1.2".
type TLSVersion int

func (v TLSVersion) MarshalJSON() ([]byte, error) {
	return []byte(`"` + SupportedTLSVersionsToString[v] + `"`), nil
}

func (v *TLSVersion) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" {
		*v = 0
		return nil
	}
	ver, ok := SupportedTLSVersions[str]
	if !ok {
		return errors.Errorf("unknown TLS version: %s", str)
	}
	*v = ver
	return nil
}

// Fields for TLSVersions. Unmarshalling hack.
type TLSVersionsFields struct {
	Min TLSVersion `json:"min" ignored:"true"` // Minimum allowed version, 0 = any.
	Max TLSVersion `json:"max" ignored:"true"` // Maximum allowed version, 0 = any.
}

// Describes a set (min/max) of TLS versions.
type TLSVersions TLSVersionsFields

func (v *TLSVersions) UnmarshalJSON(data []byte) error {
	var fields TLSVersionsFields
	if err := json.Unmarshal(data, &fields); err != nil {
		var ver TLSVersion
		if err2 := json.Unmarshal(data, &ver); err2 != nil {
			return err
		}
		fields.Min = ver
		fields.Max = ver
	}
	*v = TLSVersions(fields)
	return nil
}

func (v *TLSVersions) isTLS13() bool {
	return v.Min == TLSVersion13 || v.Max == TLSVersion13
}

// A list of TLS cipher suites.
// Marshals and unmarshals from a list of names, eg. "TLS_ECDHE_RSA_WITH_RC4_128_SHA".
type TLSCipherSuites []uint16

// MarshalJSON will return the JSON representation according to supported TLS cipher suites
func (s *TLSCipherSuites) MarshalJSON() ([]byte, error) {
	var suiteNames []string
	for _, id := range *s {
		if suiteName, ok := SupportedTLSCipherSuitesToString[id]; ok {
			suiteNames = append(suiteNames, suiteName)
		} else {
			return nil, errors.Errorf("Unknown cipher suite id: %d", id)
		}
	}

	return json.Marshal(suiteNames)
}

func (s *TLSCipherSuites) UnmarshalJSON(data []byte) error {
	var suiteNames []string
	if err := json.Unmarshal(data, &suiteNames); err != nil {
		return err
	}

	var suiteIDs []uint16
	for _, name := range suiteNames {
		if suiteID, ok := SupportedTLSCipherSuites[name]; ok {
			suiteIDs = append(suiteIDs, suiteID)
		} else {
			return errors.New("Unknown cipher suite: " + name)
		}
	}

	*s = suiteIDs

	return nil
}

// Fields for TLSAuth. Unmarshalling hack.
type TLSAuthFields struct {
	// Certificate and key as a PEM-encoded string, including "-----BEGIN CERTIFICATE-----".
	Cert string `json:"cert"`
	Key  string `json:"key"`

	// Domains to present the certificate to. May contain wildcards, eg. "*.example.com".
	Domains []string `json:"domains"`
}

// Defines a TLS client certificate to present to certain hosts.
type TLSAuth struct {
	TLSAuthFields
	certificate *tls.Certificate
}

func (c *TLSAuth) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &c.TLSAuthFields); err != nil {
		return err
	}
	if _, err := c.Certificate(); err != nil {
		return err
	}
	return nil
}

func (c *TLSAuth) Certificate() (*tls.Certificate, error) {
	if c.certificate == nil {
		cert, err := tls.X509KeyPair([]byte(c.Cert), []byte(c.Key))
		if err != nil {
			return nil, err
		}
		c.certificate = &cert
	}
	return c.certificate, nil
}

// IPNet is a wrapper around net.IPNet for JSON unmarshalling
type IPNet struct {
	net.IPNet
}

// UnmarshalText populates the IPNet from the given CIDR
func (ipnet *IPNet) UnmarshalText(b []byte) error {
	newIPNet, err := ParseCIDR(string(b))
	if err != nil {
		return errors.Wrap(err, "Failed to parse CIDR")
	}

	*ipnet = *newIPNet

	return nil
}

// HostAddress stores information about IP and port
// for a host.
type HostAddress net.TCPAddr

// NewHostAddress creates a pointer to a new address with an IP object.
func NewHostAddress(ip net.IP, portString string) (*HostAddress, error) {
	var port int
	if portString != "" {
		var err error
		if port, err = strconv.Atoi(portString); err != nil {
			return nil, err
		}
	}

	return &HostAddress{
		IP:   ip,
		Port: port,
	}, nil
}

// String converts a HostAddress into a string.
func (h *HostAddress) String() string {
	return (*net.TCPAddr)(h).String()
}

// MarshalText implements the encoding.TextMarshaler interface.
// The encoding is the same as returned by String, with one exception:
// When len(ip) is zero, it returns an empty slice.
func (h *HostAddress) MarshalText() ([]byte, error) {
	if h == nil || len(h.IP) == 0 {
		return []byte(""), nil
	}

	if len(h.IP) != net.IPv4len && len(h.IP) != net.IPv6len {
		return nil, &net.AddrError{Err: "invalid IP address", Addr: h.IP.String()}
	}

	return []byte(h.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The IP address is expected in a form accepted by ParseIP.
func (h *HostAddress) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return &net.ParseError{Type: "IP address", Text: "<nil>"}
	}

	ip, port, err := splitHostPort(text)
	if err != nil {
		return err
	}

	nh, err := NewHostAddress(ip, port)
	if err != nil {
		return err
	}

	*h = *nh

	return nil
}

func splitHostPort(text []byte) (net.IP, string, error) {
	host, port, err := net.SplitHostPort(string(text))
	if err != nil {
		// This error means that there is no port.
		// Make host the full text.
		host = string(text)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil, "", &net.ParseError{Type: "IP address", Text: host}
	}

	return ip, port, nil
}

// ParseCIDR creates an IPNet out of a CIDR string
func ParseCIDR(s string) (*IPNet, error) {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}

	parsedIPNet := IPNet{IPNet: *ipnet}

	return &parsedIPNet, nil
}

type Options struct {
	// Should the test start in a paused state?
	Paused null.Bool `json:"paused" envconfig:"K6_PAUSED"`

	// Initial values for VUs, max VUs, duration cap, iteration cap, and stages.
	// See the Runner or Executor interfaces for more information.
	VUs        null.Int           `json:"vus" envconfig:"K6_VUS"`
	Duration   types.NullDuration `json:"duration" envconfig:"K6_DURATION"`
	Iterations null.Int           `json:"iterations" envconfig:"K6_ITERATIONS"`
	Stages     []Stage            `json:"stages" envconfig:"K6_STAGES"`

	// TODO: remove the `ignored:"true"` from the field tags, it's there so that
	// the envconfig library will ignore those fields.
	//
	// We should support specifying execution segments via environment
	// variables, but we currently can't, because envconfig has this nasty bug
	// (among others): https://github.com/kelseyhightower/envconfig/issues/113
	Scenarios                ScenarioConfigs           `json:"scenarios,omitempty" ignored:"true"`
	ExecutionSegment         *ExecutionSegment         `json:"executionSegment" ignored:"true"`
	ExecutionSegmentSequence *ExecutionSegmentSequence `json:"executionSegmentSequence" ignored:"true"`

	// Timeouts for the setup() and teardown() functions
	NoSetup         null.Bool          `json:"noSetup" envconfig:"NO_SETUP"`
	SetupTimeout    types.NullDuration `json:"setupTimeout" envconfig:"K6_SETUP_TIMEOUT"`
	NoTeardown      null.Bool          `json:"noTeardown" envconfig:"NO_TEARDOWN"`
	TeardownTimeout types.NullDuration `json:"teardownTimeout" envconfig:"K6_TEARDOWN_TIMEOUT"`

	// Limit HTTP requests per second.
	RPS null.Int `json:"rps" envconfig:"K6_RPS"`

	// DNS handling configuration.
	DNS types.DNSConfig `json:"dns" envconfig:"K6_DNS"`

	// How many HTTP redirects do we follow?
	MaxRedirects null.Int `json:"maxRedirects" envconfig:"K6_MAX_REDIRECTS"`

	// Default User Agent string for HTTP requests.
	UserAgent null.String `json:"userAgent" envconfig:"K6_USER_AGENT"`

	// How many batch requests are allowed in parallel, in total and per host?
	Batch        null.Int `json:"batch" envconfig:"K6_BATCH"`
	BatchPerHost null.Int `json:"batchPerHost" envconfig:"K6_BATCH_PER_HOST"`

	// Should all HTTP requests and responses be logged (excluding body)?
	HTTPDebug null.String `json:"httpDebug" envconfig:"K6_HTTP_DEBUG"`

	// Accept invalid or untrusted TLS certificates.
	InsecureSkipTLSVerify null.Bool `json:"insecureSkipTLSVerify" envconfig:"K6_INSECURE_SKIP_TLS_VERIFY"`

	// Specify TLS versions and cipher suites, and present client certificates.
	TLSCipherSuites *TLSCipherSuites `json:"tlsCipherSuites" envconfig:"K6_TLS_CIPHER_SUITES"`
	TLSVersion      *TLSVersions     `json:"tlsVersion" ignored:"true"`
	TLSAuth         []*TLSAuth       `json:"tlsAuth" envconfig:"K6_TLSAUTH"`

	// Throw warnings (eg. failed HTTP requests) as errors instead of simply logging them.
	Throw null.Bool `json:"throw" envconfig:"K6_THROW"`

	// Define thresholds; these take the form of 'metric=["snippet1", "snippet2"]'.
	// To create a threshold on a derived metric based on tag queries ("submetrics"), create a
	// metric on a nonexistent metric named 'real_metric{tagA:valueA,tagB:valueB}'.
	Thresholds map[string]stats.Thresholds `json:"thresholds" envconfig:"K6_THRESHOLDS"`

	// Blacklist IP ranges that tests may not contact. Mainly useful in hosted setups.
	BlacklistIPs []*IPNet `json:"blacklistIPs" envconfig:"K6_BLACKLIST_IPS"`

	// Block hostname patterns that tests may not contact.
	BlockedHostnames types.NullHostnameTrie `json:"blockHostnames" envconfig:"K6_BLOCK_HOSTNAMES"`

	// Hosts overrides dns entries for given hosts
	Hosts map[string]*HostAddress `json:"hosts" envconfig:"K6_HOSTS"`

	// Disable keep-alive connections
	NoConnectionReuse null.Bool `json:"noConnectionReuse" envconfig:"K6_NO_CONNECTION_REUSE"`

	// Do not reuse connections between VU iterations. This gives more realistic results (depending
	// on what you're looking for), but you need to raise various kernel limits or you'll get
	// errors about running out of file handles or sockets, or being unable to bind addresses.
	NoVUConnectionReuse null.Bool `json:"noVUConnectionReuse" envconfig:"K6_NO_VU_CONNECTION_REUSE"`

	// MinIterationDuration can be used to force VUs to pause between iterations if a specific
	// iteration is shorter than the specified value.
	MinIterationDuration types.NullDuration `json:"minIterationDuration" envconfig:"K6_MIN_ITERATION_DURATION"`

	// These values are for third party collectors' benefit.
	// Can't be set through env vars.
	External map[string]json.RawMessage `json:"ext" ignored:"true"`

	// Summary trend stats for trend metrics (response times) in CLI output
	SummaryTrendStats []string `json:"summaryTrendStats" envconfig:"K6_SUMMARY_TREND_STATS"`

	// Summary time unit for summary metrics (response times) in CLI output
	SummaryTimeUnit null.String `json:"summaryTimeUnit" envconfig:"K6_SUMMARY_TIME_UNIT"`

	// Which system tags to include with metrics ("method", "vu" etc.)
	// Use pointer for identifying whether user provide any tag or not.
	SystemTags *stats.SystemTagSet `json:"systemTags" envconfig:"K6_SYSTEM_TAGS"`

	// Tags to be applied to all samples for this running
	RunTags *stats.SampleTags `json:"tags" envconfig:"K6_TAGS"`

	// Buffer size of the channel for metric samples; 0 means unbuffered
	MetricSamplesBufferSize null.Int `json:"metricSamplesBufferSize" envconfig:"K6_METRIC_SAMPLES_BUFFER_SIZE"`

	// Do not reset cookies after a VU iteration
	NoCookiesReset null.Bool `json:"noCookiesReset" envconfig:"K6_NO_COOKIES_RESET"`

	// Discard Http Responses Body
	DiscardResponseBodies null.Bool `json:"discardResponseBodies" envconfig:"K6_DISCARD_RESPONSE_BODIES"`

	// Redirect console logging to a file
	ConsoleOutput null.String `json:"-" envconfig:"K6_CONSOLE_OUTPUT"`

	// Specify client IP ranges and/or CIDR from which VUs will make requests
	LocalIPs types.NullIPPool `json:"-" envconfig:"K6_LOCAL_IPS"`
}

// Returns the result of overwriting any fields with any that are set on the argument.
//
// Example:
//   a := Options{VUs: null.IntFrom(10), VUsMax: null.IntFrom(10)}
//   b := Options{VUs: null.IntFrom(5)}
//   a.Apply(b) // Options{VUs: null.IntFrom(5), VUsMax: null.IntFrom(10)}
func (o Options) Apply(opts Options) Options {
	if opts.Paused.Valid {
		o.Paused = opts.Paused
	}
	if opts.VUs.Valid {
		o.VUs = opts.VUs
	}

	// Specifying duration, iterations, stages, or execution in a "higher" config tier
	// will overwrite all of the the previous execution settings (if any) from any
	// "lower" config tiers
	// Still, if more than one of those options is simultaneously specified in the same
	// config tier, they will be preserved, so the validation after we've consolidated
	// all of the options can return an error.
	if opts.Duration.Valid || opts.Iterations.Valid || opts.Stages != nil || opts.Scenarios != nil {
		// TODO: emit a warning or a notice log message if overwrite lower tier config options?
		o.Duration = types.NewNullDuration(0, false)
		o.Iterations = null.NewInt(0, false)
		o.Stages = nil
		o.Scenarios = nil
	}

	if opts.Duration.Valid {
		o.Duration = opts.Duration
	}
	if opts.Iterations.Valid {
		o.Iterations = opts.Iterations
	}
	if opts.Stages != nil {
		o.Stages = []Stage{}
		for _, s := range opts.Stages {
			if s.Duration.Valid {
				o.Stages = append(o.Stages, s)
			}
		}
	}
	// o.Execution can also be populated by the duration/iterations/stages config shortcuts, but
	// that happens after the configuration from the different sources is consolidated. It can't
	// happen here, because something like `K6_ITERATIONS=10 k6 run --vus 5 script.js` wont't
	// work correctly at this level.
	if opts.Scenarios != nil {
		o.Scenarios = opts.Scenarios
	}
	if opts.ExecutionSegment != nil {
		o.ExecutionSegment = opts.ExecutionSegment
	}

	if opts.ExecutionSegmentSequence != nil {
		o.ExecutionSegmentSequence = opts.ExecutionSegmentSequence
	}
	if opts.NoSetup.Valid {
		o.NoSetup = opts.NoSetup
	}
	if opts.SetupTimeout.Valid {
		o.SetupTimeout = opts.SetupTimeout
	}
	if opts.NoTeardown.Valid {
		o.NoTeardown = opts.NoTeardown
	}
	if opts.TeardownTimeout.Valid {
		o.TeardownTimeout = opts.TeardownTimeout
	}
	if opts.RPS.Valid {
		o.RPS = opts.RPS
	}
	if opts.MaxRedirects.Valid {
		o.MaxRedirects = opts.MaxRedirects
	}
	if opts.UserAgent.Valid {
		o.UserAgent = opts.UserAgent
	}
	if opts.Batch.Valid {
		o.Batch = opts.Batch
	}
	if opts.BatchPerHost.Valid {
		o.BatchPerHost = opts.BatchPerHost
	}
	if opts.HTTPDebug.Valid {
		o.HTTPDebug = opts.HTTPDebug
	}
	if opts.InsecureSkipTLSVerify.Valid {
		o.InsecureSkipTLSVerify = opts.InsecureSkipTLSVerify
	}
	if opts.TLSCipherSuites != nil {
		o.TLSCipherSuites = opts.TLSCipherSuites
	}
	if opts.TLSVersion != nil {
		o.TLSVersion = opts.TLSVersion
		if o.TLSVersion.isTLS13() {
			enableTLS13()
		}
	}
	if opts.TLSAuth != nil {
		o.TLSAuth = opts.TLSAuth
	}
	if opts.Throw.Valid {
		o.Throw = opts.Throw
	}
	if opts.Thresholds != nil {
		o.Thresholds = opts.Thresholds
	}
	if opts.BlacklistIPs != nil {
		o.BlacklistIPs = opts.BlacklistIPs
	}
	if opts.BlockedHostnames.Valid {
		o.BlockedHostnames = opts.BlockedHostnames
	}
	if opts.Hosts != nil {
		o.Hosts = opts.Hosts
	}
	if opts.NoConnectionReuse.Valid {
		o.NoConnectionReuse = opts.NoConnectionReuse
	}
	if opts.NoVUConnectionReuse.Valid {
		o.NoVUConnectionReuse = opts.NoVUConnectionReuse
	}
	if opts.MinIterationDuration.Valid {
		o.MinIterationDuration = opts.MinIterationDuration
	}
	if opts.NoCookiesReset.Valid {
		o.NoCookiesReset = opts.NoCookiesReset
	}
	if opts.External != nil {
		o.External = opts.External
	}
	if opts.SummaryTrendStats != nil {
		o.SummaryTrendStats = opts.SummaryTrendStats
	}
	if opts.SummaryTimeUnit.Valid {
		o.SummaryTimeUnit = opts.SummaryTimeUnit
	}
	if opts.SystemTags != nil {
		o.SystemTags = opts.SystemTags
	}
	if !opts.RunTags.IsEmpty() {
		o.RunTags = opts.RunTags
	}
	if opts.MetricSamplesBufferSize.Valid {
		o.MetricSamplesBufferSize = opts.MetricSamplesBufferSize
	}
	if opts.DiscardResponseBodies.Valid {
		o.DiscardResponseBodies = opts.DiscardResponseBodies
	}
	if opts.ConsoleOutput.Valid {
		o.ConsoleOutput = opts.ConsoleOutput
	}
	if opts.LocalIPs.Valid {
		o.LocalIPs = opts.LocalIPs
	}
	if opts.DNS.TTL.Valid {
		o.DNS.TTL = opts.DNS.TTL
	}
	if opts.DNS.Select.Valid {
		o.DNS.Select = opts.DNS.Select
	}
	if opts.DNS.Policy.Valid {
		o.DNS.Policy = opts.DNS.Policy
	}

	return o
}

// Validate checks if all of the specified options make sense
func (o Options) Validate() []error {
	// TODO: validate all of the other options... that we should have already been validating...
	// TODO: maybe integrate an external validation lib: https://github.com/avelino/awesome-go#validation
	var errors []error
	if o.ExecutionSegmentSequence != nil {
		var segmentFound bool
		for _, segment := range *o.ExecutionSegmentSequence {
			if o.ExecutionSegment.Equal(segment) {
				segmentFound = true
				break
			}
		}
		if !segmentFound {
			errors = append(errors,
				fmt.Errorf("provided segment %s can't be found in sequence %s",
					o.ExecutionSegment, o.ExecutionSegmentSequence))
		}
	}
	return append(errors, o.Scenarios.Validate()...)
}

// ForEachSpecified enumerates all struct fields and calls the supplied function with each
// element that is valid. It panics for any unfamiliar or unexpected fields, so make sure
// new fields in Options are accounted for.
func (o Options) ForEachSpecified(structTag string, callback func(key string, value interface{})) {
	structType := reflect.TypeOf(o)
	structVal := reflect.ValueOf(o)
	for i := 0; i < structType.NumField(); i++ {
		fieldType := structType.Field(i)
		fieldVal := structVal.Field(i)
		value := fieldVal.Interface()

		shouldCall := false
		switch fieldType.Type.Kind() {
		case reflect.Struct:
			// Unpack any guregu/null values
			shouldCall = fieldVal.FieldByName("Valid").Bool()
			valOrZero := fieldVal.MethodByName("ValueOrZero")
			if shouldCall && valOrZero.IsValid() {
				value = valOrZero.Call([]reflect.Value{})[0].Interface()
				if v, ok := value.(types.Duration); ok {
					value = v.String()
				}
			}
		case reflect.Slice:
			shouldCall = fieldVal.Len() > 0
		case reflect.Map:
			shouldCall = fieldVal.Len() > 0
		case reflect.Ptr:
			shouldCall = !fieldVal.IsNil()
		default:
			panic(fmt.Sprintf("Unknown Options field %#v", fieldType))
		}

		if shouldCall {
			key, ok := fieldType.Tag.Lookup(structTag)
			if !ok {
				key = fieldType.Name
			}

			callback(key, value)
		}
	}
}
