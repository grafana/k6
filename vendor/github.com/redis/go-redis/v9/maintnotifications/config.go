package maintnotifications

import (
	"context"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/redis/go-redis/v9/internal"
	"github.com/redis/go-redis/v9/internal/maintnotifications/logs"
	"github.com/redis/go-redis/v9/internal/util"
)

// Mode represents the maintenance notifications mode
type Mode string

// Constants for maintenance push notifications modes
const (
	ModeDisabled Mode = "disabled" // Client doesn't send CLIENT MAINT_NOTIFICATIONS ON command
	ModeEnabled  Mode = "enabled"  // Client forcefully sends command, interrupts connection on error
	ModeAuto     Mode = "auto"     // Client tries to send command, disables feature on error
)

// IsValid returns true if the maintenance notifications mode is valid
func (m Mode) IsValid() bool {
	switch m {
	case ModeDisabled, ModeEnabled, ModeAuto:
		return true
	default:
		return false
	}
}

// String returns the string representation of the mode
func (m Mode) String() string {
	return string(m)
}

// EndpointType represents the type of endpoint to request in MOVING notifications
type EndpointType string

// Constants for endpoint types
const (
	EndpointTypeAuto         EndpointType = "auto"          // Auto-detect based on connection
	EndpointTypeInternalIP   EndpointType = "internal-ip"   // Internal IP address
	EndpointTypeInternalFQDN EndpointType = "internal-fqdn" // Internal FQDN
	EndpointTypeExternalIP   EndpointType = "external-ip"   // External IP address
	EndpointTypeExternalFQDN EndpointType = "external-fqdn" // External FQDN
	EndpointTypeNone         EndpointType = "none"          // No endpoint (reconnect with current config)
)

// IsValid returns true if the endpoint type is valid
func (e EndpointType) IsValid() bool {
	switch e {
	case EndpointTypeAuto, EndpointTypeInternalIP, EndpointTypeInternalFQDN,
		EndpointTypeExternalIP, EndpointTypeExternalFQDN, EndpointTypeNone:
		return true
	default:
		return false
	}
}

// String returns the string representation of the endpoint type
func (e EndpointType) String() string {
	return string(e)
}

// Config provides configuration options for maintenance notifications
type Config struct {
	// Mode controls how client maintenance notifications are handled.
	// Valid values: ModeDisabled, ModeEnabled, ModeAuto
	// Default: ModeAuto
	Mode Mode

	// EndpointType specifies the type of endpoint to request in MOVING notifications.
	// Valid values: EndpointTypeAuto, EndpointTypeInternalIP, EndpointTypeInternalFQDN,
	//               EndpointTypeExternalIP, EndpointTypeExternalFQDN, EndpointTypeNone
	// Default: EndpointTypeAuto
	EndpointType EndpointType

	// RelaxedTimeout is the concrete timeout value to use during
	// MIGRATING/FAILING_OVER states to accommodate increased latency.
	// This applies to both read and write timeouts.
	// Default: 10 seconds
	RelaxedTimeout time.Duration

	// HandoffTimeout is the maximum time to wait for connection handoff to complete.
	// If handoff takes longer than this, the old connection will be forcibly closed.
	// Default: 15 seconds (matches server-side eviction timeout)
	HandoffTimeout time.Duration

	// MaxWorkers is the maximum number of worker goroutines for processing handoff requests.
	// Workers are created on-demand and automatically cleaned up when idle.
	// If zero, defaults to min(10, PoolSize/2) to handle bursts effectively.
	// If explicitly set, enforces minimum of PoolSize/2
	//
	// Default: min(PoolSize/2, max(10, PoolSize/3)), Minimum when set: PoolSize/2
	MaxWorkers int

	// HandoffQueueSize is the size of the buffered channel used to queue handoff requests.
	// If the queue is full, new handoff requests will be rejected.
	// Scales with both worker count and pool size for better burst handling.
	//
	// Default: max(20×MaxWorkers, PoolSize), capped by MaxActiveConns+1 (if set) or 5×PoolSize
	// When set: minimum 200, capped by MaxActiveConns+1 (if set) or 5×PoolSize
	HandoffQueueSize int

	// PostHandoffRelaxedDuration is how long to keep relaxed timeouts on the new connection
	// after a handoff completes. This provides additional resilience during cluster transitions.
	// Default: 2 * RelaxedTimeout
	PostHandoffRelaxedDuration time.Duration

	// Circuit breaker configuration for endpoint failure handling
	// CircuitBreakerFailureThreshold is the number of failures before opening the circuit.
	// Default: 5
	CircuitBreakerFailureThreshold int

	// CircuitBreakerResetTimeout is how long to wait before testing if the endpoint recovered.
	// Default: 60 seconds
	CircuitBreakerResetTimeout time.Duration

	// CircuitBreakerMaxRequests is the maximum number of requests allowed in half-open state.
	// Default: 3
	CircuitBreakerMaxRequests int

	// MaxHandoffRetries is the maximum number of times to retry a failed handoff.
	// After this many retries, the connection will be removed from the pool.
	// Default: 3
	MaxHandoffRetries int
}

