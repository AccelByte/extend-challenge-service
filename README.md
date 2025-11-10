# AccelByte Extend Challenge Service (Backend)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Test Coverage](https://img.shields.io/badge/Coverage-95%25-brightgreen)]()
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**REST API service for challenge queries and reward claiming.**

This service provides the REST API for the AccelByte Extend Challenge Platform, handling challenge queries, progress tracking, and reward claiming through AGS Platform Service integration.

---

## Service Overview

### Purpose

The Challenge Service exposes REST API endpoints that allow game clients to:
- Query available challenges and user progress
- Claim rewards for completed goals
- Authenticate using AGS IAM JWT tokens

### Architecture

```
Game Client
    │
    ▼
┌───────────────────────────────────┐
│   Challenge Service (This)        │
│                                   │
│  ┌─────────────────────────────┐ │
│  │  gRPC Service               │ │
│  │  • GetChallenges            │ │
│  │  • ClaimGoalReward          │ │
│  └──────────┬──────────────────┘ │
│             │                     │
│  ┌──────────▼──────────────────┐ │
│  │  gRPC-Gateway (HTTP)        │ │
│  │  • GET /v1/challenges       │ │
│  │  • POST /v1/.../claim       │ │
│  └─────────────────────────────┘ │
└───────────────────────────────────┘
    │
    ├─────► PostgreSQL (user_goal_progress)
    ├─────► Redis (config cache, optional)
    └─────► AGS Platform Service (reward grants)
```

### Key Features

✅ **Dual API** - gRPC + HTTP Gateway (OpenAPI/Swagger)
✅ **JWT Authentication** - AGS IAM token validation
✅ **Optimized Queries** - Custom HTTP handler for GET /v1/challenges (bypasses gRPC-Gateway)
✅ **Reward Integration** - AGS Platform SDK for ITEM and WALLET rewards
✅ **Observability** - Prometheus metrics, structured logging, OpenTelemetry traces
✅ **Production-Ready** - 95%+ test coverage, error handling, retry logic

---

## Quick Start

### Prerequisites

- **Go** 1.25+
- **PostgreSQL** 15+ (or use Docker Compose)
- **Redis** 6+ (optional for caching)
- **Make** (optional but recommended)

### 1. Clone Repository

```bash
git clone https://github.com/AccelByte/extend-challenge-service.git
cd extend-challenge-service
```

### 2. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install golang-migrate (for migrations)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### 3. Configure Environment

```bash
# Copy example config
cp .env.example .env

# Edit .env with your settings
vi .env
```

**Key environment variables**:
```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=challenge_db
DB_USER=postgres
DB_PASSWORD=postgres

# Server
GRPC_PORT=6565
HTTP_PORT=8000
METRICS_PORT=8080

# AccelByte AGS (for production)
AB_BASE_URL=https://your-environment.accelbyte.io
AB_CLIENT_ID=your-client-id
AB_CLIENT_SECRET=your-client-secret
AB_NAMESPACE=your-namespace

# Reward Client
REWARD_CLIENT_MODE=mock  # Use 'real' for AGS integration
```

### 4. Apply Database Migrations

```bash
# Run migrations
make migrate-up

# Or manually
migrate -database "postgresql://postgres:postgres@localhost:5432/challenge_db?sslmode=disable" \
        -path migrations up
```

### 5. Run Service

```bash
# Development mode
make run

# Or with Go directly
go run cmd/main.go
```

Service will start on:
- **gRPC**: `localhost:6565`
- **HTTP**: `localhost:8000`
- **Metrics**: `localhost:8080/metrics`

### 6. Test API

```bash
# Health check
curl http://localhost:8000/healthz

# List challenges (requires JWT token)
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     http://localhost:8000/v1/challenges

# Claim reward
curl -X POST \
     -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     http://localhost:8000/v1/challenges/{challenge_id}/goals/{goal_id}/claim
```

---

## API Endpoints

### REST API (HTTP Gateway)

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| GET | `/v1/challenges` | List all challenges with user progress | Required |
| GET | `/v1/challenges/{challenge_id}` | Get specific challenge with user progress | Required |
| POST | `/v1/challenges/{challenge_id}/goals/{goal_id}/claim` | Claim reward for completed goal | Required |
| GET | `/healthz` | Health check | None |

### gRPC API

| RPC | Description |
|-----|-------------|
| `GetChallenges` | List all challenges with user progress |
| `GetChallengeById` | Get specific challenge by ID |
| `ClaimGoalReward` | Claim reward for completed goal |

**Proto definition**: See `pkg/pb/challenge.proto`

---

## Configuration

### Challenge Configuration

Challenges are defined in `config/challenges.json`:

```json
{
  "challenges": [
    {
      "id": "daily-quests",
      "name": "Daily Quests",
      "description": "Complete daily tasks to earn rewards",
      "goals": [
        {
          "id": "daily-login",
          "name": "Daily Login",
          "description": "Log in to the game",
          "type": "daily",
          "event_source": "login",
          "requirement": {
            "target": 1
          },
          "reward": {
            "type": "ITEM",
            "reward_id": "daily-reward-box",
            "quantity": 1
          }
        }
      ]
    }
  ]
}
```

