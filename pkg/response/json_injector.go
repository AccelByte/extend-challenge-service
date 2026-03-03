// Copyright (c) 2023-2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package response

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
	"time"

	commonDomain "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// progressBufPool pools buffers used by writeProgressFields to eliminate
// per-goal buffer allocation in the hot path (processGoalsArray).
var progressBufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 160))
	},
}

// defaultProgressFieldsLiteral is the pre-computed default progress fields string.
var defaultProgressFieldsLiteral = []byte(`,"progress":0,"status":"not_started","completedAt":"","claimedAt":"","isActive":false,"expiresAt":"","expiresInSeconds":0`)

// goalIDFieldPattern is the byte pattern for goalId JSON field lookup.
// Package-level var avoids per-call []byte allocation in extractGoalIDRange.
var goalIDFieldPattern = []byte(`"goalId"`)

// goalsFieldPattern is the byte pattern for goals JSON field lookup.
var goalsFieldPattern = []byte(`"goals":`)

// InjectProgressIntoGoal injects user progress fields into a pre-serialized goal JSON.
//
// This uses zero-copy string manipulation instead of unmarshal/marshal cycle.
// Performance: ~100-200us vs ~2-3ms for unmarshal+marshal (15-30x faster)
//
// Used by BuildGoalResponse for single-goal responses (not the hot path).
// For the hot path (600-goal challenge responses), processGoalsArray writes
// directly to the parent buffer to avoid per-goal allocations.
func InjectProgressIntoGoal(
	staticJSON []byte,
	progress *commonDomain.UserGoalProgress,
) []byte {
	closingBraceIdx := bytes.LastIndexByte(staticJSON, '}')
	if closingBraceIdx == -1 {
		return staticJSON
	}

	// Build progress fields
	var progressFields []byte
	if progress == nil {
		progressFields = buildDefaultProgressFields()
	} else {
		progressFields = buildProgressFields(progress)
	}

	result := make([]byte, 0, len(staticJSON)+len(progressFields)+10)
	result = append(result, staticJSON[:closingBraceIdx]...)
	result = append(result, progressFields...)
	result = append(result, '}')

	return result
}

// buildDefaultProgressFields returns JSON fields for goals with no user progress.
func buildDefaultProgressFields() []byte {
	return defaultProgressFieldsLiteral
}

// buildProgressFields builds JSON fields for a goal with user progress.
// Delegates to writeProgressFields for the actual writing.
func buildProgressFields(progress *commonDomain.UserGoalProgress) []byte {
	buf := progressBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writeProgressFields(buf, progress)
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	progressBufPool.Put(buf)
	return result
}

// writeProgressFields writes JSON progress fields directly into the provided buffer.
// This is the zero-allocation core used by both buildProgressFields (single goal)
// and processGoalsArray (hot path).
func writeProgressFields(buf *bytes.Buffer, progress *commonDomain.UserGoalProgress) {
	buf.WriteString(`,"progress":`)
	buf.WriteString(strconv.FormatInt(int64(progress.Progress), 10))

	buf.WriteString(`,"status":"`)
	buf.WriteString(escapeJSONString(string(progress.Status)))
	buf.WriteByte('"')

	buf.WriteString(`,"completedAt":"`)
	if progress.CompletedAt != nil {
		buf.WriteString(progress.CompletedAt.Format(time.RFC3339))
	}
	buf.WriteByte('"')

	buf.WriteString(`,"claimedAt":"`)
	if progress.ClaimedAt != nil {
		buf.WriteString(progress.ClaimedAt.Format(time.RFC3339))
	}
	buf.WriteByte('"')

	buf.WriteString(`,"isActive":`)
	if progress.IsActive {
		buf.WriteString(`true`)
	} else {
		buf.WriteString(`false`)
	}

	buf.WriteString(`,"expiresAt":"`)
	if progress.ExpiresAt != nil {
		buf.WriteString(progress.ExpiresAt.Format(time.RFC3339))
	}
	buf.WriteByte('"')

	buf.WriteString(`,"expiresInSeconds":`)
	if progress.ExpiresAt != nil {
		seconds := max(int64(progress.ExpiresAt.Sub(time.Now().UTC()).Seconds()), 0)
		buf.WriteString(strconv.FormatInt(seconds, 10))
	} else {
		buf.WriteByte('0')
	}
}

// writeDefaultProgressFields writes default progress fields directly into the provided buffer.
func writeDefaultProgressFields(buf *bytes.Buffer) {
	buf.Write(defaultProgressFieldsLiteral)
}