func (c *Config) IsEnabled() bool {
	return c != nil && c.Mode != ModeDisabled
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Mode:                       ModeAuto,         // Enable by default for Redis Cloud
		EndpointType:               EndpointTypeAuto, // Auto-detect based on connection
		RelaxedTimeout:             10 * time.Second,
		HandoffTimeout:             15 * time.Second,
		MaxWorkers:                 0, // Auto-calculated based on pool size
		HandoffQueueSize:           0, // Auto-calculated based on max workers
		PostHandoffRelaxedDuration: 0, // Auto-calculated based on relaxed timeout

		// Circuit breaker configuration
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerResetTimeout:     60 * time.Second,
		CircuitBreakerMaxRequests:      3,

		// Connection Handoff Configuration
		MaxHandoffRetries: 3,
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.RelaxedTimeout <= 0 {
		return ErrInvalidRelaxedTimeout
	}
	if c.HandoffTimeout <= 0 {
		return ErrInvalidHandoffTimeout
	}
	// Validate worker configuration
	// Allow 0 for auto-calculation, but negative values are invalid
	if c.MaxWorkers < 0 {
		return ErrInvalidHandoffWorkers
	}
	// HandoffQueueSize validation - allow 0 for auto-calculation
	if c.HandoffQueueSize < 0 {
		return ErrInvalidHandoffQueueSize
	}
	if c.PostHandoffRelaxedDuration < 0 {
		return ErrInvalidPostHandoffRelaxedDuration
	}

	// Circuit breaker validation
	if c.CircuitBreakerFailureThreshold < 1 {
		return ErrInvalidCircuitBreakerFailureThreshold
	}
	if c.CircuitBreakerResetTimeout < 0 {
		return ErrInvalidCircuitBreakerResetTimeout
	}
	if c.CircuitBreakerMaxRequests < 1 {
		return ErrInvalidCircuitBreakerMaxRequests
	}

	// Validate Mode (maintenance notifications mode)
	if !c.Mode.IsValid() {
		return ErrInvalidMaintNotifications
	}

	// Validate EndpointType
	if !c.EndpointType.IsValid() {
		return ErrInvalidEndpointType
	}

	// Validate configuration fields
	if c.MaxHandoffRetries < 1 || c.MaxHandoffRetries > 10 {
		return ErrInvalidHandoffRetries
	}

	return nil
}

// ApplyDefaults applies default values to any zero-value fields in the configuration.
// This ensures that partially configured structs get sensible defaults for missing fields.
func (c *Config) ApplyDefaults() *Config {
	return c.ApplyDefaultsWithPoolSize(0)
}

// ApplyDefaultsWithPoolSize applies default values to any zero-value fields in the configuration,
// using the provided pool size to calculate worker defaults.
// This ensures that partially configured structs get sensible defaults for missing fields.
func (c *Config) ApplyDefaultsWithPoolSize(poolSize int) *Config {
	return c.ApplyDefaultsWithPoolConfig(poolSize, 0)
}