**Goal Types**:
- `absolute`: Replace progress with stat value (e.g., reach level 10)
- `increment`: Accumulate stat updates (e.g., play 10 matches)
- `daily`: Once per day, resets next day (e.g., daily login)

**Reward Types**:
- `ITEM`: Grants item entitlement via AGS Platform Service
- `WALLET`: Credits wallet currency via AGS Platform Service

See [Platform docs - TECH_SPEC_CONFIGURATION.md](https://github.com/AccelByte/extend-challenge-platform/blob/master/docs/TECH_SPEC_CONFIGURATION.md) for full schema.

---

## Development

### Project Structure

```
extend-challenge-service/
├── cmd/
│   └── main.go                    # Service entrypoint
├── internal/
│   ├── handler/                   # gRPC handlers
│   ├── httphandler/              # Optimized HTTP handlers
│   ├── service/                   # Business logic
│   ├── repository/                # Database layer
│   └── middleware/                # Auth, logging, metrics
├── pkg/
│   ├── pb/                        # Protobuf definitions
│   └── client/                    # External clients (AGS)
├── migrations/                    # Database migrations
├── config/                        # Configuration files
├── tests/
│   ├── unit/                      # Unit tests
│   └── integration/               # Integration tests
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

### Key Components

#### 1. gRPC Service (`internal/handler/`)
- Implements challenge service gRPC handlers
- Handles GetChallenges, GetChallengeById, ClaimGoalReward
- Uses business logic from `internal/service/`

#### 2. Optimized HTTP Handler (`internal/httphandler/`)
- Custom HTTP handler for `GET /v1/challenges` (bypasses gRPC-Gateway)
- 30% faster than gRPC-Gateway for high-traffic endpoint
- See [ADR_001_OPTIMIZED_HTTP_HANDLER.md](https://github.com/AccelByte/extend-challenge-platform/blob/master/docs/ADR_001_OPTIMIZED_HTTP_HANDLER.md)

#### 3. Business Logic (`internal/service/`)
- Core business logic for challenges and goals
- Coordinates between repository, cache, and reward client
- Implements retry logic for reward grants (3 attempts, exponential backoff)

#### 4. Repository Layer (`internal/repository/`)
- PostgreSQL database operations
- Implements `GoalRepository` interface from `extend-challenge-common`
- UPSERT and batch UPSERT for progress updates

#### 5. Reward Client (`pkg/client/`)
- `AGSRewardClient`: Real AGS Platform SDK integration
- `MockRewardClient`: Logs rewards without AGS calls (for local dev)
- Switchable via `REWARD_CLIENT_MODE` environment variable

---

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# With coverage
make test-coverage

# Specific package
go test ./internal/service/... -v
```

**Target**: 80%+ code coverage

### Integration Tests

```bash
# Setup test database (one-time)
make test-integration-setup

# Run integration tests
make test-integration-run

# Teardown test database
make test-integration-teardown

# All-in-one
make test-integration
```

Integration tests use testcontainers to spin up PostgreSQL automatically.

### Linting

```bash
# Run linter
make lint

# Auto-fix issues
make lint-fix
```

---

## Database

### Schema

**Table**: `user_goal_progress`

| Column | Type | Description |
|--------|------|-------------|
| `user_id` | VARCHAR(100) | User ID from AGS IAM |
| `goal_id` | VARCHAR(100) | Goal ID from config |
| `challenge_id` | VARCHAR(100) | Challenge ID from config |
| `namespace` | VARCHAR(100) | AGS namespace |
| `progress` | INT | Current progress value |
| `status` | VARCHAR(20) | `not_started`, `in_progress`, `completed`, `claimed` |
| `completed_at` | TIMESTAMP | When goal completed |
| `claimed_at` | TIMESTAMP | When reward claimed |
| `created_at` | TIMESTAMP | Row creation time |
| `updated_at` | TIMESTAMP | Last update time |

**Primary Key**: `(user_id, goal_id)`

**Index**: `idx_user_goal_progress_user_challenge` on `(user_id, challenge_id)`

### Migrations

Migrations are managed using [golang-migrate](https://github.com/golang-migrate/migrate):

```bash
# Apply all migrations
make migrate-up

# Rollback one migration
make migrate-down

# Create new migration
migrate create -ext sql -dir migrations -seq add_new_column
```

See [Platform docs - TECH_SPEC_DATABASE.md](https://github.com/AccelByte/extend-challenge-platform/blob/master/docs/TECH_SPEC_DATABASE.md) for detailed database design.

---

## Deployment

### Docker Build

```bash
# Build image
docker build -t challenge-service:latest .

# Run container
docker run -p 6565:6565 -p 8000:8000 \
  --env-file .env \
  challenge-service:latest
```

### AccelByte Extend Deployment

1. **Build and push image**:
   ```bash
   docker build -t your-registry/challenge-service:v1.0.0 .
   docker push your-registry/challenge-service:v1.0.0
   ```

2. **Deploy using extend-helper-cli**:
   ```bash
   extend-helper-cli deploy \
     --namespace your-namespace \
     --image your-registry/challenge-service:v1.0.0
   ```

3. **Configure environment variables** in Extend console:
   - Database credentials
   - AGS credentials (AB_CLIENT_ID, AB_CLIENT_SECRET)
   - Set `REWARD_CLIENT_MODE=real` for production

See [Platform docs - TECH_SPEC_DEPLOYMENT.md](https://github.com/AccelByte/extend-challenge-platform/blob/master/docs/TECH_SPEC_DEPLOYMENT.md) for detailed deployment guide.

---

## Observability

### Metrics

Prometheus metrics available at `http://localhost:8080/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `challenge_service_requests_total` | Counter | Total API requests |
| `challenge_service_request_duration_seconds` | Histogram | Request latency |
| `challenge_service_db_query_duration_seconds` | Histogram | Database query latency |
| `challenge_service_reward_grants_total` | Counter | Total reward grants |
| `challenge_service_reward_grant_errors_total` | Counter | Failed reward grants |

### Logging

Structured logging using [logrus](https://github.com/sirupsen/logrus):

```json
{
  "level": "info",
  "msg": "Processing claim request",
  "user_id": "abc123",
  "goal_id": "daily-login",
  "challenge_id": "daily-quests",
  "namespace": "mygame",
  "timestamp": "2025-11-10T10:30:00Z"
}
```

### Tracing

OpenTelemetry traces exported to Zipkin (if configured):

```bash
OTEL_EXPORTER_ZIPKIN_ENDPOINT=http://zipkin:9411/api/v2/spans
```

---

## Performance

### Benchmarks

- **GET /v1/challenges**: < 200ms (p95) for 100 challenges, 10 goals each
- **POST /claim**: < 100ms (p95) excluding AGS Platform call
- **Database queries**: < 50ms (p95)

### Optimization Tips

1. **Use optimized HTTP handler** - Already implemented for `GET /v1/challenges`
2. **Enable Redis caching** - Cache challenge config in Redis (optional for M1)
3. **Database connection pooling** - Configure max connections (default: 25)
4. **Horizontal scaling** - Run multiple replicas behind load balancer

See [Platform docs - PERFORMANCE_BASELINE.md](https://github.com/AccelByte/extend-challenge-platform/blob/master/docs/PERFORMANCE_BASELINE.md) for detailed benchmarks.

---

## Dependencies

### Core Dependencies

- **extend-challenge-common** v0.8.0 - Shared library
- **accelbyte-go-sdk** v0.80.0 - AGS Platform SDK
- **grpc-ecosystem/grpc-gateway** v2.26.3 - HTTP Gateway
- **lib/pq** v1.10.9 - PostgreSQL driver
- **golang-migrate** v4.19.0 - Database migrations
- **prometheus/client_golang** v1.22.0 - Metrics

### Version Management

The service depends on `extend-challenge-common` via Go modules:

```go
require github.com/AccelByte/extend-challenge-common v0.8.0
```

To update common library:
```bash
go get github.com/AccelByte/extend-challenge-common@v0.9.0
go mod tidy
```

---

## Troubleshooting

### Database connection failed

**Error**: `connection refused` or `timeout`

**Solution**:
1. Verify PostgreSQL is running: `pg_isready -h localhost -p 5432`
2. Check credentials in `.env`
3. Ensure database exists: `psql -U postgres -c "CREATE DATABASE challenge_db;"`

### JWT authentication failed

**Error**: `401 Unauthorized` or `invalid token`

**Solution**:
1. Verify JWT token is valid (not expired)
2. Check `AB_BASE_URL` matches token issuer
3. For local dev, use demo app to generate valid tokens

### Reward grant failed

**Error**: `502 Bad Gateway` or `reward grant failed`

**Solution**:
1. Check AGS credentials (`AB_CLIENT_ID`, `AB_CLIENT_SECRET`)
2. Verify service account has permissions (entitlement, wallet)
3. Ensure item/currency exists in AGS Platform
4. Check logs for retry attempts (service retries 3 times automatically)

---

## Contributing

See [Platform repo - CONTRIBUTING.md](https://github.com/AccelByte/extend-challenge-platform/blob/master/CONTRIBUTING.md)

---

## License

[Apache 2.0 License](LICENSE)

---

## Links

- **Platform Repo**: https://github.com/AccelByte/extend-challenge-platform
- **Common Library**: https://github.com/AccelByte/extend-challenge-common
- **Event Handler**: https://github.com/AccelByte/extend-challenge-event-handler
- **Demo App**: https://github.com/AccelByte/extend-challenge-demo-app
- **AccelByte Docs**: https://docs.accelbyte.io/extend/
