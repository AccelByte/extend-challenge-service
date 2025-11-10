# Integration Tests

This directory contains integration tests for the Challenge Service that test against a real PostgreSQL database.

## Quick Start

### One-Time Database Setup

**Option 1: Use Main Postgres Container (Recommended)**

Run this one-liner to set up the test database in the existing postgres container:

```bash
docker-compose up -d postgres && sleep 3 && docker exec challenge-postgres psql -U postgres -c "CREATE DATABASE testdb;" 2>/dev/null || true && docker exec challenge-postgres psql -U postgres -c "CREATE USER testuser WITH PASSWORD 'testpass';" 2>/dev/null || true && docker exec challenge-postgres psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE testdb TO testuser;" && docker exec challenge-postgres psql -U postgres -d testdb -c "ALTER SCHEMA public OWNER TO testuser;"
```

This command is **idempotent** - safe to run multiple times. Run it once, then just run tests anytime.

**Option 2: Use Dedicated Test Container**

```bash
# From project root
docker-compose -f docker-compose.test.yml up -d postgres-test
```

### Run Tests

```bash
# From extend-challenge-service directory
go test ./tests/integration/... -v

# Or run specific test
go test ./tests/integration/... -v -run TestGetUserChallenges_EmptyProgress
```

### Using Makefile

```bash
# From extend-challenge-service directory
make test-integration-setup    # Start test database
make test-integration-run      # Run integration tests
make test-integration-teardown # Clean up

# Or all in one command
make test-integration
```

## Test Database Configuration

Integration tests connect to PostgreSQL with these credentials (defined in `setup_test.go`):

```go
const testDBURL = "postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable"
```

**Credentials:**
- Host: `localhost`
- Port: `5433`
- Database: `testdb`
- User: `testuser`
- Password: `testpass`

## Test Structure

### setup_test.go
- `TestMain()`: Manages test lifecycle (database connection, migrations, cleanup)
- `setupTestServer()`: Creates in-process gRPC server with real PostgreSQL (legacy)
- `setupHTTPTestServer()`: **Creates HTTP handler with grpc-gateway (recommended)**
- `setupTestServerWithMockDB()`: Creates in-process gRPC server with mock repository
- `createAuthContext()`: Helper to create authenticated gRPC context

### Test Files

**HTTP Tests (Recommended):**
- `challenge_http_test.go`: HTTP tests for challenges and claiming rewards
- `http_grpc_parity_test.go`: Feature parity tests for HTTP vs gRPC handlers

**Legacy gRPC Tests (Being Phased Out):**
- `challenge_test.go`: gRPC tests for GetUserChallenges endpoint
- `initialize_test.go`: gRPC tests for InitializePlayer endpoint
- `set_goal_active_test.go`: gRPC tests for SetGoalActive endpoint

## Testing Philosophy: HTTP vs gRPC

### Why Test HTTP REST Endpoints?

Real production clients call **HTTP REST APIs**, not gRPC directly. Testing HTTP ensures:

✅ **Tests real client behavior** - Matches how actual clients call the service
✅ **Tests the full stack** - Including grpc-gateway translation layer
✅ **Catches more bugs** - HTTP→gRPC translation, routing, serialization issues
✅ **Better documentation** - Tests serve as API usage examples
✅ **No performance penalty** - `httptest` is fast (in-memory, no network)

### HTTP Testing Pattern

**Use `httptest` to test REST endpoints without running a real server:**

```go
func TestMyFeature_HTTP(t *testing.T) {
    // Setup HTTP handler (grpc-gateway + real database)
    handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
    defer cleanup()

    // Seed test data if needed
    seedCompletedGoal(t, testDB, "user123", "goal1", "challenge1")

    // Make HTTP request using httptest
    req := httptest.NewRequest(http.MethodGet, "/v1/challenges", nil)
    req.Header.Set("x-mock-user-id", "user123") // Mock auth
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    // Verify HTTP response
    assert.Equal(t, http.StatusOK, w.Code)

    var resp map[string]interface{}
    err := json.NewDecoder(w.Body).Decode(&resp)
    require.NoError(t, err)

    // Assert on JSON response (use camelCase field names)
    challenges, ok := resp["challenges"].([]interface{})
    require.True(t, ok)
    assert.Len(t, challenges, 2)

    // Verify mock expectations
    mockRewardClient.AssertExpectations(t)
}
```

