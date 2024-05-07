package internal

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9/internal/util"
)

func Sleep(ctx context.Context, dur time.Duration) error {
	t := time.NewTimer(dur)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ToLower(s string) string {
	if isLower(s) {
		return s
	}

	b := make([]byte, len(s))
	for i := range b {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return util.BytesToString(b)
}

func isLower(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			return false
		}
	}
	return true
}

func ReplaceSpaces(s string) string {
	// Pre-allocate a builder with the same length as s to minimize allocations.
	// This is a basic optimization; adjust the initial size based on your use case.
	var builder strings.Builder
	builder.Grow(len(s))

	for _, char := range s {
		if char == ' ' {
			// Replace space with a hyphen.
			builder.WriteRune('-')
		} else {
			// Copy the character as-is.
			builder.WriteRune(char)
		}
	}

	return builder.String()
}
