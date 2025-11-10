// Copyright (c) 2023 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

// Package common provides gRPC server interceptors for authentication and authorization.
//
// # JWT Authentication Flow
//
// The auth interceptor centralizes JWT validation and claim extraction:
//
//  1. Client sends request with "Authorization: Bearer <jwt>" header
//  2. gRPC Gateway or gRPC client includes JWT in metadata
//  3. Auth interceptor (NewUnaryAuthServerIntercept) intercepts the request
//  4. checkAuthorizationMetadata performs two operations:
//     a. Validates JWT using AccelByte validator (signature, expiration, permissions)
//     b. Decodes JWT payload and extracts user claims (user_id, namespace)
//  5. User claims are stored in context using context.WithValue()
//  6. Modified context is passed to the gRPC handler
//  7. Handler extracts user_id using common.GetUserIDFromContext(ctx)
//
// # Benefits of This Approach
//
// - Single point of JWT validation (DRY principle)
// - Handlers don't need to understand JWT format or base64 encoding
// - Easy to mock in tests (just set context values)
// - Consistent error handling across all endpoints
// - Performance: JWT decoded once per request, not multiple times
//
// # Usage in Service Handlers
//
//	func (s *ChallengeServiceServer) GetUserChallenges(ctx context.Context, req *pb.Request) (*pb.Response, error) {
//	    // Extract authenticated user ID from context (populated by auth interceptor)
//	    userID, err := common.GetUserIDFromContext(ctx)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Use userID for business logic
//	    challenges, err := s.service.GetChallenges(ctx, userID)
//	    ...
//	}
//
// # Testing
//
// In unit tests, simulate the auth interceptor by setting context values:
//
//	ctx := context.Background()
//	ctx = context.WithValue(ctx, common.ContextKeyUserID, "test-user-123")
//	ctx = context.WithValue(ctx, common.ContextKeyNamespace, "test-namespace")
//
// For integration tests with JWT validation, ensure the Validator is initialized.
package common

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	pb "extend-challenge-service/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/AccelByte/accelbyte-go-sdk/iam-sdk/pkg/iamclientmodels"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/service/iam"
	"github.com/AccelByte/accelbyte-go-sdk/services-api/pkg/utils/auth/validator"
	"github.com/pkg/errors"
)

var (
	Validator validator.AuthTokenValidator
)

// Context keys for storing user information extracted from JWT
type contextKey string

const (
	// ContextKeyUserID is the context key for storing the authenticated user ID from JWT
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyNamespace is the context key for storing the namespace from JWT
	ContextKeyNamespace contextKey = "namespace"
)

// JWTClaims represents the standard JWT claims we extract from AccelByte tokens
type JWTClaims struct {
	Sub       string `json:"sub"`       // Subject (user ID)
	Namespace string `json:"namespace"` // Namespace
	Exp       int64  `json:"exp"`       // Expiration time
	Iat       int64  `json:"iat"`       // Issued at
}

type ProtoPermissionExtractor interface {
	ExtractPermission(infoUnary *grpc.UnaryServerInfo, infoStream *grpc.StreamServerInfo) (permission *iam.Permission, err error)
}

func NewProtoPermissionExtractor() *ProtoPermissionExtractorImpl {
	return &ProtoPermissionExtractorImpl{}
}

type ProtoPermissionExtractorImpl struct{}

func (p *ProtoPermissionExtractorImpl) ExtractPermission(infoUnary *grpc.UnaryServerInfo, infoStream *grpc.StreamServerInfo) (*iam.Permission, error) {
	if infoUnary != nil && infoStream != nil {
		return nil, errors.New("both infoUnary and infoStream cannot be filled at the same time")
	}

	var serviceName string
	var methodName string
	var err error

	if infoUnary != nil {
		serviceName, methodName, err = parseFullMethod(infoUnary.FullMethod)
	} else if infoStream != nil {
		serviceName, methodName, err = parseFullMethod(infoStream.FullMethod)
	} else {
		return nil, errors.New("both infoUnary and infoStream are nil")
	}
	if err != nil {
		return nil, err
	}

	// Read the required permission stated in the proto file
	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(serviceName))
	if err != nil {
		return nil, err
	}

	serviceDesc := desc.(protoreflect.ServiceDescriptor)
	method := serviceDesc.Methods().ByName(protoreflect.Name(methodName))
	resource := proto.GetExtension(method.Options(), pb.E_Resource).(string)
	action := proto.GetExtension(method.Options(), pb.E_Action).(pb.Action)
	permission := wrapPermission(resource, int(action.Number()))

	if resource == "" {
		return nil, nil
	}

	return &permission, nil
}

