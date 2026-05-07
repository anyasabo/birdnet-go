// client_test.go: Package mqtt provides an MQTT client implementation and associated tests.

package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/logger"
	"github.com/tphakala/birdnet-go/internal/observability"
)

const (
	// Test constants for goconst compliance
	testExampleBroker = "tcp://test.example.com:1883"
	testClientID      = "test-client"
)

// sanitizeClientID ensures the client ID is valid for MQTT brokers
func sanitizeClientID(id string) string {
	// Replace invalid characters with hyphen
	sanitized := strings.ReplaceAll(id, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")

	// Truncate to 23 characters if needed
	if len(sanitized) > 23 {
		sanitized = sanitized[:23]
	}

	return sanitized
}

// createTestClient is a helper function that creates and configures an MQTT client for testing purposes.
func createTestClient(t *testing.T, broker string) (Client, *observability.Metrics) {
	t.Helper()
	// Use test name as client ID to ensure uniqueness when running tests in parallel
	clientID := sanitizeClientID(t.Name())

	testSettings := &conf.Settings{
		Realtime: conf.RealtimeSettings{
			MQTT: conf.MQTTSettings{
				Broker:   broker,
				Username: "",
				Password: "",
			},
		},
	}
	testSettings.Main.Name = clientID
	metrics, err := observability.NewMetrics()
	require.NoError(t, err, "Failed to create metrics")

	client, err := NewClient(testSettings, metrics)
	require.NoError(t, err, "Failed to create MQTT client")

	return client, metrics
}

// TestIsIPAddress verifies the IP address detection function
func TestIsIPAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// IPv4 addresses
		{"Simple IPv4", "192.168.1.1", true},
		{"IPv4 with tcp protocol", "tcp://192.168.1.1:1883", true},
		{"IPv4 with mqtt protocol", "mqtt://10.0.0.1:1883", true},
		{"IPv4 localhost", "127.0.0.1", true},
		{"IPv4 with port", "127.0.0.1:1883", true},

		// IPv6 addresses
		{"Simple IPv6", "::1", true},
		{"IPv6 localhost with brackets", "[::1]", true},
		{"IPv6 with port", "[::1]:1883", true},
		{"IPv6 with tcp protocol", "tcp://[2001:db8::1]:1883", true},
		{"IPv6 with mqtt protocol", "mqtt://[2001:db8::1]:1883", true},
		{"IPv6 address only", "2001:db8::1", true},
		{"IPv6 with brackets", "[2001:db8::1]", true},

		// Hostnames (should return false)
		{"Simple hostname", "localhost", false},
		{"Hostname with protocol", "mqtt://localhost:1883", false},
		{"FQDN", "broker.hivemq.com", false},
		{"FQDN with port", "test.mosquitto.org:1883", false},
		{"Subdomain", "mqtt.example.com", false},

		// Invalid inputs (should return false)
		{"Empty string", "", false},
		{"Invalid hostname", "not-an-ip", false},
		{"Invalid IPv4", "256.256.256.256", false},
		{"Invalid IPv6", "2001:zz::1", false},
		{"Invalid protocol", "invalid://192.168.1.1", false},
		{"Malformed IPv6 brackets", "[2001:db8::1", false},
		{"IPv6 without closing bracket", "[2001:db8::1:1883", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIPAddress(tt.input)
			assert.Equal(t, tt.expected, result, "isIPAddress(%q) result mismatch", tt.input)
		})
	}
}

