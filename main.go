// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-openapi/loads"

	"extend-challenge-service/pkg/cache"
	"extend-challenge-service/pkg/client"
	"extend-challenge-service/pkg/common"
	"extend-challenge-service/pkg/handler"
	"extend-challenge-service/pkg/mapper"
	"extend-challenge-service/pkg/migrations"
	pb "extend-challenge-service/pkg/pb"
	"extend-challenge-service/pkg/server"

	commonCache "github.com/AccelByte/extend-challenge-common/pkg/cache"
	commonClient "github.com/AccelByte/extend-challenge-common/pkg/client"
	commonConfig "github.com/AccelByte/extend-challenge-common/pkg/config"
	commonDB "github.com/AccelByte/extend-challenge-common/pkg/db"
	commonRepo "github.com/AccelByte/extend-challenge-common/pkg/repository"

	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/factory"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/repository"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/platform"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	sdkAuth "github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth"
	prometheusGrpc "github.com/grpc-ecosystem/go-grpc-prometheus"
	prometheusCollectors "github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	metricsEndpoint     = "/metrics"
	metricsPort         = 8080
	grpcServerPort      = 6565
	grpcGatewayHTTPPort = 8000
)

var (
	serviceName = common.GetEnv("OTEL_SERVICE_NAME", "ExtendCustomServiceGo")
	logLevelStr = common.GetEnv("LOG_LEVEL", logrus.InfoLevel.String())
	basePath    = common.GetBasePath()
)

