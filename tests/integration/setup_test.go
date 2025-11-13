package integration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	serviceCommon "extend-challenge-service/pkg/common"
	pb "extend-challenge-service/pkg/pb"
	"extend-challenge-service/pkg/server"

	commonCache "github.com/AccelByte/extend-challenge-common/pkg/cache"
	commonClient "github.com/AccelByte/extend-challenge-common/pkg/client"
	commonConfig "github.com/AccelByte/extend-challenge-common/pkg/config"
	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"
)

var (
	testDB *sql.DB
	logger *slog.Logger
)

const (
	testDBURL      = "postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable"
	migrationsPath = "file://../../migrations"
)

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Setup logger
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Connect to test database (docker-compose postgres)
	var err error
	testDB, err = sql.Open("postgres", testDBURL)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to test DB: %v", err))
	}
	defer func() {
		if err := testDB.Close(); err != nil {
			logger.Warn("Failed to close test DB", "error", err)
		}
	}()

	// Wait for database to be ready
	if err := waitForDB(testDB, 30*time.Second); err != nil {
		panic(fmt.Sprintf("Database not ready: %v", err))
	}

	// Apply migrations once for all tests
	applyMigrations(testDB)

	// Run all tests (serial execution - Decision IQ7)
	code := m.Run()

	// Cleanup: Rollback migrations
	rollbackMigrations(testDB)

	os.Exit(code)
}

// waitForDB waits for the database to be ready
func waitForDB(db *sql.DB, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for database")
		case <-ticker.C:
			if err := db.Ping(); err == nil {
				logger.Info("Database connection established")
				return nil
			}
		}
	}
}

// applyMigrations applies database migrations from filesystem
func applyMigrations(db *sql.DB) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create migrate driver: %v", err))
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"testdb",
		driver,
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create migrate instance: %v", err))
	}

	// Apply all up migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		panic(fmt.Sprintf("Failed to apply migrations: %v", err))
	}

	logger.Info("Migrations applied successfully")
}

// rollbackMigrations rolls back all migrations (cleanup)
func rollbackMigrations(db *sql.DB) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.Warn("Failed to create migrate driver for rollback", "error", err)
		return
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"testdb",
		driver,
	)
	if err != nil {
		logger.Warn("Failed to create migrate instance for rollback", "error", err)
		return
	}

	// Rollback all migrations
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		logger.Warn("Failed to rollback migrations", "error", err)
	} else {
		logger.Info("Migrations rolled back successfully")
	}
}

// truncateTables clears all test data for isolation
func truncateTables(t *testing.T, db *sql.DB) {
	_, err := db.Exec("TRUNCATE user_goal_progress")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}
}

// testAuthInterceptor is a simple auth interceptor for tests that extracts
// user ID and namespace from gRPC metadata and injects into context.
// This simulates the real auth interceptor but without JWT validation.
func testAuthInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// Extract user ID and namespace from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if userIDs := md.Get("user-id"); len(userIDs) > 0 {
			ctx = context.WithValue(ctx, serviceCommon.ContextKeyUserID, userIDs[0])
		}
		if namespaces := md.Get("namespace"); len(namespaces) > 0 {
			ctx = context.WithValue(ctx, serviceCommon.ContextKeyNamespace, namespaces[0])
		}
	}

	return handler(ctx, req)
}

// setupTestServer creates in-process gRPC server with injected dependencies
func setupTestServer(t *testing.T) (pb.ServiceClient, *commonClient.MockRewardClient, func()) {
	// 1. Truncate tables for test isolation
	truncateTables(t, testDB)

	// 2. Load challenge config (use test-specific config for predictable test data)
	configPath := "../../config/challenges.test.json"
	configLoader := commonConfig.NewConfigLoader(configPath, logger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load challenge config: %v", err)
	}

	// 3. Initialize dependencies
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, logger)
	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)
	mockRewardClient := commonClient.NewMockRewardClient()

	// 4. Create ChallengeServiceServer with mocks
	challengeServer := server.NewChallengeServiceServer(
		goalCache,
		goalRepo,
		mockRewardClient,
		testDB,
		"test-namespace",
	)

	// 5. Create in-process gRPC server with test auth interceptor
	// Note: We use a simple test auth interceptor (not the full JWT validator)
	// to inject user ID/namespace from gRPC metadata into context
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(testAuthInterceptor),
	)
	pb.RegisterServiceServer(grpcServer, challengeServer)

	// 6. Create in-memory listener (bufconn) for testing
	const bufSize = 1024 * 1024
	listener := bufconn.Listen(bufSize)

	// 7. Start server in background
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// 8. Create client connected to in-memory listener
	// Note: Using grpc.Dial instead of grpc.NewClient because bufconn requires
	// the older API. grpc.NewClient doesn't work with bufconn's in-memory connections.
	//nolint:staticcheck // SA1019: grpc.Dial is deprecated but required for bufconn
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial in-memory server: %v", err)
	}

	client := pb.NewServiceClient(conn)

	// 9. Return client, mock, and cleanup function
	cleanup := func() {
		if err := conn.Close(); err != nil {
			t.Logf("Warning: failed to close connection: %v", err)
		}
		grpcServer.Stop()
		if err := listener.Close(); err != nil {
			t.Logf("Warning: failed to close listener: %v", err)
		}
	}

	return client, mockRewardClient, cleanup
}