// ApplyDefaultsWithPoolConfig applies default values to any zero-value fields in the configuration,
// using the provided pool size and max active connections to calculate worker and queue defaults.
// This ensures that partially configured structs get sensible defaults for missing fields.
func (c *Config) ApplyDefaultsWithPoolConfig(poolSize int, maxActiveConns int) *Config {
	if c == nil {
		return DefaultConfig().ApplyDefaultsWithPoolSize(poolSize)
	}

	defaults := DefaultConfig()
	result := &Config{}

	// Apply defaults for enum fields (empty/zero means not set)
	result.Mode = defaults.Mode
	if c.Mode != "" {
		result.Mode = c.Mode
	}

	result.EndpointType = defaults.EndpointType
	if c.EndpointType != "" {
		result.EndpointType = c.EndpointType
	}

	// Apply defaults for duration fields (zero means not set)
	result.RelaxedTimeout = defaults.RelaxedTimeout
	if c.RelaxedTimeout > 0 {
		result.RelaxedTimeout = c.RelaxedTimeout
	}

	result.HandoffTimeout = defaults.HandoffTimeout
	if c.HandoffTimeout > 0 {
		result.HandoffTimeout = c.HandoffTimeout
	}

	// Copy worker configuration
	result.MaxWorkers = c.MaxWorkers

	// Apply worker defaults based on pool size
	result.applyWorkerDefaults(poolSize)

	// Apply queue size defaults with new scaling approach
	// Default: max(20x workers, PoolSize), capped by maxActiveConns or 5x pool size
	workerBasedSize := result.MaxWorkers * 20
	poolBasedSize := poolSize
	result.HandoffQueueSize = util.Max(workerBasedSize, poolBasedSize)
	if c.HandoffQueueSize > 0 {
		// When explicitly set: enforce minimum of 200
		result.HandoffQueueSize = util.Max(200, c.HandoffQueueSize)
	}

	// Cap queue size: use maxActiveConns+1 if set, otherwise 5x pool size
	var queueCap int
	if maxActiveConns > 0 {
		queueCap = maxActiveConns + 1
		// Ensure queue cap is at least 2 for very small maxActiveConns
		if queueCap < 2 {
			queueCap = 2
		}
	} else {
		queueCap = poolSize * 5
	}
	result.HandoffQueueSize = util.Min(result.HandoffQueueSize, queueCap)

	// Ensure minimum queue size of 2 (fallback for very small pools)
	if result.HandoffQueueSize < 2 {
		result.HandoffQueueSize = 2
	}

	result.PostHandoffRelaxedDuration = result.RelaxedTimeout * 2
	if c.PostHandoffRelaxedDuration > 0 {
		result.PostHandoffRelaxedDuration = c.PostHandoffRelaxedDuration
	}

	// Apply defaults for configuration fields
	result.MaxHandoffRetries = defaults.MaxHandoffRetries
	if c.MaxHandoffRetries > 0 {
		result.MaxHandoffRetries = c.MaxHandoffRetries
	}

	// Circuit breaker configuration
	result.CircuitBreakerFailureThreshold = defaults.CircuitBreakerFailureThreshold
	if c.CircuitBreakerFailureThreshold > 0 {
		result.CircuitBreakerFailureThreshold = c.CircuitBreakerFailureThreshold
	}

	result.CircuitBreakerResetTimeout = defaults.CircuitBreakerResetTimeout
	if c.CircuitBreakerResetTimeout > 0 {
		result.CircuitBreakerResetTimeout = c.CircuitBreakerResetTimeout
	}

	result.CircuitBreakerMaxRequests = defaults.CircuitBreakerMaxRequests
	if c.CircuitBreakerMaxRequests > 0 {
		result.CircuitBreakerMaxRequests = c.CircuitBreakerMaxRequests
	}

	if internal.LogLevel.DebugOrAbove() {
		internal.Logger.Printf(context.Background(), logs.DebugLoggingEnabled())
		internal.Logger.Printf(context.Background(), logs.ConfigDebug(result))
	}
	return result
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	if c == nil {
		return DefaultConfig()
	}

	return &Config{
		Mode:                       c.Mode,
		EndpointType:               c.EndpointType,
		RelaxedTimeout:             c.RelaxedTimeout,
		HandoffTimeout:             c.HandoffTimeout,
		MaxWorkers:                 c.MaxWorkers,
		HandoffQueueSize:           c.HandoffQueueSize,
		PostHandoffRelaxedDuration: c.PostHandoffRelaxedDuration,

		// Circuit breaker configuration
		CircuitBreakerFailureThreshold: c.CircuitBreakerFailureThreshold,
		CircuitBreakerResetTimeout:     c.CircuitBreakerResetTimeout,
		CircuitBreakerMaxRequests:      c.CircuitBreakerMaxRequests,

		// Configuration fields
		MaxHandoffRetries: c.MaxHandoffRetries,
	}
}

