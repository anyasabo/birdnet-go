# BirdNET-Go Testing Guidelines

This document describes how to write consistent, maintainable tests for BirdNET-Go. It serves both human developers and LLM assistants working on the codebase.

## Critical Rules

### Absolute Requirements

1. **All tests MUST use testify** - Use `assert` and `require` from `github.com/stretchr/testify`
2. **No artificial passing** - Tests must NEVER be written to pass artificially. No shortcuts. Ever.
3. **No manual assertions** - Never use `if err != nil { t.Fatal(err) }` patterns

### The Testify Rule

```go
// WRONG - Never do this
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}
if got != expected {
    t.Errorf("got %v, want %v", got, expected)
}

// CORRECT - Always use testify
require.NoError(t, err)
assert.Equal(t, expected, got)
```

## Running Tests

### Quick Reference

| Tier | Command | Preconditions | Expected Time |
|------|---------|---------------|---------------|
| Unit | `task test` | CGO enabled, TFLite headers | < 60s |
| Integration | `go test -tags=integration -race -timeout 300s ./internal/datastore/v2/... ./internal/mqtt/... ./internal/spectrogram/... ./internal/testutil/containers/...` | Docker (testcontainers), SoX | 1-5 min |
| Stress/perf | `task test` (runs by default) | Same as unit | Varies |

### Zero-Skip Rule

`task test` must produce **zero skipped tests**. If `go test` output shows skipped tests at the unit tier, that is a bug. Tests that need external infrastructure belong behind `//go:build integration`, not a runtime `t.Skip()`.

### Tier 1: Unit Tests

Unit tests run on every push and PR via `task test`.

- **Build tags:** `noembed,skipfrontend` (avoids embedding 72MB of TFLite models and the Svelte frontend into test binaries; see "Build Tags" below)
- **Rule:** Must pass with zero skips in a standard dev environment -- no Docker, no MQTT broker, no external services
- **CI:** `golangci-test.yml` runs these with `-short` to skip tier 3 stress tests. Locally, `task test` runs all tiers.

### Tier 2: Integration Tests

Integration tests require Docker and are invisible to `task test`. Opt in via `//go:build integration`.

- **What runs:** Testcontainer-backed MySQL, Mosquitto, and MediaMTX; full migration pipelines; real SoX execution for spectrogram tests
- **When to use this tag:** Any test that needs a real external service (database, message broker, media server) or a binary that may not be installed (`sox`)

### Tier 3: Stress/Performance Tests

Stress tests run by default with `task test` but can be skipped with `task test -- -short` during rapid iteration.

- **Rule:** Only for tests taking >5 seconds due to deliberate heavy workloads (thousands of rows, hundreds of goroutines, memory pressure). Never for sleeps or missing infrastructure.
- **CI:** `golangci-test.yml` runs with `-short` so stress tests are skipped in PR CI. Release and nightly workflows should run without `-short`.

### When to Use `testing.Short()`

Use `testing.Short()` **only** for deliberate stress/performance tests that take >5 seconds due to intentional heavy computation. Each guard must have a one-line comment explaining the workload:

```go
// Stress test: 10,000 iterations measuring MemStats for leak detection (~5s).
if testing.Short() {
    t.Skip("Skipping memory leak test in short mode")
}
```

Never use `testing.Short()` for:
- Tests that are slow because of `time.Sleep` (fix the sleep instead)
- Tests that need external infrastructure (use `//go:build integration`)
- Tests that "might be flaky" (fix the flakiness)

### Build Tags

**`noembed`** -- Replaces the 72MB of `//go:embed` TFLite model data with nil byte slices. Without this tag, every test binary embeds the full model files, ballooning compile time and memory. The runtime code paths are identical (`if modelData != nil { return modelData }` vs loading from disk).

**`skipfrontend`** -- Replaces the `//go:embed all:dist` Svelte build output with a stub `fstest.MapFS`. Without this tag, `go test` requires `frontend/dist/` to exist (i.e., a full frontend build).

**`integration`** -- Gates tests that need Docker (testcontainers) or external binaries. These tests are invisible to `task test` and `go test ./...` by default.

