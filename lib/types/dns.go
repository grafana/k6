package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/guregu/null.v3"
)

// DNSConfig is the DNS resolver configuration.
type DNSConfig struct {
	// If positive, defines how long DNS lookups should be returned from the cache.
	TTL null.String `json:"ttl"`
	// Select specifies the strategy to use when picking a single IP if more than one is returned for a host name.
	Select NullDNSSelect `json:"select"`
	// Policy specifies how to handle returning of IPv4 or IPv6 addresses.
	Policy NullDNSPolicy `json:"policy"`
	// FIXME: Valid is unused and is only added to satisfy some logic in
	// lib.Options.ForEachSpecified(), otherwise it would panic with
	// `reflect: call of reflect.Value.Bool on zero Value`.
	Valid bool `json:"-"`
}

// DefaultDNSConfig returns the default DNS configuration.
func DefaultDNSConfig() DNSConfig {
	return DNSConfig{
		TTL:    null.NewString("5m", false),
		Select: NullDNSSelect{DNSrandom, false},
		Policy: NullDNSPolicy{DNSpreferIPv4, false},
	}
}

// DNSPolicy specifies the preference for handling IP versions in DNS resolutions.
//
//go:generate enumer -type=DNSPolicy -trimprefix DNS -output dns_policy_gen.go
type DNSPolicy uint8

// These are lower camel cased since enumer doesn't support it as a transform option.
// See https://github.com/alvaroloes/enumer/pull/60 .
const (
	// DNSpreferIPv4 returns an IPv4 address if available, falling back to IPv6 otherwise.
	DNSpreferIPv4 DNSPolicy = iota + 1
	// DNSpreferIPv6 returns an IPv6 address if available, falling back to IPv4 otherwise.
	DNSpreferIPv6
	// DNSonlyIPv4 only returns an IPv4 address and the resolution will fail if no IPv4 address is found.
	DNSonlyIPv4
	// DNSonlyIPv6 only returns an IPv6 address and the resolution will fail if no IPv6 address is found.
	DNSonlyIPv6
	// DNSany returns any resolved address regardless of version.
	DNSany
)

// UnmarshalJSON converts JSON data to a valid DNSPolicy
func (d *DNSPolicy) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	v, err := DNSPolicyString(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// MarshalJSON returns the JSON representation of d.
func (d DNSPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// NullDNSPolicy is a nullable wrapper around DNSPolicy, required for the
// current configuration system.
type NullDNSPolicy struct {
	DNSPolicy
	Valid bool
}

// UnmarshalJSON converts JSON data to a valid NullDNSPolicy.
func (d *NullDNSPolicy) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		return nil
	}
	if err := json.Unmarshal(data, &d.DNSPolicy); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

// MarshalJSON returns the JSON representation of d.
func (d NullDNSPolicy) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte(`null`), nil
	}
	return json.Marshal(d.DNSPolicy)
}

// DNSSelect is the strategy to use when picking a single IP if more than one
// is returned for a host name.
//
//go:generate enumer -type=DNSSelect -trimprefix DNS -output dns_select_gen.go
type DNSSelect uint8

// These are lower camel cased since enumer doesn't support it as a transform option.
// See https://github.com/alvaroloes/enumer/pull/60 .
const (
	// DNSfirst returns the first IP from the response.
	DNSfirst DNSSelect = iota + 1
	// DNSroundRobin rotates the IP returned on each lookup.
	DNSroundRobin
	// DNSrandom returns a random IP from the response.
	DNSrandom
)

// UnmarshalJSON converts JSON data to a valid DNSSelect
func (d *DNSSelect) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	v, err := DNSSelectString(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// MarshalJSON returns the JSON representation of d.
func (d DNSSelect) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// NullDNSSelect is a nullable wrapper around DNSSelect, required for the
// current configuration system.
type NullDNSSelect struct {
	DNSSelect
	Valid bool
}

// UnmarshalJSON converts JSON data to a valid NullDNSSelect.
func (d *NullDNSSelect) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		return nil
	}
	if err := json.Unmarshal(data, &d.DNSSelect); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

// MarshalJSON returns the JSON representation of d.
func (d NullDNSSelect) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte(`null`), nil
	}
	return json.Marshal(d.DNSSelect)
}

// String implements fmt.Stringer.
func (c DNSConfig) String() string {
	return fmt.Sprintf("ttl=%s,select=%s,policy=%s",
		c.TTL.String, c.Select.String(), c.Policy.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *DNSConfig) UnmarshalJSON(data []byte) error {
	var s struct {
		TTL    null.String   `json:"ttl"`
		Select NullDNSSelect `json:"select"`
		Policy NullDNSPolicy `json:"policy"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	c.TTL = s.TTL
	c.Select = s.Select
	c.Policy = s.Policy
	return nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *DNSConfig) UnmarshalText(text []byte) error {
	if string(text) == DefaultDNSConfig().String() {
		*c = DefaultDNSConfig()
		return nil
	}
	values := strings.Split(string(text), ",")
	params := make(map[string]string, len(values))
	for _, value := range values {
		args := strings.SplitN(value, "=", 2)
		if len(args) != 2 {
			return fmt.Errorf("no value for key %s", value)
		}
		params[args[0]] = args[1]
	}
	return c.unmarshal(params)
}

func (c *DNSConfig) unmarshal(params map[string]string) error {
	for k, v := range params {
		switch k {
		case "policy":
			p, err := DNSPolicyString(v)
			if err != nil {
				return err
			}
			c.Policy.DNSPolicy = p
			c.Policy.Valid = true
		case "select":
			s, err := DNSSelectString(v)
			if err != nil {
				return err
			}
			c.Select.DNSSelect = s
			c.Select.Valid = true
		case "ttl":
			c.TTL = null.StringFrom(v)
		default:
			return fmt.Errorf("unknown DNS configuration field: %s", k)
		}
	}
	return nil
}