// applyWorkerDefaults calculates and applies worker defaults based on pool size
func (c *Config) applyWorkerDefaults(poolSize int) {
	// Calculate defaults based on pool size
	if poolSize <= 0 {
		poolSize = 10 * runtime.GOMAXPROCS(0)
	}

	// When not set: min(poolSize/2, max(10, poolSize/3)) - balanced scaling approach
	originalMaxWorkers := c.MaxWorkers
	c.MaxWorkers = util.Min(poolSize/2, util.Max(10, poolSize/3))
	if originalMaxWorkers != 0 {
		// When explicitly set: max(poolSize/2, set_value) - ensure at least poolSize/2 workers
		c.MaxWorkers = util.Max(poolSize/2, originalMaxWorkers)
	}

	// Ensure minimum of 1 worker (fallback for very small pools)
	if c.MaxWorkers < 1 {
		c.MaxWorkers = 1
	}
}

// DetectEndpointType automatically detects the appropriate endpoint type
// based on the connection address and TLS configuration.
//
// For IP addresses:
//   - If TLS is enabled: requests FQDN for proper certificate validation
//   - If TLS is disabled: requests IP for better performance
//
// For hostnames:
//   - If TLS is enabled: always requests FQDN for proper certificate validation
//   - If TLS is disabled: requests IP for better performance
//
// Internal vs External detection:
//   - For IPs: uses private IP range detection
//   - For hostnames: uses heuristics based on common internal naming patterns
func DetectEndpointType(addr string, tlsEnabled bool) EndpointType {
	// Extract host from "host:port" format
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr // Assume no port
	}

	// Check if the host is an IP address or hostname
	ip := net.ParseIP(host)
	isIPAddress := ip != nil
	var endpointType EndpointType

	if isIPAddress {
		// Address is an IP - determine if it's private or public
		isPrivate := ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()

		if tlsEnabled {
			// TLS with IP addresses - still prefer FQDN for certificate validation
			if isPrivate {
				endpointType = EndpointTypeInternalFQDN
			} else {
				endpointType = EndpointTypeExternalFQDN
			}
		} else {
			// No TLS - can use IP addresses directly
			if isPrivate {
				endpointType = EndpointTypeInternalIP
			} else {
				endpointType = EndpointTypeExternalIP
			}
		}
	} else {
		// Address is a hostname
		isInternalHostname := isInternalHostname(host)
		if isInternalHostname {
			endpointType = EndpointTypeInternalFQDN
		} else {
			endpointType = EndpointTypeExternalFQDN
		}
	}

	return endpointType
}

// isInternalHostname determines if a hostname appears to be internal/private.
// This is a heuristic based on common naming patterns.
func isInternalHostname(hostname string) bool {
	// Convert to lowercase for comparison
	hostname = strings.ToLower(hostname)

	// Common internal hostname patterns
	internalPatterns := []string{
		"localhost",
		".local",
		".internal",
		".corp",
		".lan",
		".intranet",
		".private",
	}

	// Check for exact match or suffix match
	for _, pattern := range internalPatterns {
		if hostname == pattern || strings.HasSuffix(hostname, pattern) {
			return true
		}
	}

	// Check for RFC 1918 style hostnames (e.g., redis-1, db-server, etc.)
	// If hostname doesn't contain dots, it's likely internal
	if !strings.Contains(hostname, ".") {
		return true
	}

	// Default to external for fully qualified domain names
	return false
}