### Production Compile Check

Tests use `noembed,skipfrontend` but production binaries do not. To ensure the production code paths compile correctly, CI runs `go build ./...` without these tags before the test step. This uses a dummy `frontend/dist/index.html` stub (no frontend build needed) and the model files already checked into the repo. The check catches broken `//go:embed` directives, type mismatches, and missing symbols in about 3 seconds.

Running `go test` without these tags is not worth the cost: it embeds 72MB of model data into every package's test binary, and the runtime code paths are trivial.

### Environment Variables

| Variable | Tier | Description |
|----------|------|-------------|
| `CGO_ENABLED=1` | Unit, Integration | Required for TFLite C bindings |
| `CGO_CFLAGS` | Unit, Integration | Path to TFLite C API headers |
| `MQTT_TEST_BROKER` | Integration (CI only) | Broker address for MQTT integration tests (e.g., `tcp://localhost:1883`) |

## Assert vs Require

Understanding when to use `assert` vs `require` is critical:

| Function | Behavior | Use Case |
|----------|----------|----------|
| `require.*` | Stops test immediately on failure | Setup, prerequisites, conditions that must succeed |
| `assert.*` | Continues test on failure | Validations, checking multiple conditions |

### Examples

```go
func TestExample(t *testing.T) {
    // Use require for setup - if this fails, nothing else matters
    cfg, err := loadConfig()
    require.NoError(t, err, "config must load")
    require.NotNil(t, cfg, "config must not be nil")

    // Use assert for validations - check all conditions
    assert.Equal(t, "expected", cfg.Name)
    assert.True(t, cfg.Enabled)
    assert.Len(t, cfg.Items, 3)
}
```

## Shared Test Helpers

### Why Shared Helpers Matter

Shared helpers reduce maintenance burden and ensure consistency. When you see a pattern repeated across tests, extract it to a helper.

### File Naming Convention

Place shared helpers in `*_test_helpers_test.go` files within the same package:

```
internal/mypackage/
├── mypackage.go
├── mypackage_test.go
└── mypackage_test_helpers_test.go  # Shared helpers
```

### Helper Patterns

#### Factory Functions

Create test objects consistently:

```go
// In *_test_helpers_test.go
func createTestConfig(t *testing.T, opts ...func(*Config)) *Config {
    t.Helper()
    cfg := &Config{
        Name:    "test-config",
        Timeout: 5 * time.Second,
        Enabled: true,
    }
    for _, opt := range opts {
        opt(cfg)
    }
    return cfg
}

// Usage
func TestSomething(t *testing.T) {
    cfg := createTestConfig(t, func(c *Config) {
        c.Timeout = 10 * time.Second
    })
}
```

#### Setup Helpers

Encapsulate complex setup:

```go
func setupTestServer(t *testing.T) (*Server, func()) {
    t.Helper()

    srv := NewServer(testConfig())
    require.NoError(t, srv.Start())

    cleanup := func() {
        srv.Stop()
    }

    return srv, cleanup
}

// Usage
func TestServer(t *testing.T) {
    srv, cleanup := setupTestServer(t)
    defer cleanup()
    // ... test code
}
```

#### Assertion Helpers

For domain-specific assertions:

```go
func assertDetectionValid(t *testing.T, d *Detection) {
    t.Helper()
    assert.NotEmpty(t, d.Species)
    assert.Greater(t, d.Confidence, 0.0)
    assert.LessOrEqual(t, d.Confidence, 1.0)
    assert.False(t, d.Timestamp.IsZero())
}
```

### The t.Helper() Rule

**Always call `t.Helper()` at the start of helper functions.** This ensures error messages point to the calling test, not the helper.

```go
func requireFileExists(t *testing.T, path string) {
    t.Helper()  // REQUIRED - makes errors point to caller
    _, err := os.Stat(path)
    require.NoError(t, err, "file should exist: %s", path)
}
```

## Table-Driven Tests

