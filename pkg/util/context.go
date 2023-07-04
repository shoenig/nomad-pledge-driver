package util

import (
	"context"
	"time"
)

// Timeout will create a context.Context and context.CancelFunc that will
// expire after the given duration.
func Timeout(d time.Duration) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	ctx2, cancel := context.WithTimeout(ctx, d)
	return ctx2, cancel
}