// TestCheckConnectionCooldown tests the connection cooldown validation
func TestCheckConnectionCooldown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		lastAttempt         time.Duration // how long ago was last attempt
		cooldownPeriod      time.Duration
		expectError         bool
		expectedErrorSubstr string
	}{
		{
			name:           "No previous attempt",
			lastAttempt:    24 * time.Hour, // Very long ago
			cooldownPeriod: 5 * time.Second,
			expectError:    false,
		},
		{
			name:                "Recent attempt within cooldown",
			lastAttempt:         1 * time.Second, // Recent
			cooldownPeriod:      5 * time.Second,
			expectError:         true,
			expectedErrorSubstr: "connection attempt too recent",
		},
		{
			name:           "Attempt just after cooldown period",
			lastAttempt:    6 * time.Second, // Just outside cooldown
			cooldownPeriod: 5 * time.Second,
			expectError:    false,
		},
		{
			name:           "Zero cooldown period",
			lastAttempt:    1 * time.Second,
			cooldownPeriod: 0,
			expectError:    false,
		},
		{
			name:           "Exactly at cooldown boundary",
			lastAttempt:    5 * time.Second,
			cooldownPeriod: 5 * time.Second,
			expectError:    false, // Should be allowed at boundary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test client
			config := DefaultConfig()
			config.Broker = testExampleBroker
			config.ReconnectCooldown = tt.cooldownPeriod
			metrics, _ := observability.NewMetrics()
			c := &client{
				config:          config,
				metrics:         metrics.MQTT,
				lastConnAttempt: time.Now().Add(-tt.lastAttempt),
				reconnectStop:   make(chan struct{}),
			}

			// Create logger for test
			testLog := GetLogger().With(
				logger.String("broker", config.Broker),
				logger.String("client_id", config.ClientID))

			// Test the method - acquire read lock as required by the method
			c.mu.RLock()
			err := c.checkConnectionCooldownLocked(testLog)
			c.mu.RUnlock()

			// Verify results
			if tt.expectError {
				require.Error(t, err, "Expected error but got nil")
				assert.Contains(t, err.Error(), tt.expectedErrorSubstr, "Error message mismatch")
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

// TestConfigureClientOptions tests the MQTT client options configuration
func TestConfigureClientOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupConfig func(*Config)
		expectError bool
		errorSubstr string
		verifyOpts  func(*testing.T, *paho.ClientOptions)
	}{
		{
			name: "Basic configuration",
			setupConfig: func(c *Config) {
				c.Broker = testExampleBroker
				c.ClientID = testClientID
				c.Username = "testuser"
				c.Password = "testpass"
				c.ConnectTimeout = 10 * time.Second
			},
			expectError: false,
			verifyOpts:  verifyClientNotConnected,
		},
		{
			name: "TLS configuration enabled but invalid cert",
			setupConfig: func(c *Config) {
				c.Broker = "ssl://test.example.com:8883"
				c.ClientID = testClientID
				c.TLS.Enabled = true
				c.TLS.CACert = "/nonexistent/ca.crt"
			},
			expectError: true,
			errorSubstr: "does not exist",
		},
		{
			name: "TLS configuration with InsecureSkipVerify",
			setupConfig: func(c *Config) {
				c.Broker = "ssl://test.example.com:8883"
				c.ClientID = testClientID
				c.TLS.Enabled = true
				c.TLS.InsecureSkipVerify = true
			},
			expectError: false,
			verifyOpts:  verifyClientNotConnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runConfigureClientOptionsTest(t, tt.setupConfig, tt.expectError, tt.errorSubstr, tt.verifyOpts)
		})
	}
}

// verifyClientNotConnected verifies that a client created with options is not initially connected
func verifyClientNotConnected(t *testing.T, opts *paho.ClientOptions) {
	t.Helper()
	client := paho.NewClient(opts)
	require.NotNil(t, client, "Expected client to be created successfully")
	assert.False(t, client.IsConnected(), "Expected client to not be connected initially (AutoReconnect should be disabled)")
	client.Disconnect(250)
}