// setupHTTPTestServer creates an HTTP handler with all REST endpoints for testing.
// This simulates the real production HTTP API that clients call.
//
// Returns:
//   - http.Handler: HTTP router with all REST endpoints
//   - *commonClient.MockRewardClient: Mock reward client for verifying reward grants
//   - func(): Cleanup function to call after test
func setupHTTPTestServer(t *testing.T) (http.Handler, *commonClient.MockRewardClient, func()) {
	// 1. Truncate tables for test isolation
	truncateTables(t, testDB)

	// 2. Load challenge config
	configPath := "../../config/challenges.test.json"
	configLoader := commonConfig.NewConfigLoader(configPath, logger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load challenge config: %v", err)
	}

	// 3. Initialize dependencies
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, logger)
	goalRepo := commonRepo.NewPostgresGoalRepository(testDB)
	mockRewardClient := commonClient.NewMockRewardClient()

	// 4. Create gRPC server (needed for grpc-gateway)
	challengeServer := server.NewChallengeServiceServer(
		goalCache,
		goalRepo,
		mockRewardClient,
		testDB,
		"test-namespace",
	)

	// 5. Create grpc-gateway mux
	ctx := context.Background()
	mux := runtime.NewServeMux()

	// 6. Register gRPC server with grpc-gateway
	err = pb.RegisterServiceHandlerServer(ctx, mux, challengeServer)
	if err != nil {
		t.Fatalf("Failed to register grpc-gateway handler: %v", err)
	}

	// 7. Wrap mux with auth middleware
	// In tests, we use x-mock-user-id header instead of JWT
	handler := testAuthMiddleware(mux)

	cleanup := func() {
		// No cleanup needed for HTTP handler (no connections to close)
	}

	return handler, mockRewardClient, cleanup
}

// testAuthMiddleware adds authentication context for HTTP tests.
// Extracts x-mock-user-id header and injects into request context.
func testAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract user ID from mock header
		userID := r.Header.Get("x-mock-user-id")
		if userID != "" {
			ctx := context.WithValue(r.Context(), serviceCommon.ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, serviceCommon.ContextKeyNamespace, "test-namespace")
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// MockGoalRepository is a mock implementation of repository.Repository for database failure testing
type MockGoalRepository struct {
	mock.Mock
}

func (m *MockGoalRepository) GetProgress(ctx context.Context, userID, goalID string) (*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) GetGoal(ctx context.Context, userID, goalID string) (*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*commonDomain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockGoalRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

// M3 Phase 4: Added activeOnly parameter
func (m *MockGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, challengeID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) UpsertProgress(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGoalRepository) UpdateProgress(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

func (m *MockGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	args := m.Called(ctx, userID, goalID)
	return args.Error(0)
}

func (m *MockGoalRepository) IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int, isDailyIncrement bool) error {
	args := m.Called(ctx, userID, goalID, challengeID, namespace, delta, targetValue, isDailyIncrement)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgress(ctx context.Context, progressList []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progressList)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, updates []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockGoalRepository) BatchIncrementProgress(ctx context.Context, increments []commonRepo.ProgressIncrement) error {
	args := m.Called(ctx, increments)
	return args.Error(0)
}

func (m *MockGoalRepository) GetProgressForUpdate(ctx context.Context, userID, goalID string) (*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID, goalIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) BulkInsert(ctx context.Context, progresses []*commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progresses)
	return args.Error(0)
}

func (m *MockGoalRepository) UpsertGoalActive(ctx context.Context, progress *commonDomain.UserGoalProgress) error {
	args := m.Called(ctx, progress)
	return args.Error(0)
}

// M3 Phase 9: Fast path optimization methods
func (m *MockGoalRepository) GetUserGoalCount(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockGoalRepository) GetActiveGoals(ctx context.Context, userID string) ([]*commonDomain.UserGoalProgress, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*commonDomain.UserGoalProgress), args.Error(1)
}

func (m *MockGoalRepository) BeginTx(ctx context.Context) (commonRepo.TxRepository, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(commonRepo.TxRepository), args.Error(1)
}

// setupTestServerWithMockDB creates in-process gRPC server with mock database for failure testing
func setupTestServerWithMockDB(t *testing.T) (pb.ServiceClient, *commonClient.MockRewardClient, *MockGoalRepository, func()) {
	// Load challenge config
	configPath := "../../config/challenges.test.json"
	configLoader := commonConfig.NewConfigLoader(configPath, logger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load challenge config: %v", err)
	}

	// Initialize dependencies with mocks
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, logger)
	mockGoalRepo := new(MockGoalRepository)
	mockRewardClient := commonClient.NewMockRewardClient()

	// Create ChallengeServiceServer with mocks
	challengeServer := server.NewChallengeServiceServer(
		goalCache,
		mockGoalRepo,
		mockRewardClient,
		nil, // no real DB needed
		"test-namespace",
	)

	// Create in-process gRPC server with test auth interceptor
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(testAuthInterceptor),
	)
	pb.RegisterServiceServer(grpcServer, challengeServer)

	// Create in-memory listener (bufconn) for testing
	const bufSize = 1024 * 1024
	listener := bufconn.Listen(bufSize)

	// Start server in background
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Create client connected to in-memory listener
	//nolint:staticcheck // SA1019: grpc.Dial is deprecated but required for bufconn
	conn, err := grpc.Dial("bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial in-memory server: %v", err)
	}

	client := pb.NewServiceClient(conn)

	// Return client, mocks, and cleanup function
	cleanup := func() {
		if err := conn.Close(); err != nil {
			t.Logf("Warning: failed to close connection: %v", err)
		}
		grpcServer.Stop()
		if err := listener.Close(); err != nil {
			t.Logf("Warning: failed to close listener: %v", err)
		}
	}

	return client, mockRewardClient, mockGoalRepo, cleanup
}
