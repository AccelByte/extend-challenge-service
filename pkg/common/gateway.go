// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/credentials/insecure"

	pb "extend-challenge-service/pkg/pb"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

type Gateway struct {
	mux      *runtime.ServeMux
	basePath string
}

func NewGateway(ctx context.Context, grpcServerEndpoint string, basePath string) (*Gateway, error) {
	// Configure gRPC Gateway to forward x-mock-user-id header to gRPC metadata
	// This enables E2E testing with different user IDs when backend auth is disabled
	headerMatcher := func(key string) (string, bool) {
		switch strings.ToLower(key) {
		case "x-mock-user-id":
			// Forward mock user ID header for testing
			return key, true
		default:
			// Use default behavior for other headers
			return runtime.DefaultHeaderMatcher(key)
		}
	}

	// Use sonic marshaler for 2-3x faster JSON encoding (52% CPU time reduction)
	sonicMarshaler := NewSonicMarshaler()

	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(headerMatcher),
		runtime.WithMarshalerOption(runtime.MIMEWildcard, sonicMarshaler),
	)
	// Configure gRPC buffer sizes to reduce reallocations
	// Typical challenge list response: ~10-20KB, 32KB buffers provide headroom
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithWriteBufferSize(32 * 1024),        // 32KB write buffer
		grpc.WithReadBufferSize(32 * 1024),         // 32KB read buffer
		grpc.WithInitialWindowSize(64 * 1024),      // 64KB initial window size
		grpc.WithInitialConnWindowSize(128 * 1024), // 128KB connection window
	}
	err := pb.RegisterServiceHandlerFromEndpoint(ctx, mux, grpcServerEndpoint, opts)
	if err != nil {
		return nil, err
	}

	return &Gateway{
		mux:      mux,
		basePath: basePath,
	}, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip the base path, since the base_path configuration in protofile won't actually do the routing
	// Reference: https://github.com/grpc-ecosystem/grpc-gateway/pull/919/commits/1c34df861cfc0d6cb19ea617921d7d9eaa209977
	http.StripPrefix(g.basePath, g.mux).ServeHTTP(w, r)
}