// runConfigureClientOptionsTest executes a single test case for configureClientOptions
func runConfigureClientOptionsTest(t *testing.T, setupConfig func(*Config), expectError bool, errorSubstr string, verifyOpts func(*testing.T, *paho.ClientOptions)) {
	t.Helper()
	config := DefaultConfig()
	setupConfig(&config)
	metrics, _ := observability.NewMetrics()
	c := &client{
		config:        config,
		metrics:       metrics.MQTT,
		reconnectStop: make(chan struct{}),
	}

	testLog := GetLogger().With(
		logger.String("broker", config.Broker),
		logger.String("client_id", config.ClientID))
	opts, err := c.configureClientOptions(testLog)

	verifyConfigureClientOptionsResult(t, opts, err, expectError, errorSubstr, verifyOpts)
}

// verifyConfigureClientOptionsResult verifies the result of configureClientOptions
func verifyConfigureClientOptionsResult(t *testing.T, opts *paho.ClientOptions, err error, expectError bool, errorSubstr string, verifyOpts func(*testing.T, *paho.ClientOptions)) {
	t.Helper()
	if expectError {
		require.Error(t, err, "Expected error but got nil")
		assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(errorSubstr), "Error message mismatch")
		return
	}

	require.NoError(t, err, "Expected no error")
	require.NotNil(t, opts, "Expected non-nil options")
	if verifyOpts != nil {
		verifyOpts(t, opts)
	}
}

// TestPerformDNSResolution tests the DNS resolution functionality
func TestPerformDNSResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		broker      string
		expectError bool
		errorSubstr string
	}{
		{
			name:        "Valid hostname resolution",
			broker:      "tcp://example.com:1883",
			expectError: false,
		},
		{
			name:        "IP address (no DNS needed)",
			broker:      "tcp://8.8.8.8:1883",
			expectError: false,
		},
		{
			name:        "IPv6 address (no DNS needed)",
			broker:      "tcp://[::1]:1883",
			expectError: false,
		},
		{
			name:        "Invalid hostname",
			broker:      "tcp://this-hostname-does-not-exist.invalid:1883",
			expectError: true,
			errorSubstr: "no such host",
		},
		{
			name:        "Invalid broker URL format",
			broker:      "invalid://[malformed",
			expectError: true,
			errorSubstr: "parse",
		},
		{
			name:        "Empty broker URL",
			broker:      "",
			expectError: true,
			errorSubstr: "lookup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test client with config
			config := DefaultConfig()
			config.Broker = tt.broker
			metrics, _ := observability.NewMetrics()
			c := &client{
				config:        config,
				metrics:       metrics.MQTT,
				reconnectStop: make(chan struct{}),
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			// Create logger for test
			testLog := GetLogger().With(
				logger.String("broker", config.Broker),
				logger.String("client_id", config.ClientID))

			// Test the method
			err := c.performDNSResolution(ctx, testLog)

			// Verify results
			if tt.expectError {
				require.Error(t, err, "Expected error but got nil")
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorSubstr), "Error message mismatch")
			} else {
				assert.NoError(t, err, "Expected no error")
			}
		})
	}
}

