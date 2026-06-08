package syncx

import (
	"context"
	"sync"
	"time"
)

// WaitWithContext waits for the WaitGroup until ctx is canceled.
// It returns true if the waiting timed out, and false otherwise.
func WaitWithContext(ctx context.Context, wg *sync.WaitGroup) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // Completed normally.
	case <-ctx.Done():
		return true // Timed out.
	}
}

// WaitWithTimeout waits for the WaitGroup for a specified duration.
// It returns true if the wait timed out, and false otherwise.
func WaitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // Completed normally.
	case <-time.After(timeout):
		return true // Timed out.
	}
}