func main() {
	logrus.Infof("Starting %s...", serviceName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logrusLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		logrusLevel = logrus.InfoLevel
	}
	logrusLogger := logrus.New()
	logrusLogger.SetLevel(logrusLevel)

	loggingOptions := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall, logging.PayloadReceived, logging.PayloadSent),
		logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
			if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
				return logging.Fields{"traceID", span.TraceID().String()}
			}

			return nil
		}),
		logging.WithLevels(logging.DefaultClientCodeToLevel),
		logging.WithDurationField(logging.DurationToDurationField),
	}

	unaryServerInterceptors := []grpc.UnaryServerInterceptor{
		prometheusGrpc.UnaryServerInterceptor,
		logging.UnaryServerInterceptor(common.InterceptorLogger(logrusLogger), loggingOptions...),
	}
	streamServerInterceptors := []grpc.StreamServerInterceptor{
		prometheusGrpc.StreamServerInterceptor,
		logging.StreamServerInterceptor(common.InterceptorLogger(logrusLogger), loggingOptions...),
	}

	// Preparing the IAM authorization
	var tokenRepo repository.TokenRepository = sdkAuth.DefaultTokenRepositoryImpl()
	var configRepo repository.ConfigRepository = sdkAuth.DefaultConfigRepositoryImpl()
	var refreshRepo repository.RefreshTokenRepository = &sdkAuth.RefreshTokenImpl{RefreshRate: 0.8, AutoRefresh: true}

	oauthService := iam.OAuth20Service{
		Client:                 factory.NewIamClient(configRepo),
		TokenRepository:        tokenRepo,
		RefreshTokenRepository: refreshRepo,
		ConfigRepository:       configRepo,
	}

	// Always register auth interceptor (it handles both enabled and disabled auth modes)
	permissionExtractor := common.NewProtoPermissionExtractor()
	unaryServerInterceptor := common.NewUnaryAuthServerIntercept(permissionExtractor)
	serverServerInterceptor := common.NewStreamAuthServerIntercept(permissionExtractor)

	unaryServerInterceptors = append(unaryServerInterceptors, unaryServerInterceptor)
	streamServerInterceptors = append(streamServerInterceptors, serverServerInterceptor)

	if strings.ToLower(common.GetEnv("PLUGIN_GRPC_SERVER_AUTH_ENABLED", "true")) == "true" {
		refreshInterval := common.GetEnvInt("REFRESH_INTERVAL", 600)
		common.Validator = common.NewTokenValidator(oauthService, time.Duration(refreshInterval)*time.Second, true)
		err := common.Validator.Initialize(ctx)
		if err != nil {
			logrus.Infof("%s", err.Error())
		}
		logrus.Infof("JWT authentication enabled with token validator")
	} else {
		logrus.Infof("JWT authentication disabled - using test user for local development")
	}

	// Create gRPC Server
	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(unaryServerInterceptors...),
		grpc.ChainStreamInterceptor(streamServerInterceptors...),
	)

	// Get namespace from environment
	namespace := common.GetEnv("AB_NAMESPACE", "accelbyte")
	logrus.Infof("Using namespace: %s", namespace)

	// Check if OAuth login is required (only for real mode or when auth is enabled)
	rewardMode := common.GetEnv("REWARD_CLIENT_MODE", "real")
	authEnabled := strings.ToLower(common.GetEnv("PLUGIN_GRPC_SERVER_AUTH_ENABLED", "true")) == "true"

	if rewardMode == "real" || authEnabled {
		// Configure IAM authorization
		clientId := configRepo.GetClientId()
		clientSecret := configRepo.GetClientSecret()
		err = oauthService.LoginClient(&clientId, &clientSecret)
		if err != nil {
			logrus.Fatalf("Error unable to login using clientId and clientSecret: %v", err)
		}
		logrus.Infof("Successfully logged in to AGS IAM")
	} else {
		logrus.Infof("Skipping AGS OAuth login (mock mode with auth disabled)")
	}

	// Initialize database connection (Decision Q10: shared database package)
	dbConfig := commonDB.NewConfigFromEnv()
	db, err := commonDB.Connect(dbConfig)
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logrus.Errorf("Failed to close database connection: %v", err)
		}
	}()
	logrus.Infof("Database connected successfully")

	// Run database migrations automatically on startup
	migrationsPath := common.GetEnv("MIGRATIONS_PATH", "file:///app/migrations")
	logrus.Infof("Running database migrations from: %s", migrationsPath)
	if err := migrations.RunMigrations(db, migrationsPath); err != nil {
		logrus.Errorf("Failed to run database migrations: %v", err)
		logrus.Fatalf("Service cannot start without successful migrations")
	}
	logrus.Infof("Database migrations completed successfully")

	// Load challenge configuration from challenges.json
	configPath := common.GetEnv("CHALLENGE_CONFIG_PATH", "config/challenges.json")
	slogLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	configLoader := commonConfig.NewConfigLoader(configPath, slogLogger)
	challengeConfig, err := configLoader.LoadConfig()
	if err != nil {
		logrus.Fatalf("Failed to load challenge config: %v", err)
	}
	logrus.Infof("Loaded %d challenges from config", len(challengeConfig.Challenges))

	// Initialize GoalCache with in-memory implementation
	goalCache := commonCache.NewInMemoryGoalCache(challengeConfig, configPath, slogLogger)
	logrus.Infof("GoalCache initialized with %d challenges", len(challengeConfig.Challenges))

	// Initialize pre-serialization cache for optimized challenge responses (Optimization 2)
	// This cache stores pre-marshaled JSON for static challenge data, reducing CPU by ~40%
	serializedCache := cache.NewSerializedChallengeCache()

	// Convert domain challenges to protobuf format for cache warm-up
	pbChallenges := make([]*pb.Challenge, 0, len(challengeConfig.Challenges))
	for _, domainChallenge := range goalCache.GetAllChallenges() {
		// Convert without user progress (progress will be injected at request time)
		pbChallenge, err := mapper.ChallengeToProto(domainChallenge, nil)
		if err != nil {
			logrus.Warnf("Failed to convert challenge %s for serialization cache: %v", domainChallenge.ID, err)
			continue
		}
		pbChallenges = append(pbChallenges, pbChallenge)
	}

	// Warm up the serialization cache with pre-marshaled challenge JSON
	if err := serializedCache.WarmUp(pbChallenges); err != nil {
		logrus.Fatalf("Failed to warm up serialization cache: %v", err)
	}

	challengeCount, goalCount, totalBytes := serializedCache.GetStats()
	logrus.Infof("Serialization cache warmed up: %d challenges, %d goals, %d bytes cached", challengeCount, goalCount, totalBytes)

	// Initialize GoalRepository with PostgreSQL implementation
	goalRepo := commonRepo.NewPostgresGoalRepository(db)
	logrus.Infof("GoalRepository initialized")

	// Initialize Platform SDK services for reward granting (Phase 7)
	platformClient := factory.NewPlatformClient(configRepo)
	entitlementService := &platform.EntitlementService{
		Client:           platformClient,
		TokenRepository:  tokenRepo,
		ConfigRepository: configRepo,
	}
	walletService := &platform.WalletService{
		Client:           platformClient,
		TokenRepository:  tokenRepo,
		ConfigRepository: configRepo,
	}
	logrus.Infof("Platform SDK services initialized (EntitlementService, WalletService)")

	// Create RewardClient based on REWARD_CLIENT_MODE environment variable (already read above)
	var rewardClient commonClient.RewardClient

	switch rewardMode {
	case "mock":
		rewardClient = commonClient.NewDevMockRewardClient()
		logrus.Warnf("Using DevMockRewardClient (for local development only - rewards will be logged but not granted)")
	case "real":
		rewardClient = client.NewAGSRewardClient(entitlementService, walletService, logrusLogger)
		logrus.Infof("AGSRewardClient initialized")
	default:
		logrus.Fatalf("Invalid REWARD_CLIENT_MODE: %s (must be 'mock' or 'real')", rewardMode)
	}

	// Create ChallengeServiceServer with all dependencies
	challengeServiceServer := server.NewChallengeServiceServer(
		goalCache,
		goalRepo,
		rewardClient,
		db,
		namespace,
	)

	// Register Challenge Service with gRPC server
	pb.RegisterServiceServer(s, challengeServiceServer)
	logrus.Infof("ChallengeService registered with gRPC server")

	// Enable gRPC Reflection
	reflection.Register(s)

	// Enable gRPC Health Check
	grpc_health_v1.RegisterHealthServer(s, health.NewServer())

	// Create a new HTTP server for the gRPC-Gateway
	grpcGateway, err := common.NewGateway(ctx, fmt.Sprintf("localhost:%d", grpcServerPort), basePath)
	if err != nil {
		logrus.Fatalf("Failed to create gRPC-Gateway: %v", err)
	}

	// Start the gRPC-Gateway HTTP server with optimized challenge handler
	go func() {
		swaggerDir := "gateway/apidocs" // Path to swagger directory

		// Create optimized challenges handler (uses pre-serialized cache for 40% CPU reduction)
		optimizedChallengesHandler := handler.NewOptimizedChallengesHandler(
			goalCache,
			goalRepo,
			serializedCache,
			namespace,
			authEnabled,
			common.Validator, // Token validator (may be nil if auth disabled)
		)

		// Create optimized initialize handler (bypasses Protobuf marshaling for 50% CPU reduction)
		optimizedInitializeHandler := handler.NewOptimizedInitializeHandler(
			goalCache,
			goalRepo,
			namespace,
			authEnabled,
			common.Validator, // Token validator (may be nil if auth disabled)
		)

		grpcGatewayHTTPServer := newGRPCGatewayHTTPServer(
			fmt.Sprintf(":%d", grpcGatewayHTTPPort),
			grpcGateway,
			logrus.New(),
			swaggerDir,
			optimizedChallengesHandler, // Pass optimized challenges handler
			optimizedInitializeHandler, // Pass optimized initialize handler
			basePath,
		)
		logrus.Infof("Starting gRPC-Gateway HTTP server on port %d (with optimized /v1/challenges and /v1/challenges/initialize endpoints)", grpcGatewayHTTPPort)
		if err := grpcGatewayHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Failed to run gRPC-Gateway HTTP server: %v", err)
		}
	}()

	prometheusGrpc.Register(s)

	// Register Prometheus Metrics
	prometheusRegistry := prometheus.NewRegistry()
	prometheusRegistry.MustRegister(
		prometheusCollectors.NewGoCollector(),
		prometheusCollectors.NewProcessCollector(prometheusCollectors.ProcessCollectorOpts{}),
		prometheusGrpc.DefaultServerMetrics,
	)

	go func() {
		mux := http.NewServeMux()
		mux.Handle(metricsEndpoint, promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{}))

		// Register pprof handlers
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		metricsServer := &http.Server{
			Addr:              fmt.Sprintf(":%d", metricsPort),
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Failed to run metrics server: %v", err)
		}
	}()
	logrus.Infof("Metrics endpoint: (:%d%s)", metricsPort, metricsEndpoint)
	logrus.Infof("Pprof endpoints: (:%d/debug/pprof/*)", metricsPort)

	// Set Tracer Provider
	tracerProvider, err := common.NewTracerProvider(serviceName)
	if err != nil {
		logrus.Fatalf("Failed to create tracer provider: %v", err)

		return
	}
	otel.SetTracerProvider(tracerProvider)
	defer func(ctx context.Context) {
		if err := tracerProvider.Shutdown(ctx); err != nil {
			logrus.Fatal(err)
		}
	}(ctx)

	// Set Text Map Propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			b3.New(),
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// Start gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcServerPort))
	if err != nil {
		logrus.Fatalf("Failed to listen to tcp:%d: %v", grpcServerPort, err)

		return
	}
	go func() {
		if err = s.Serve(lis); err != nil {
			logrus.Fatalf("Failed to run gRPC server: %v", err)

			return
		}
	}()

	logrus.Infof("%s started", serviceName)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	logrus.Infof("SIGTERM received")
}

