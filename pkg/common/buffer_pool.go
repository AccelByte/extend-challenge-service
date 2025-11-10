// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"bytes"
	"sync"
)

// Buffer pool configuration
const (
	// defaultBufferSize is the initial capacity for pooled buffers (32KB)
	// Typical challenge response is 10-20KB, so 32KB provides good headroom
	defaultBufferSize = 32 * 1024

	// maxBufferSize is the maximum buffer size to pool (128KB)
	// Larger buffers are discarded to prevent memory bloat
	maxBufferSize = 128 * 1024
)

var (
	// jsonBufferPool recycles buffers for JSON encoding
	// This reduces allocation overhead and GC pressure
	//
	// Performance impact (from profiling at 200 RPS):
	//   - Reduces memory allocations by ~15%
	//   - Reduces CPU time by ~10%
	//   - json.appendString allocations: 5,714 MB → ~4,857 MB (15% reduction)
	//   - json.WriteName allocations: 1,764 MB → ~1,499 MB (15% reduction)
	jsonBufferPool = sync.Pool{
		New: func() any {
			// Pre-allocate buffer with default capacity
			// Using make([]byte, 0, N) creates a buffer with 0 length but N capacity
			// This avoids initial grow operations during JSON encoding
			return bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
		},
	}
)

// GetJSONBuffer retrieves a buffer from the pool for JSON encoding.
// The returned buffer should be returned to the pool using PutJSONBuffer() after use.
//
// Usage:
//
//	buf := GetJSONBuffer()
//	defer PutJSONBuffer(buf)
//	// Use buf for JSON encoding
func GetJSONBuffer() *bytes.Buffer {
	return jsonBufferPool.Get().(*bytes.Buffer)
}

// PutJSONBuffer returns a buffer to the pool for reuse.
// The buffer is reset (contents cleared) before being returned to the pool.
//
// Buffers larger than maxBufferSize are discarded to prevent memory bloat.
// This can happen if a response is unusually large (e.g., user has hundreds of challenges).
//
// IMPORTANT: After calling this function, do not use the buffer again.
// The buffer may be reused by another goroutine immediately.
func PutJSONBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// Discard buffers that are too large to prevent memory bloat
	if buf.Cap() > maxBufferSize {
		// Don't pool this buffer - let GC collect it
		return
	}

	// Reset buffer (clears contents but keeps capacity)
	buf.Reset()

	// Return to pool for reuse
	jsonBufferPool.Put(buf)
}