### Key Differences: HTTP vs gRPC Tests

| Aspect | HTTP Tests | gRPC Tests |
|--------|-----------|-----------|
| **Setup** | `setupHTTPTestServer()` | `setupTestServer()` |
| **Request** | `httptest.NewRequest()` | `client.SomeEndpoint(ctx, req)` |
| **Auth** | `req.Header.Set("x-mock-user-id", ...)` | `createAuthContext(userID, ns)` |
| **Response** | JSON `map[string]interface{}` | Protobuf struct |
| **Field Names** | `camelCase` (grpc-gateway) | `snake_case` |
| **Status** | `w.Code` (200, 404, 500) | `err != nil` |

### JSON Field Naming Convention

grpc-gateway uses **camelCase** for JSON fields (not snake_case):

```go
// ✅ Correct - camelCase
challengeId := challenge["challengeId"]
completedAt := goal["completedAt"]

// ❌ Wrong - snake_case
challengeId := challenge["challenge_id"]  // nil!
completedAt := goal["completed_at"]       // nil!
```

### Migration Strategy

**Old gRPC tests → New HTTP tests:**

1. Copy test to `*_http_test.go`
2. Replace `setupTestServer()` → `setupHTTPTestServer()`
3. Replace gRPC calls with `httptest` HTTP calls
4. Update assertions for JSON responses (use camelCase)
5. Run both versions until HTTP tests are stable
6. Remove old gRPC tests

## Test Lifecycle

1. **TestMain** runs once before all tests
2. Connects to PostgreSQL at `localhost:5433`
3. Waits up to 30 seconds for database to be ready
4. Applies all migrations from `../../migrations/`
5. Runs all tests (serial execution to avoid conflicts)
6. Rolls back all migrations
7. Closes database connection

Each test calls `setupTestServer()` which:
1. Truncates `user_goal_progress` table for isolation
2. Loads challenge config from `../../config/challenges.test.json`
3. Initializes in-memory cache, real repository, mock reward client
4. Creates in-process gRPC server with test auth interceptor
5. Returns gRPC client for testing

## Database Isolation

- **Table truncation**: Each test starts with clean `user_goal_progress` table
- **No transactions**: Tests use real database commits (no rollback isolation)
- **Serial execution**: Tests run one at a time (`-p 1`) to avoid conflicts

## Troubleshooting

### Database Connection Timeout

**Error:** `panic: Database not ready: timeout waiting for database`

**Solution:** Ensure PostgreSQL is running and test database is created:

```bash
# Check if postgres is running
docker ps | grep postgres

# If not running, start it
docker-compose up -d postgres

# Create test database (one-time setup)
docker exec challenge-postgres psql -U postgres -c "CREATE DATABASE testdb;"
docker exec challenge-postgres psql -U postgres -c "CREATE USER testuser WITH PASSWORD 'testpass';"
docker exec challenge-postgres psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE testdb TO testuser;"
docker exec challenge-postgres psql -U postgres -d testdb -c "ALTER SCHEMA public OWNER TO testuser;"
```

### Migration Permission Errors

**Error:** `pq: permission denied for schema public`

**Solution:** Make testuser owner of public schema:

```bash
docker exec challenge-postgres psql -U postgres -d testdb -c "ALTER SCHEMA public OWNER TO testuser;"
```

### Port Already in Use

**Error:** `Bind for 0.0.0.0:5433 failed: port is already allocated`

**Solution:** Main postgres container already uses 5433. Use Option 1 (main container) instead of dedicated test container, or stop main container first:

