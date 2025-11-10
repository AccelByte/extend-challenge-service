// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJSONBuffer(t *testing.T) {
	buf := GetJSONBuffer()

	require.NotNil(t, buf, "GetJSONBuffer should return a non-nil buffer")
	assert.Equal(t, 0, buf.Len(), "Buffer should be empty initially")
	assert.GreaterOrEqual(t, buf.Cap(), defaultBufferSize, "Buffer should have default capacity")

	// Return buffer to pool
	PutJSONBuffer(buf)
}

func TestPutJSONBuffer_ResetsBuffer(t *testing.T) {
	// Get buffer and write some data
	buf := GetJSONBuffer()
	_, err := buf.WriteString("test data")
	require.NoError(t, err)
	assert.Equal(t, 9, buf.Len(), "Buffer should contain written data")

	// Return to pool
	PutJSONBuffer(buf)

	// Get another buffer (should be the same one, now reset)
	buf2 := GetJSONBuffer()
	assert.Equal(t, 0, buf2.Len(), "Buffer from pool should be reset (empty)")

	PutJSONBuffer(buf2)
}

func TestPutJSONBuffer_DiscardsLargeBuffers(t *testing.T) {
	// Create a buffer larger than maxBufferSize
	largeData := make([]byte, maxBufferSize+1024)
	buf := bytes.NewBuffer(largeData)

	// This should not panic, but the buffer won't be pooled
	PutJSONBuffer(buf)

	// Get a new buffer from pool
	buf2 := GetJSONBuffer()

	// It should be a fresh buffer with default capacity, not the large one
	assert.LessOrEqual(t, buf2.Cap(), maxBufferSize, "Large buffer should not be pooled")

	PutJSONBuffer(buf2)
}

func TestPutJSONBuffer_NilBuffer(t *testing.T) {
	// Should not panic
	assert.NotPanics(t, func() {
		PutJSONBuffer(nil)
	}, "PutJSONBuffer should handle nil buffer gracefully")
}

func TestBufferPoolReuse(t *testing.T) {
	// Get multiple buffers
	buf1 := GetJSONBuffer()
	buf2 := GetJSONBuffer()
	buf3 := GetJSONBuffer()

	// Write different data to each
	_, err := buf1.WriteString("buffer1")
	require.NoError(t, err)
	_, err = buf2.WriteString("buffer2")
	require.NoError(t, err)
	_, err = buf3.WriteString("buffer3")
	require.NoError(t, err)

	// Return all to pool
	PutJSONBuffer(buf1)
	PutJSONBuffer(buf2)
	PutJSONBuffer(buf3)

	// Get new buffers - they should be reset
	newBuf1 := GetJSONBuffer()
	newBuf2 := GetJSONBuffer()
	newBuf3 := GetJSONBuffer()

	assert.Equal(t, 0, newBuf1.Len(), "Reused buffer should be empty")
	assert.Equal(t, 0, newBuf2.Len(), "Reused buffer should be empty")
	assert.Equal(t, 0, newBuf3.Len(), "Reused buffer should be empty")

	// Capacity should be preserved (or at least >= default)
	assert.GreaterOrEqual(t, newBuf1.Cap(), defaultBufferSize)
	assert.GreaterOrEqual(t, newBuf2.Cap(), defaultBufferSize)
	assert.GreaterOrEqual(t, newBuf3.Cap(), defaultBufferSize)

	PutJSONBuffer(newBuf1)
	PutJSONBuffer(newBuf2)
	PutJSONBuffer(newBuf3)
}

func TestBufferPoolConcurrency(t *testing.T) {
	// Test that buffer pool is safe for concurrent use
	const goroutines = 100
	const iterations = 100

	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				buf := GetJSONBuffer()
				_, err := buf.WriteString("concurrent test data")
				if err != nil {
					t.Errorf("Failed to write to buffer: %v", err)
				}
				PutJSONBuffer(buf)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func BenchmarkBufferPoolWithPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := GetJSONBuffer()
		_, _ = buf.WriteString("benchmark test data with some JSON content")
		PutJSONBuffer(buf)
	}
}

func BenchmarkBufferPoolWithoutPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
		_, _ = buf.WriteString("benchmark test data with some JSON content")
		// No pooling - buffer is garbage collected
	}
}
