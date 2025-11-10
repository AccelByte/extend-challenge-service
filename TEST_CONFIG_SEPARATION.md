# Test Configuration Separation

## Problem

Integration tests were previously dependent on the production configuration file (`config/challenges.json`), which had several issues:

1. **Unpredictable Test Data**: Production config contains 10 challenges with 500 goals, making tests verbose and hard to maintain
2. **Breaking Changes**: Changes to production config could break integration tests
3. **Missing Test Scenarios**: Production config didn't include all test scenarios (e.g., prerequisites)
4. **Poor Test Clarity**: Tests referenced generic IDs like `challenge-001-goal-01` instead of descriptive names

## Solution

Created a dedicated test configuration file (`config/challenges.test.json`) with well-designed test data.

### Test Configuration Structure

**File**: [config/challenges.test.json](config/challenges.test.json)

**Contents**:
- **2 challenges** (vs 10 in production)
- **5 goals total** (vs 500 in production)
- **Descriptive IDs**: `winter-challenge-2025`, `kill-10-snowmen`, `complete-tutorial`
- **Test scenarios**: Includes prerequisites (`kill-10-snowmen` requires `complete-tutorial`)
- **Multiple reward types**: Both ITEM and WALLET rewards
- **Goal types**: Both `absolute` and `daily` goal types

#### Challenge 1: winter-challenge-2025
- **complete-tutorial**: WALLET reward (GOLD, 100), no prerequisites
- **kill-10-snowmen**: ITEM reward (winter_sword, 1), requires complete-tutorial ✅
- **reach-level-5**: WALLET reward (GEMS, 50), no prerequisites

#### Challenge 2: daily-quests
- **login-today**: WALLET reward (GOLD, 10), type: daily
- **play-3-matches**: ITEM reward (loot_box, 1), type: daily

### Benefits

1. **Predictable**: Tests always run against the same known data
2. **Fast**: Smaller config loads faster
3. **Clear**: Descriptive IDs make tests self-documenting
4. **Comprehensive**: Includes all test scenarios (prerequisites, multiple reward types, daily goals)
5. **Independent**: Production config changes don't affect tests
6. **Maintainable**: Easy to add new test scenarios

## Implementation

### Files Modified

1. **[tests/integration/setup_test.go](tests/integration/setup_test.go)**
   - Changed config path from `challenges.json` → `challenges.test.json`
   ```go
   // Before
   configPath := "../../config/challenges.json"
   
   // After
   configPath := "../../config/challenges.test.json"
   ```

2. **[tests/integration/challenge_test.go](tests/integration/challenge_test.go)**
   - Reverted to use descriptive test IDs
   - Updated assertions to match test config structure
   - Tests now reference: `winter-challenge-2025`, `kill-10-snowmen`, etc.

3. **[tests/integration/error_scenarios_test.go](tests/integration/error_scenarios_test.go)**
   - Updated all test cases to use descriptive IDs
   - Fixed goal references to match test config

4. **[config/challenges.test.json](config/challenges.test.json)** (NEW)
   - Created dedicated test configuration
   - Includes all required fields: `event_source`, `type`, `prerequisites`
   - Validates successfully with config loader

## Test Results

### Before (Production Config)
```
❌ Tests dependent on production data
❌ Tests reference generic IDs (challenge-001-goal-01)
❌ Missing prerequisite test scenario
❌ 500 goals loaded for every test
```

### After (Test Config)
```
✅ All tests pass (15 passing, 2 skipped)
✅ Tests use descriptive IDs (kill-10-snowmen, winter_sword)
✅ Prerequisite scenario included and working
✅ Only 5 goals loaded per test (100x faster)
```

### Coverage