Use table-driven tests with subtests for comprehensive coverage:

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {
            name:  "valid config",
            input: `{"name": "test"}`,
            want:  &Config{Name: "test"},
        },
        {
            name:    "invalid json",
            input:   `{invalid}`,
            wantErr: true,
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := ParseConfig(tt.input)
            got, err := ParseConfig(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## Goroutine Safety

### Critical Rule

**Never call `t.Fatal`, `t.Error`, or testify assertions from goroutines.** These methods are not thread-safe and can cause panics or missed failures.

```go
// WRONG - Will panic or miss failures
func TestConcurrent(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            result := process(n)
            assert.NotNil(t, result)  // UNSAFE!
        }(i)
    }
    wg.Wait()
}

// CORRECT - Collect results, assert in main goroutine
func TestConcurrent(t *testing.T) {
    results := make(chan *Result, 10)
    var wg sync.WaitGroup

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            results <- process(n)
        }(i)
    }

    wg.Wait()
    close(results)

    for result := range results {
        assert.NotNil(t, result)  // Safe - main goroutine
    }
}
```

## Async Testing Patterns

### Timeout Durations for CI

GitHub Actions can be slower than local machines. Use appropriate timeouts:

| Operation Type | Minimum Timeout |
|----------------|-----------------|
| Channel operations | 500ms |
| HTTP requests | 1s |
| Database operations | 2s |
| Complex async flows | 5s |

### Eventually Pattern

For async operations, use polling with timeout:

```go
func TestAsyncOperation(t *testing.T) {
    svc := NewService()
    svc.StartAsync()

    // Wait for condition with timeout
    require.Eventually(t, func() bool {
        return svc.IsReady()
    }, 5*time.Second, 100*time.Millisecond, "service should become ready")
}
```

### Channel Testing

```go
func TestChannelReceive(t *testing.T) {
    ch := make(chan Event, 1)
    go produceEvent(ch)

    select {
    case event := <-ch:
        assert.Equal(t, "expected", event.Type)
    case <-time.After(500 * time.Millisecond):
        t.Fatal("timeout waiting for event")
    }
}
```

## Cleanup and Resource Management

### Proper Cleanup Order

Use `t.Cleanup()` for automatic cleanup in reverse order:

```go
func TestWithResources(t *testing.T) {
    // Resources are cleaned up in reverse order
    db := setupDatabase(t)
    t.Cleanup(func() { db.Close() })  // Cleaned up last

    cache := setupCache(t)
    t.Cleanup(func() { cache.Clear() })  // Cleaned up second

    srv := setupServer(t, db, cache)
    t.Cleanup(func() { srv.Stop() })  // Cleaned up first
}
```

### Goroutine Leak Detection

Use `go.uber.org/goleak` to detect goroutine leaks:

```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

// Or per-test
func TestNoLeaks(t *testing.T) {
    defer goleak.VerifyNone(t)
    // ... test code
}
```

## Mocking with testify/mock

### When to Use Mocks

- External services (APIs, databases)
- Time-dependent operations
- Non-deterministic behavior
- Expensive operations

### Mock Generation with mockery

Generate mocks automatically:

```bash
# Install mockery
go install github.com/vektra/mockery/v2@latest

# Generate mock for interface
mockery --name=MyInterface --dir=./internal/mypackage --output=./internal/mypackage/mocks
```

### Mock Usage

```go
func TestWithMock(t *testing.T) {
    mockRepo := mocks.NewMockRepository(t)

    // Setup expectation
    mockRepo.On("GetByID", "123").Return(&Entity{ID: "123"}, nil)

    svc := NewService(mockRepo)
    result, err := svc.Process("123")

    require.NoError(t, err)
    assert.Equal(t, "123", result.ID)

    // Verify expectations
    mockRepo.AssertExpectations(t)
}
```

### Async Mock Expectations

Use `.Maybe()` for expectations that may not be called (race conditions):

```go
mockRepo.On("Save", mock.Anything).Return(nil).Maybe()
```

## Modern Go Features (1.22+)

### Go 1.24: t.Context()

Get a context that's canceled when the test ends:

```go
func TestWithContext(t *testing.T) {
    ctx := t.Context()  // Automatically canceled on test end

    result, err := fetchWithContext(ctx, "http://example.com")
    require.NoError(t, err)
    assert.NotEmpty(t, result)
}
```

### Go 1.24: t.Chdir()

Temporarily change directory for a test:

```go
func TestInTempDir(t *testing.T) {
    tmpDir := t.TempDir()
    t.Chdir(tmpDir)  // Automatically restored after test

    // Working directory is now tmpDir
    err := createConfigFile("config.yaml")
    require.NoError(t, err)
}
```

### Go 1.24: b.Loop() for Benchmarks

More accurate benchmarks:

```go
func BenchmarkProcess(b *testing.B) {
    data := setupBenchmarkData()

    b.ResetTimer()
    for b.Loop() {  // More accurate than range b.N
        process(data)
    }
}
```

### Go 1.25: testing/synctest

Deterministic testing of concurrent code (experimental):

```go
import "testing/synctest"

func TestConcurrent(t *testing.T) {
    synctest.Run(func() {
        var ready atomic.Bool
        go func() {
            time.Sleep(time.Second)
            ready.Store(true)
        }()

        synctest.Wait()  // Waits for goroutine to block on sleep
        assert.False(t, ready.Load())

        time.Sleep(time.Second)
        synctest.Wait()  // Time advances, goroutine completes
        assert.True(t, ready.Load())
    })
}
```

## LLM Guidelines

### When Writing Tests

1. **Think ahead** - Identify patterns likely to repeat and create shared helpers proactively
2. **Check for existing helpers** - Look in `*_test_helpers_test.go` files before creating new ones
3. **Refactor when you see patterns** - If you encounter repeated test code, extract to helpers, and always include `t.Helper()`
4. **Use testify exclusively** - Never fall back to manual if/error patterns
5. **Consider CI environment** - Use generous timeouts (500ms minimum for async operations)

### When Reviewing Tests

1. Verify all assertions use testify
2. Check that `require` is used for setup, `assert` for validations
3. Ensure goroutine safety - no assertions in goroutines
4. Look for opportunities to extract shared helpers
5. Verify cleanup is properly handled

### Anti-Patterns to Avoid

```go
// Anti-pattern 1: Manual assertions
if err != nil {
    t.Fatal(err)
}

// Anti-pattern 2: Assertions in goroutines
go func() {
    assert.NoError(t, err)  // UNSAFE
}()

// Anti-pattern 3: Magic sleep instead of proper sync
time.Sleep(100 * time.Millisecond)
// hope the async operation completed...

// Anti-pattern 4: Ignoring errors
result, _ := operation()  // Lost error information

// Anti-pattern 5: Tests that always pass
func TestSomething(t *testing.T) {
    // TODO: implement
}
```

## Quick Reference

### Common Assertions

```go
// Equality
assert.Equal(t, expected, actual)
assert.NotEqual(t, unexpected, actual)

// Nil checks
assert.Nil(t, value)
assert.NotNil(t, value)

// Boolean
assert.True(t, condition)
assert.False(t, condition)

// Errors
assert.NoError(t, err)
assert.Error(t, err)
assert.ErrorIs(t, err, expectedErr)
assert.ErrorContains(t, err, "substring")

// Collections
assert.Len(t, slice, expectedLen)
assert.Empty(t, slice)
assert.NotEmpty(t, slice)
assert.Contains(t, slice, element)

// Numeric
assert.Greater(t, a, b)
assert.Less(t, a, b)
assert.InDelta(t, expected, actual, delta)

// Strings
assert.Contains(t, str, substring)
assert.Regexp(t, pattern, str)
```

### Test Structure Template

```go
func TestFeature(t *testing.T) {
    // Arrange
    cfg := createTestConfig(t)
    svc := NewService(cfg)
    t.Cleanup(func() { svc.Close() })

    // Act
    result, err := svc.DoSomething()

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "expected", result.Value)
}
```

## Further Reading

- [testify documentation](https://github.com/stretchr/testify)
- [Go testing package](https://pkg.go.dev/testing)
- [Go 1.24 release notes](https://go.dev/doc/go1.24)
- [Go 1.25 release notes](https://go.dev/doc/go1.25)
- [mockery documentation](https://vektra.github.io/mockery/)