// TestCalculateCancelTimeout tests the timeout calculation logic
func TestCalculateCancelTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		disconnectTimeout time.Duration
		expectedTimeout   uint
		description       string
	}{
		{
			name:              "Normal timeout calculation",
			disconnectTimeout: 5 * time.Second,
			expectedTimeout:   durationToMillisUint(CancelDisconnectTimeout), // min(1000, 5000/5) = min(1000, 1000) = 1000
			description:       "Standard case with reasonable timeout",
		},
		{
			name:              "Very short timeout",
			disconnectTimeout: 500 * time.Millisecond,
			expectedTimeout:   100, // min(1000, 500/5) = min(1000, 100) = 100
			description:       "Short timeout calculation: ms/5",
		},
		{
			name:              "Very large timeout",
			disconnectTimeout: 10 * time.Second,
			expectedTimeout:   durationToMillisUint(CancelDisconnectTimeout), // min(1000, 10000/5) = min(1000, 2000) = 1000
			description:       "Large timeout should be capped at minimum",
		},
		{
			name:              "Zero timeout",
			disconnectTimeout: 0,
			expectedTimeout:   durationToMillisUint(CancelDisconnectTimeout), // Should use minimum
			description:       "Zero timeout should use minimum safe value",
		},
		{
			name:              "Negative timeout",
			disconnectTimeout: -5 * time.Second,
			expectedTimeout:   durationToMillisUint(CancelDisconnectTimeout), // Should use minimum
			description:       "Negative timeout should use minimum safe value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test client with the specified disconnect timeout
			config := DefaultConfig()
			config.DisconnectTimeout = tt.disconnectTimeout
			metrics, _ := observability.NewMetrics()
			c := &client{
				config:        config,
				metrics:       metrics.MQTT,
				reconnectStop: make(chan struct{}),
			}

			// Test the method
			result := c.calculateCancelTimeout()

			// Verify result
			assert.Equal(t, tt.expectedTimeout, result, tt.description)

			// Verify the result is never zero
			assert.NotZero(t, result, "Calculated timeout should never be zero")

			// Verify the result is reasonable (not more than minimum timeout)
			maxTimeout := durationToMillisUint(CancelDisconnectTimeout)
			assert.LessOrEqual(t, result, maxTimeout, "Calculated timeout should be at most minimum timeout")
		})
	}
}

// TestPerformConnectionAttempt tests the connection attempt functionality
func TestPerformConnectionAttempt(t *testing.T) {
	// Network-heavy; avoid parallelism to reduce flakes
	tests := []struct {
		name         string
		setupConfig  func(*Config)
		expectError  bool
		errorSubstr  string
		shortContext bool // Use short context timeout for cancellation test
	}{
		{
			name:        "Invalid broker URL",
			setupConfig: func(config *Config) { config.Broker = "invalid://malformed-url" },
			expectError: true, errorSubstr: "network Error",
		},
		{
			name: "Connection timeout",
			setupConfig: func(config *Config) {
				config.Broker = "tcp://192.0.2.1:1883"
				config.ConnectTimeout = 100 * time.Millisecond
			},
			expectError: true, errorSubstr: "timeout",
		},
		{
			name: "Context cancelled",
			setupConfig: func(config *Config) {
				config.Broker = "tcp://192.0.2.1:1883"
				config.ConnectTimeout = 5 * time.Second
			},
			expectError: true, errorSubstr: "context", shortContext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPerformConnectionAttemptTest(t, tt.setupConfig, tt.expectError, tt.errorSubstr, tt.shortContext)
		})
	}
}

// runPerformConnectionAttemptTest executes a single test case for performConnectionAttempt
func runPerformConnectionAttemptTest(t *testing.T, setupConfig func(*Config), expectError bool, errorSubstr string, shortContext bool) {
	t.Helper()
	config := DefaultConfig()
	setupConfig(&config)
	metrics, _ := observability.NewMetrics()
	c := &client{config: config, metrics: metrics.MQTT, reconnectStop: make(chan struct{})}
	defer c.Disconnect()

	testLog := GetLogger().With(
		logger.String("broker", config.Broker),
		logger.String("client_id", config.ClientID))

	timeout := 2 * time.Second
	if shortContext {
		timeout = 50 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	opts, optsErr := c.configureClientOptions(testLog)
	if optsErr != nil {
		verifyConnectionAttemptError(t, optsErr, expectError, errorSubstr)
		return
	}

	clientToConnect := paho.NewClient(opts)
	err := c.performConnectionAttempt(ctx, clientToConnect, testLog)
	verifyConnectionAttemptError(t, err, expectError, errorSubstr)
}

// verifyConnectionAttemptError verifies the error result of a connection attempt
func verifyConnectionAttemptError(t *testing.T, err error, expectError bool, errorSubstr string) {
	t.Helper()
	if expectError {
		require.Error(t, err, "Expected error but got nil")
		assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(errorSubstr), "Error message mismatch")
	} else {
		assert.NoError(t, err, "Expected no error")
	}
}