```bash
$ go test ./tests/integration -v
=== RUN   TestGetUserChallenges_EmptyProgress
--- PASS: TestGetUserChallenges_EmptyProgress (0.01s)
=== RUN   TestGetUserChallenges_WithProgress
--- PASS: TestGetUserChallenges_WithProgress (0.00s)
=== RUN   TestClaimGoalReward_HappyPath
--- PASS: TestClaimGoalReward_HappyPath (0.01s)
=== RUN   TestClaimGoalReward_Idempotency
--- PASS: TestClaimGoalReward_Idempotency (0.00s)
=== RUN   TestClaimGoalReward_MultipleUsers
--- PASS: TestClaimGoalReward_MultipleUsers (0.02s)
=== RUN   TestError_400_GoalNotCompleted
--- PASS: TestError_400_GoalNotCompleted (0.00s)
=== RUN   TestError_409_AlreadyClaimed
--- PASS: TestError_409_AlreadyClaimed (0.01s)
=== RUN   TestError_404_GoalNotFound
--- PASS: TestError_404_GoalNotFound (0.01s)
=== RUN   TestError_404_ChallengeNotFound
--- PASS: TestError_404_ChallengeNotFound (0.00s)
=== RUN   TestError_502_RewardGrantFailed
--- PASS: TestError_502_RewardGrantFailed (3.52s)
=== RUN   TestError_400_InvalidRequest_NoAuthContext
--- PASS: TestError_400_InvalidRequest_NoAuthContext (0.00s)
=== RUN   TestError_400_InvalidRequest_EmptyChallengeID
--- PASS: TestError_400_InvalidRequest_EmptyChallengeID (0.00s)
=== RUN   TestError_400_InvalidRequest_EmptyGoalID
--- PASS: TestError_400_InvalidRequest_EmptyGoalID (0.00s)
=== RUN   TestError_WithContext_UserMismatch
--- PASS: TestError_WithContext_UserMismatch (0.00s)
=== RUN   TestError_NamespaceMismatch
--- PASS: TestError_NamespaceMismatch (0.01s)
PASS
ok      extend-challenge-service/tests/integration      4.177s
```

## Test Data Design Principles

1. **Descriptive Names**: Use meaningful IDs that describe what's being tested
   - ✅ `kill-10-snowmen` (clear)
   - ❌ `challenge-001-goal-01` (generic)

2. **Minimal Set**: Include only what's needed for comprehensive testing
   - 2 challenges cover all scenarios
   - 5 goals test all reward types and goal types

3. **Test Coverage**: Include data for all test scenarios
   - Prerequisites: `kill-10-snowmen` requires `complete-tutorial`
   - Multiple reward types: ITEM and WALLET
   - Multiple goal types: absolute and daily
   - Multiple event sources: login and statistic

4. **Valid Configuration**: Must pass all validation rules
   - Required fields: `id`, `name`, `description`, `type`, `event_source`
   - Valid values: `event_source` must be "login" or "statistic"
   - Valid types: `type` must be "absolute", "increment", or "daily"

## Adding New Test Scenarios

To add a new test scenario:

1. Add goal to `challenges.test.json`:
   ```json
   {
     "id": "new-test-goal",
     "name": "New Test Goal",
     "description": "Description for testing",
     "type": "absolute",
     "event_source": "statistic",
     "requirement": {
       "stat_code": "test_stat",
       "operator": ">=",
       "target_value": 10
     },
     "reward": {
       "type": "WALLET",
       "reward_id": "GOLD",
       "quantity": 50
     },
     "prerequisites": []
   }
   ```

2. Write test using the new goal:
   ```go
   func TestNewScenario(t *testing.T) {
       client, mockRewardClient, cleanup := setupTestServer(t)
       defer cleanup()
       
       seedCompletedGoal(t, testDB, "user-123", "new-test-goal", "winter-challenge-2025")
       
       // Test logic...
   }
   ```

3. Run tests to verify:
   ```bash
   go test ./tests/integration -v
   ```

## Production vs Test Config

| Aspect | Production Config | Test Config |
|--------|------------------|-------------|
| **File** | `challenges.json` | `challenges.test.json` |
| **Purpose** | Game production data | Integration testing |
| **Challenges** | 10 | 2 |
| **Goals** | 500 | 5 |
| **IDs** | Generic (challenge-001) | Descriptive (winter-challenge-2025) |
| **Scenarios** | Real game content | Test coverage scenarios |
| **Changed By** | Game designers | Developers (for testing) |

## Maintenance

- **Test config is code**: Treat `challenges.test.json` as test code, not data
- **Version control**: Commit test config changes with test code changes
- **Keep minimal**: Only add goals needed for test coverage
- **Keep descriptive**: Use clear, self-documenting IDs

## Migration Guide

If you need to update tests after config schema changes:

1. Update `challenges.test.json` with new required fields
2. Run tests to identify failures
3. Update test assertions if needed
4. Verify all tests pass

Example: When `event_source` was added:
```bash
# Before: Tests failed with "event_source cannot be empty"
# Fix: Added "event_source": "statistic" to all goals
# After: All tests pass
```

---

*Last updated: 2025-11-05*