func NewUnaryAuthServerIntercept(
	permissionExtractor ProtoPermissionExtractor,
) func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) { // nolint

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !skipCheckAuthorizationMetadata(info.FullMethod) {
			// Extract permission stated in the proto file
			permission, err := permissionExtractor.ExtractPermission(info, nil)
			if err != nil {
				return nil, err
			}

			// Validate JWT and extract user claims into context
			ctx, err = checkAuthorizationMetadata(ctx, permission)
			if err != nil {
				return nil, err
			}
		}

		return handler(ctx, req)
	}
}

func parseFullMethod(fullMethod string) (string, string, error) {
	// Define the regular expression according to example shown here https://github.com/grpc/grpc-java/issues/4726
	re := regexp.MustCompile(`^/([^/]+)/([^/]+)$`)
	matches := re.FindStringSubmatch(fullMethod)

	// Validate the match
	if matches == nil {
		return "", "", fmt.Errorf("invalid FullMethod format")
	}

	// Extract service and method names
	serviceName, methodName := matches[1], matches[2]

	if len(serviceName) == 0 {
		return "", "", fmt.Errorf("invalid FullMethod format: service name is empty")
	}

	if len(methodName) == 0 {
		return "", "", fmt.Errorf("invalid FullMethod format: method name is empty")
	}

	return serviceName, methodName, nil
}

func NewStreamAuthServerIntercept(
	permissionExtractor ProtoPermissionExtractor,
) func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !skipCheckAuthorizationMetadata(info.FullMethod) {
			// Extract permission stated in the proto file
			permission, err := permissionExtractor.ExtractPermission(nil, info)
			if err != nil {
				return err
			}

			// Validate JWT and extract user claims into context
			_, err = checkAuthorizationMetadata(ss.Context(), permission)
			if err != nil {
				return err
			}
			// Note: For stream interceptors, we can't modify the context passed to the handler
			// This is a limitation of gRPC's stream interceptor design
			// If needed, implement a wrapped ServerStream that returns the modified context
		}

		return handler(srv, ss)
	}
}

func skipCheckAuthorizationMetadata(fullMethod string) bool {
	if strings.HasPrefix(fullMethod, "/grpc.reflection.v1alpha.ServerReflection/") {
		return true
	}

	if strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/") {
		return true
	}

	return false
}