```bash
docker-compose down postgres
docker-compose -f docker-compose.test.yml up -d postgres-test
```

### Tests Fail After Config Changes

If you modify `../../config/challenges.test.json`, restart tests to reload config:

```bash
# Kill any running test processes
pkill -f "go test.*integration"

# Re-run tests
go test ./tests/integration/... -v
```

## Manual Database Inspection

Connect to test database to inspect data:

```bash
# Using psql
psql -h localhost -p 5433 -U testuser -d testdb
# Password: testpass

# Check test data
testdb=> SELECT * FROM user_goal_progress;
testdb=> SELECT * FROM schema_migrations;
```

## Test Configuration

The tests use a separate config file with predictable test data:

**File:** `../../config/challenges.test.json`

**Contains:**
- 2 challenges: `winter-challenge-2025`, `seasonal-event-spring-2025`
- 5 total goals across both challenges
- Various goal types: absolute, daily, count-per-day
- Mix of default/non-default goals for testing assignment logic

## Coverage

Integration tests currently cover:
- Challenge retrieval with/without user progress
- Reward claiming (success, already claimed, not completed)
- Player initialization with default goal assignment
- Goal activation/deactivation (M3 Phase 6)
- Concurrent operations
- Database error handling

**Current Coverage:** 64.0% (tests/integration package)

## Best Practices

1. **Always truncate**: Use `setupTestServer()` which auto-truncates tables
2. **Unique user IDs**: Use descriptive, unique user IDs per test (e.g., `test-user-claim-success`)
3. **Clean up**: Tests clean up automatically via `defer cleanup()`
4. **Test isolation**: Don't depend on data from other tests
5. **Use test config**: Always use `challenges.test.json` for predictable test data
6. **Mock external calls**: RewardClient is mocked - use `mockRewardClient.On()` to set expectations

## Adding New Tests

### Recommended: HTTP Integration Tests

**New tests should use HTTP** to match real client behavior:

```go
func TestMyNewFeature_HTTP(t *testing.T) {
    // Setup HTTP handler (auto-truncates tables, loads config)
    handler, mockRewardClient, cleanup := setupHTTPTestServer(t)
    defer cleanup()

    // Seed test data if needed
    seedCompletedGoal(t, testDB, "user123", "goal1", "challenge1")

    // Make HTTP request
    req := httptest.NewRequest(http.MethodPost, "/v1/challenges/challenge1/goals/goal1/claim", nil)
    req.Header.Set("x-mock-user-id", "user123")
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    // Assert HTTP response
    assert.Equal(t, http.StatusOK, w.Code)

    var resp map[string]interface{}
    err := json.NewDecoder(w.Body).Decode(&resp)
    require.NoError(t, err)

    // Use camelCase for JSON fields
    assert.Equal(t, "claimed", resp["status"])
    assert.NotEmpty(t, resp["claimedAt"])

    // Verify mock calls if needed
    mockRewardClient.AssertExpectations(t)
}
```

### Legacy: gRPC Integration Tests (For Reference Only)

```go
func TestMyNewFeature(t *testing.T) {
    // Setup (auto-truncates tables, loads config)
    client, mockRewardClient, cleanup := setupTestServer(t)
    defer cleanup()

    // Create authenticated context
    ctx := createAuthContext("my-test-user", "test-namespace")

    // Make gRPC call
    resp, err := client.SomeEndpoint(ctx, &pb.SomeRequest{})

    // Assert results
    require.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, expectedValue, resp.SomeField)

    mockRewardClient.AssertExpectations(t)
}
```

## Related Documentation

- Main testing docs: `../../CLAUDE.md` (Testing Strategy section)
- Test specification: `../../../docs/TECH_SPEC_TESTING.md`
- Database schema: `../../../docs/TECH_SPEC_DATABASE.md`
- Challenge config format: `../../../docs/TECH_SPEC_CONFIGURATION.md`
