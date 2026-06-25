package registry

import (
	"context"
	"time"
)

// contextWithOneMinute returns a context that expires after 1 minute.
func contextWithOneMinute(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, time.Minute)
}
