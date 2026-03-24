package queue

import (
	"context"
	"time"
)

// StartTTLWorker starts a background goroutine that periodically removes expired messages.
func StartTTLWorker(ctx context.Context, store *Store, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				store.ExpireMessages()
			}
		}
	}()
}