// InjectProgressIntoChallenge injects user progress into multiple goals within a challenge JSON.
//
// Performance: ~500-800us for a challenge with 5 goals vs ~15ms for unmarshal+marshal (20-30x faster)
// With allocation reduction: 0 allocs per goal in the hot path (processGoalsArray).
func InjectProgressIntoChallenge(
	staticJSON []byte,
	userProgress map[string]*commonDomain.UserGoalProgress,
	goalCount int,
) ([]byte, error) {
	goalsIdx := bytes.Index(staticJSON, goalsFieldPattern)
	if goalsIdx == -1 {
		return staticJSON, nil
	}

	arrayStartIdx := bytes.IndexByte(staticJSON[goalsIdx:], '[')
	if arrayStartIdx == -1 {
		return nil, fmt.Errorf("invalid goals structure: missing opening bracket")
	}
	arrayStartIdx += goalsIdx

	arrayEndIdx := findMatchingClosingBracket(staticJSON, arrayStartIdx)
	if arrayEndIdx == -1 {
		return nil, fmt.Errorf("invalid goals structure: missing closing bracket")
	}

	estimatedGrowth := goalCount * 150
	result := bytes.NewBuffer(make([]byte, 0, len(staticJSON)+estimatedGrowth))

	result.Write(staticJSON[:arrayStartIdx+1])

	goalsArrayJSON := staticJSON[arrayStartIdx+1 : arrayEndIdx]
	if err := processGoalsArray(result, goalsArrayJSON, userProgress); err != nil {
		return nil, fmt.Errorf("failed to process goals array: %w", err)
	}

	result.Write(staticJSON[arrayEndIdx:])

	return result.Bytes(), nil
}

// processGoalsArray processes each goal in the goals array and injects progress
// directly into the result buffer. This is the hot path optimization: instead of
// calling InjectProgressIntoGoal (which allocates per goal), we write directly
// to the parent buffer and use pooled buffers for progress fields.
//
// Allocation reduction: 4 allocs/goal -> 0 allocs/goal
func processGoalsArray(
	result *bytes.Buffer,
	goalsArrayJSON []byte,
	userProgress map[string]*commonDomain.UserGoalProgress,
) error {
	goalStart := -1
	goalIndex := 0
	depth := 0
	inString := false
	escapeNext := false

	for i := range len(goalsArrayJSON) {
		c := goalsArrayJSON[i]

		if escapeNext {
			escapeNext = false
			continue
		}
		if c == '\\' {
			escapeNext = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch c {
		case '{':
			if depth == 0 {
				goalStart = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && goalStart != -1 {
				goalJSON := goalsArrayJSON[goalStart : i+1]

				// Extract goal ID as byte range (zero-alloc map lookup)
				idStart, idEnd, err := extractGoalIDRange(goalJSON)
				if err != nil {
					return fmt.Errorf("failed to extract goal_id from goal %d: %w", goalIndex, err)
				}

				// Zero-alloc map lookup: string([]byte) in map index is optimized by Go compiler
				progress := userProgress[string(goalJSON[idStart:idEnd])]

				// Write comma separator between goals
				if goalIndex > 0 {
					result.WriteByte(',')
				}

				// Write goal JSON up to closing brace directly to result buffer
				closingBraceIdx := bytes.LastIndexByte(goalJSON, '}')
				result.Write(goalJSON[:closingBraceIdx])

				// Write progress fields directly to result buffer (zero alloc)
				if progress == nil {
					writeDefaultProgressFields(result)
				} else {
					writeProgressFields(result, progress)
				}

				// Write closing brace
				result.WriteByte('}')

				goalIndex++
				goalStart = -1
			}
		}
	}

	return nil
}

// extractGoalID extracts the goalId value from a goal JSON object.
// Kept for backward compatibility - delegates to extractGoalIDRange.
func extractGoalID(goalJSON []byte) (string, error) {
	start, end, err := extractGoalIDRange(goalJSON)
	if err != nil {
		return "", err
	}
	return string(goalJSON[start:end]), nil
}

// extractGoalIDRange extracts the byte range [start, end) of the goalId value
// from a goal JSON object. Returns indices into goalJSON, not a string,
// so the caller can do a zero-alloc map lookup via string(goalJSON[start:end]).
func extractGoalIDRange(goalJSON []byte) (start, end int, err error) {
	goalIDIdx := bytes.Index(goalJSON, goalIDFieldPattern)
	if goalIDIdx == -1 {
		return 0, 0, fmt.Errorf("goalId field not found")
	}

	colonIdx := bytes.IndexByte(goalJSON[goalIDIdx:], ':')
	if colonIdx == -1 {
		return 0, 0, fmt.Errorf("invalid goalId field: missing colon")
	}
	colonIdx += goalIDIdx

	valueStartIdx := bytes.IndexByte(goalJSON[colonIdx:], '"')
	if valueStartIdx == -1 {
		return 0, 0, fmt.Errorf("invalid goalId field: missing opening quote")
	}
	valueStartIdx += colonIdx + 1

	valueEndIdx := bytes.IndexByte(goalJSON[valueStartIdx:], '"')
	if valueEndIdx == -1 {
		return 0, 0, fmt.Errorf("invalid goalId field: missing closing quote")
	}
	valueEndIdx += valueStartIdx

	return valueStartIdx, valueEndIdx, nil
}

// findMatchingClosingBracket finds the matching closing bracket ] for an opening bracket [.
func findMatchingClosingBracket(json []byte, openIdx int) int {
	depth := 1
	for i := openIdx + 1; i < len(json); i++ {
		switch json[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// escapeJSONString escapes special characters in a string for JSON encoding.
func escapeJSONString(s string) string {
	needsEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '"' || c == '\n' || c == '\r' || c == '\t' || c < 0x20 {
			needsEscape = true
			break
		}
	}

	if !needsEscape {
		return s
	}

	result := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			result = append(result, '\\', '\\')
		case '"':
			result = append(result, '\\', '"')
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		default:
			if c < 0x20 {
				result = fmt.Appendf(result, "\\u%04x", c)
			} else {
				result = append(result, c)
			}
		}
	}

	return string(result)
}