func newGRPCGatewayHTTPServer(
	addr string,
	grpcGatewayHandler http.Handler,
	logger *logrus.Logger,
	swaggerDir string,
	optimizedChallengesHandler *handler.OptimizedChallengesHandler,
	optimizedInitializeHandler *handler.OptimizedInitializeHandler,
	basePath string,
) *http.Server {
	// Create a new ServeMux
	mux := http.NewServeMux()

	// Register optimized challenges endpoint BEFORE the catch-all gRPC-Gateway handler
	// This endpoint uses pre-serialized challenge data for ~40% CPU reduction
	// Path must match the protobuf definition: GET /v1/challenges
	optimizedChallengesPath := basePath + "/v1/challenges"
	mux.Handle(optimizedChallengesPath, optimizedChallengesHandler)
	logger.Infof("Registered optimized handler for %s (pre-serialization enabled)", optimizedChallengesPath)

	// Register optimized initialize endpoint BEFORE the catch-all gRPC-Gateway handler
	// This endpoint bypasses Protobuf marshaling for ~50% CPU reduction
	// Path must match the protobuf definition: POST /v1/challenges/initialize
	optimizedInitializePath := basePath + "/v1/challenges/initialize"
	mux.Handle(optimizedInitializePath, optimizedInitializeHandler)
	logger.Infof("Registered optimized handler for %s (direct JSON encoding enabled)", optimizedInitializePath)

	// Add the gRPC-Gateway handler as catch-all (must be last)
	// This handles all other endpoints including /v1/challenges/{id}/goals/{id}/claim
	mux.Handle("/", grpcGatewayHandler)

	// Serve Swagger UI and JSON
	serveSwaggerUI(mux)
	serveSwaggerJSON(mux, swaggerDir)

	// Add logging middleware
	loggedMux := loggingMiddleware(logger, mux)

	return &http.Server{
		Addr:              addr,
		Handler:           loggedMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ErrorLog:          log.New(os.Stderr, "httpSrv: ", log.LstdFlags), // Configure the logger for the HTTP server
	}
}