// checkAuthorizationMetadata validates the JWT token and extracts user claims into the context.
// It performs two key operations:
// 1. Validates the JWT token using the AccelByte validator (signature, expiration, permissions)
// 2. Decodes the JWT payload and stores user_id and namespace in the context for downstream handlers
//
// This centralizes JWT handling in the auth interceptor, so service handlers don't need to
// re-decode the JWT token. Handlers can simply extract user_id from context using GetUserIDFromContext().
//
// If Validator is nil (auth disabled via PLUGIN_GRPC_SERVER_AUTH_ENABLED=false), this function
// injects a test user into the context for local development and testing.
//
// Returns: Modified context with user claims, or error if validation/decoding fails
func checkAuthorizationMetadata(ctx context.Context, permission *iam.Permission) (context.Context, error) {
	// When auth is disabled (Validator not initialized), inject test user for local development
	// Check for mock user ID header to support E2E testing with different user IDs
	if Validator == nil {
		namespace := getNamespace()
		userID := "test-user-123" // Default for local development

		// Check for mock user ID header (for E2E testing)
		// This allows tests to specify different user IDs even when auth is disabled
		meta, found := metadata.FromIncomingContext(ctx)
		if found {
			// Try different possible header names (gRPC Gateway might transform the name)
			possibleKeys := []string{"x-mock-user-id", "X-Mock-User-Id", "grpcgateway-x-mock-user-id"}
			for _, key := range possibleKeys {
				if mockUserIDs := meta.Get(key); len(mockUserIDs) > 0 && mockUserIDs[0] != "" {
					userID = mockUserIDs[0]
					break
				}
			}
		}

		ctx = context.WithValue(ctx, ContextKeyUserID, userID)
		ctx = context.WithValue(ctx, ContextKeyNamespace, namespace)
		return ctx, nil
	}

	meta, found := metadata.FromIncomingContext(ctx)

	if !found {
		return ctx, status.Error(codes.Unauthenticated, "metadata is missing")
	}

	if _, ok := meta["authorization"]; !ok {
		return ctx, status.Error(codes.Unauthenticated, "authorization metadata is missing")
	}

	if len(meta["authorization"]) == 0 {
		return ctx, status.Error(codes.Unauthenticated, "authorization metadata length is 0")
	}

	authorization := meta["authorization"][0]
	token := strings.TrimPrefix(authorization, "Bearer ")
	namespace := getNamespace()

	// Validate JWT signature, expiration, and permissions using AccelByte validator
	err := Validator.Validate(token, permission, &namespace, nil)
	if err != nil {
		return ctx, status.Error(codes.PermissionDenied, err.Error())
	}

	// After successful validation, decode JWT payload to extract user claims
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return ctx, status.Errorf(codes.Internal, "failed to decode JWT claims: %v", err)
	}

	// Store user claims in context for downstream handlers
	ctx = context.WithValue(ctx, ContextKeyUserID, claims.Sub)
	ctx = context.WithValue(ctx, ContextKeyNamespace, claims.Namespace)

	return ctx, nil
}

// decodeJWTClaims decodes the JWT token payload and extracts standard claims.
// This is called AFTER token validation to extract user information.
// The token format is: header.payload.signature (base64url encoded)
func decodeJWTClaims(token string) (*JWTClaims, error) {
	// Split JWT into parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	// Decode payload (second part) - try RawURLEncoding first, then URLEncoding
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Fallback to standard URLEncoding if RawURLEncoding fails
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
		}
	}

	// Parse JSON claims
	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	// Validate required claims
	if claims.Sub == "" {
		return nil, fmt.Errorf("user ID (sub claim) is empty")
	}

	return &claims, nil
}

// GetUserIDFromContext extracts the authenticated user ID from the request context.
// This should be called by service handlers after the auth interceptor has validated the JWT.
// Returns empty string if user ID is not found in context (which shouldn't happen after auth).
func GetUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	if !ok || userID == "" {
		return "", status.Error(codes.Unauthenticated, "user ID not found in context")
	}
	return userID, nil
}

// GetNamespaceFromContext extracts the namespace from the request context.
// Returns empty string if namespace is not found in context.
func GetNamespaceFromContext(ctx context.Context) string {
	namespace, ok := ctx.Value(ContextKeyNamespace).(string)
	if !ok {
		return ""
	}
	return namespace
}

func getNamespace() string {
	return GetEnv("AB_NAMESPACE", "accelbyte")
}

func wrapPermission(resource string, action int) iam.Permission {
	return iam.Permission{
		Action:   action,
		Resource: resource,
	}
}

func NewTokenValidator(authService iam.OAuth20Service, refreshInterval time.Duration, validateLocally bool) validator.AuthTokenValidator {
	return &iam.TokenValidator{
		AuthService:     authService,
		RefreshInterval: refreshInterval,

		Filter:                nil,
		JwkSet:                nil,
		JwtClaims:             iam.JWTClaims{},
		JwtEncoding:           *base64.URLEncoding.WithPadding(base64.NoPadding),
		PublicKeys:            make(map[string]*rsa.PublicKey),
		LocalValidationActive: validateLocally,
		RevokedUsers:          make(map[string]time.Time),
		Roles:                 make(map[string]*iamclientmodels.ModelRolePermissionResponseV3),
	}
}