// TestTimeRoundingEdgeCase verifies that sub-second durations are handled correctly
// when rounding time for display in error messages.
func TestTimeRoundingEdgeCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		lastAttempt         time.Duration // how long ago was last attempt
		expectedDisplayTime string        // expected time shown in error
	}{
		{
			name:                "Sub-second rounds to 1s",
			lastAttempt:         500 * time.Millisecond,
			expectedDisplayTime: "1s",
		},
		{
			name:                "Exactly 1 second",
			lastAttempt:         1 * time.Second,
			expectedDisplayTime: "1s",
		},
		{
			name:                "1.4 seconds rounds to 1s",
			lastAttempt:         1400 * time.Millisecond,
			expectedDisplayTime: "1s",
		},
		{
			name:                "1.5 seconds rounds to 2s",
			lastAttempt:         1500 * time.Millisecond,
			expectedDisplayTime: "2s",
		},
		{
			name:                "2.1 seconds rounds to 2s",
			lastAttempt:         2100 * time.Millisecond,
			expectedDisplayTime: "2s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test client with cooldown that will trigger
			config := DefaultConfig()
			config.Broker = testExampleBroker
			config.ReconnectCooldown = 5 * time.Second
			metrics, _ := observability.NewMetrics()
			c := &client{
				config:          config,
				metrics:         metrics.MQTT,
				lastConnAttempt: time.Now().Add(-tt.lastAttempt),
				reconnectStop:   make(chan struct{}),
			}

			testLog := GetLogger().With(
				logger.String("broker", config.Broker),
				logger.String("client_id", config.ClientID))

			// Test the checkConnectionCooldownLocked method
			c.mu.RLock()
			err := c.checkConnectionCooldownLocked(testLog)
			c.mu.RUnlock()

			// Should always error since we're within cooldown
			require.Error(t, err, "Expected error but got nil")

			// Check that the error message contains the expected rounded time
			expectedMsg := fmt.Sprintf("last attempt was %s ago", tt.expectedDisplayTime)
			assert.Contains(t, err.Error(), expectedMsg, "Error message should contain expected time")
		})
	}
}