// loggingMiddleware is a middleware that logs HTTP requests
func loggingMiddleware(logger *logrus.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"duration": duration,
		}).Info("HTTP request")
	})
}

func serveSwaggerUI(mux *http.ServeMux) {
	swaggerUIDir := "third_party/swagger-ui"
	fileServer := http.FileServer(http.Dir(swaggerUIDir))
	swaggerUiPath := fmt.Sprintf("%s/apidocs/", basePath)
	mux.Handle(swaggerUiPath, http.StripPrefix(swaggerUiPath, fileServer))
}

func serveSwaggerJSON(mux *http.ServeMux, swaggerDir string) {
	fileHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matchingFiles, err := filepath.Glob(filepath.Join(swaggerDir, "*.swagger.json"))
		if err != nil || len(matchingFiles) == 0 {
			http.Error(w, "Error finding Swagger JSON file", http.StatusInternalServerError)

			return
		}

		firstMatchingFile := matchingFiles[0]
		swagger, err := loads.Spec(firstMatchingFile)
		if err != nil {
			http.Error(w, "Error parsing Swagger JSON file", http.StatusInternalServerError)

			return
		}

		// Update the base path
		swagger.Spec().BasePath = basePath

		updatedSwagger, err := swagger.Spec().MarshalJSON()
		if err != nil {
			http.Error(w, "Error serializing updated Swagger JSON", http.StatusInternalServerError)

			return
		}
		var prettySwagger bytes.Buffer
		err = json.Indent(&prettySwagger, updatedSwagger, "", "  ")
		if err != nil {
			http.Error(w, "Error formatting updated Swagger JSON", http.StatusInternalServerError)

			return
		}

		_, err = w.Write(prettySwagger.Bytes())
		if err != nil {
			http.Error(w, "Error writing Swagger JSON response", http.StatusInternalServerError)

			return
		}
	})
	apidocsPath := fmt.Sprintf("%s/apidocs/api.json", basePath)
	mux.Handle(apidocsPath, fileHandler)
}
