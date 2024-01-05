package main

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiting(t *testing.T) {
	testUserID1 := "test-user-id-1"
	testUserID2 := "test-user-id-2"

	t.Run("request rate is lower than the capacity", func(t *testing.T) {
		// We allow 10 requests per 100 milliseconds per user.
		rl := NewRateLimiter(10, 100*time.Millisecond)
		for i := 0; i < 100; i++ {
			assert.False(t, rl.IsLimitReached(testUserID1))
			time.Sleep(20 * time.Millisecond)
		}
	})

	t.Run("request rate is equal to the capacity", func(t *testing.T) {
		// We allow 10 requests per 100 milliseconds per user.
		rl := NewRateLimiter(10, 100*time.Millisecond)
		for i := 0; i < 100; i++ {
			assert.False(t, rl.IsLimitReached(testUserID1))
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("request rate is higher than the capacity", func(t *testing.T) {
		// We allow 10 requests per 100 milliseconds per user.
		rl := NewRateLimiter(10, 100*time.Millisecond)
		for i := 0; i < 10; i++ {
			assert.False(t, rl.IsLimitReached(testUserID1))
			time.Sleep(5 * time.Millisecond)
		}

		for i := 0; i < 10; i++ {
			assert.True(t, rl.IsLimitReached(testUserID1))
			time.Sleep(5 * time.Millisecond)
		}

		for i := 0; i < 10; i++ {
			assert.False(t, rl.IsLimitReached(testUserID1))
			time.Sleep(5 * time.Millisecond)
		}
	})

	t.Run("requests from several users", func(t *testing.T) {
		// We allow 10 requests per 100 milliseconds per user.
		rl := NewRateLimiter(10, 100*time.Millisecond)

		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i < 10; i++ {
				assert.False(t, rl.IsLimitReached(testUserID1))
				time.Sleep(5 * time.Millisecond)
			}

			for i := 0; i < 10; i++ {
				assert.True(t, rl.IsLimitReached(testUserID1))
				time.Sleep(5 * time.Millisecond)
			}

			for i := 0; i < 10; i++ {
				assert.False(t, rl.IsLimitReached(testUserID1))
				time.Sleep(5 * time.Millisecond)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				assert.False(t, rl.IsLimitReached(testUserID2))
				time.Sleep(10 * time.Millisecond)
			}
		}()

		wg.Wait()
	})
}