// TestCalculateBackoffDelay verifies exponential backoff delay calculation
func TestCalculateBackoffDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseDelay time.Duration
		attempts  int
		expected  time.Duration
	}{
		{
			name:      "Zero attempts returns base delay",
			baseDelay: 1 * time.Second,
			attempts:  0,
			expected:  1 * time.Second,
		},
		{
			name:      "One attempt doubles delay",
			baseDelay: 1 * time.Second,
			attempts:  1,
			expected:  2 * time.Second,
		},
		{
			name:      "Two attempts quadruples delay",
			baseDelay: 1 * time.Second,
			attempts:  2,
			expected:  4 * time.Second,
		},
		{
			name:      "Three attempts gives 8x delay",
			baseDelay: 1 * time.Second,
			attempts:  3,
			expected:  8 * time.Second,
		},
		{
			name:      "Large attempts cap at MaxReconnectDelay",
			baseDelay: 1 * time.Second,
			attempts:  100,
			expected:  MaxReconnectDelay,
		},
		{
			name:      "Caps at boundary",
			baseDelay: 1 * time.Second,
			attempts:  20, // 2^20 seconds = ~1048576s, way past 5 min
			expected:  MaxReconnectDelay,
		},
		{
			name:      "Custom base delay",
			baseDelay: 500 * time.Millisecond,
			attempts:  1,
			expected:  1 * time.Second,
		},
		{
			name:      "Custom base delay caps correctly",
			baseDelay: 2 * time.Minute,
			attempts:  2, // 2min * 4 = 8min > MaxReconnectDelay (5min)
			expected:  MaxReconnectDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := calculateBackoffDelay(tt.baseDelay, tt.attempts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHandleReconnectFailureErrorSuppression verifies that repeated identical
// connection errors are suppressed after the first occurrence.
func TestHandleReconnectFailureErrorSuppression(t *testing.T) {
	t.Parallel()

	metrics, err := observability.NewMetrics()
	require.NoError(t, err)

	config := DefaultConfig()
	config.Broker = testExampleBroker
	config.ClientID = testClientID

	c := &client{
		config:        config,
		metrics:       metrics.MQTT,
		reconnectStop: make(chan struct{}),
	}

	testLog := GetLogger().With(
		logger.String("broker", config.Broker),
		logger.String("client_id", config.ClientID))

	testErr := errors.New("connection refused")

	// First failure should set state
	c.handleReconnectFailure(testLog, testErr)

	c.mu.RLock()
	assert.Equal(t, 1, c.reconnectAttempts, "First failure should increment attempts")
	assert.Equal(t, testErr.Error(), c.lastConnErrMsg, "Should store error message")
	assert.Equal(t, 1, c.connErrCount, "First occurrence count should be 1")
	assert.False(t, c.lastConnErrLogTime.IsZero(), "Should record log time")
	c.mu.RUnlock()

	// Second failure with same error should increment count
	c.handleReconnectFailure(testLog, testErr)

	c.mu.RLock()
	assert.Equal(t, 2, c.reconnectAttempts, "Second failure should increment attempts")
	assert.Equal(t, 2, c.connErrCount, "Same error should increment count")
	c.mu.RUnlock()

	// Failure with different error should reset suppression state
	differentErr := errors.New("no route to host")
	c.handleReconnectFailure(testLog, differentErr)

	c.mu.RLock()
	assert.Equal(t, 3, c.reconnectAttempts, "Third failure should increment attempts")
	assert.Equal(t, differentErr.Error(), c.lastConnErrMsg, "Should update to new error message")
	assert.Equal(t, 1, c.connErrCount, "Different error should reset count to 1")
	c.mu.RUnlock()
}

// TestOnConnectResetsBackoffState verifies that successful connection resets
// all reconnection backoff state.
func TestOnConnectResetsBackoffState(t *testing.T) {
	t.Parallel()

	metrics, err := observability.NewMetrics()
	require.NoError(t, err)

	config := DefaultConfig()
	config.Broker = testExampleBroker
	config.ClientID = testClientID

	c := &client{
		config:             config,
		metrics:            metrics.MQTT,
		reconnectStop:      make(chan struct{}),
		reconnectAttempts:  5,
		lastConnErrMsg:     "some error",
		connErrCount:       3,
		lastConnErrLogTime: time.Now(),
	}

	// Create a mock paho client that reports as connected
	opts := paho.NewClientOptions()
	opts.AddBroker(testExampleBroker)
	opts.SetAutoReconnect(false)
	mockClient := paho.NewClient(opts)

	// Call onConnect (will fail to publish LWT since we're not really connected,
	// but it should still reset backoff state)
	c.onConnect(mockClient)

	c.mu.RLock()
	assert.Equal(t, 0, c.reconnectAttempts, "Should reset reconnect attempts")
	assert.Equal(t, 0, c.connErrCount, "Should reset error count")
	assert.Empty(t, c.lastConnErrMsg, "Should clear last error message")
	c.mu.RUnlock()
}

// TestPublishSuppressionWhileDisconnected verifies that publish attempts are
// suppressed while the client is in a known disconnected state, preventing
// Sentry event floods when the broker is unreachable.
func TestPublishSuppressionWhileDisconnected(t *testing.T) {
	t.Parallel()

	metrics, err := observability.NewMetrics()
	require.NoError(t, err)

	config := DefaultConfig()
	config.Broker = testExampleBroker
	config.ClientID = testClientID

	c := &client{
		config:        config,
		metrics:       metrics.MQTT,
		reconnectStop: make(chan struct{}),
	}

	t.Run("suppresses publishes after onConnectionLost", func(t *testing.T) {
		t.Parallel()

		localMetrics, localErr := observability.NewMetrics()
		require.NoError(t, localErr)

		localConfig := DefaultConfig()
		localConfig.Broker = testExampleBroker
		localConfig.ClientID = testClientID

		tc := &client{
			config:        localConfig,
			metrics:       localMetrics.MQTT,
			reconnectStop: make(chan struct{}),
		}

		// Simulate connection loss
		tc.mu.Lock()
		tc.disconnected = true
		tc.disconnectedSince = time.Now()
		tc.mu.Unlock()

		ctx := t.Context()

		// First publish should be suppressed (with warning logged)
		err := tc.publishInternal(ctx, "test/topic", "payload1", false)
		require.NoError(t, err, "Suppressed publish should return nil, not an error")

		tc.mu.RLock()
		assert.True(t, tc.publishSuppressed, "Should be marked as suppressed after first publish")
		assert.Equal(t, int64(1), tc.suppressedPublishCount, "Should count first suppressed publish")
		tc.mu.RUnlock()

		// Subsequent publishes should also be suppressed silently
		for range 10 {
			err = tc.publishInternal(ctx, "test/topic", "payload", false)
			require.NoError(t, err, "Suppressed publish should return nil")
		}

		tc.mu.RLock()
		assert.Equal(t, int64(11), tc.suppressedPublishCount, "Should count all suppressed publishes")
		tc.mu.RUnlock()
	})

	t.Run("onConnect resets suppression state", func(t *testing.T) {
		t.Parallel()

		localMetrics, localErr := observability.NewMetrics()
		require.NoError(t, localErr)

		localConfig := DefaultConfig()
		localConfig.Broker = testExampleBroker
		localConfig.ClientID = testClientID

		tc := &client{
			config:                 localConfig,
			metrics:                localMetrics.MQTT,
			reconnectStop:          make(chan struct{}),
			disconnected:           true,
			publishSuppressed:      true,
			suppressedPublishCount: 42,
			disconnectedSince:      time.Now().Add(-5 * time.Minute),
		}

		// Create a mock paho client
		opts := paho.NewClientOptions()
		opts.AddBroker(testExampleBroker)
		opts.SetAutoReconnect(false)
		mockClient := paho.NewClient(opts)

		// Simulate reconnection
		tc.onConnect(mockClient)

		tc.mu.RLock()
		assert.False(t, tc.disconnected, "Should clear disconnected flag")
		assert.False(t, tc.publishSuppressed, "Should clear publishSuppressed flag")
		assert.Equal(t, int64(0), tc.suppressedPublishCount, "Should reset suppressed count")
		tc.mu.RUnlock()
	})

	t.Run("not suppressed when connected", func(t *testing.T) {
		// When not disconnected, suppressPublishWhileDisconnected should return false
		c.mu.Lock()
		c.disconnected = false
		c.mu.Unlock()

		suppressed := c.suppressPublishWhileDisconnected("test/topic")
		assert.False(t, suppressed, "Should not suppress when connected")
	})

	t.Run("onConnectionLost sets disconnected state", func(t *testing.T) {
		t.Parallel()

		localMetrics, localErr := observability.NewMetrics()
		require.NoError(t, localErr)

		localConfig := DefaultConfig()
		localConfig.Broker = testExampleBroker
		localConfig.ClientID = testClientID

		tc := &client{
			config:        localConfig,
			metrics:       localMetrics.MQTT,
			reconnectStop: make(chan struct{}),
		}

		// Simulate connection loss
		tc.onConnectionLost(nil, errors.New("connection refused"))

		tc.mu.RLock()
		assert.True(t, tc.disconnected, "Should set disconnected flag")
		assert.False(t, tc.publishSuppressed, "Should not be suppressed yet (no publish attempted)")
		assert.Equal(t, int64(0), tc.suppressedPublishCount, "Should start with zero suppressed count")
		assert.False(t, tc.disconnectedSince.IsZero(), "Should record disconnect time")
		tc.mu.RUnlock()
	})
}
