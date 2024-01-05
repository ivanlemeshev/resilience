package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	remainingTokens int
	lastRefillTime  time.Time
}

type RateLimiter struct {
	maxTokens      int
	refillInterval time.Duration
	buckets        map[string]*bucket
	mu             sync.Mutex
}

func NewRateLimiter(rate int, perInterval time.Duration) *RateLimiter {
	return &RateLimiter{
		maxTokens:      rate,
		refillInterval: perInterval,
		buckets:        make(map[string]*bucket),
	}
}

func (rl *RateLimiter) IsLimitReached(id string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[id]

	// If the bucket doesn't exist, it is the first request for this client.
	// Create a new bucket and allow the request.
	if !ok {
		rl.buckets[id] = &bucket{
			remainingTokens: rl.maxTokens - 1,
			lastRefillTime:  time.Now(),
		}
		return false
	}

	// Calculate the number of tokens to add to the bucket since the last
	// request.
	refillInterval := int(time.Since(b.lastRefillTime) / rl.refillInterval)
	tokensAdded := rl.maxTokens * refillInterval
	currentTokens := b.remainingTokens + tokensAdded

	// There is no tokens to serve the request for this client.
	// Reject the request.
	if currentTokens < 1 {
		return true
	}

	if currentTokens > rl.maxTokens {
		// If the number of current tokens is greater than the maximum allowed,
		// then reset the bucket and decrease the number of tokens by 1.
		b.lastRefillTime = time.Now()
		b.remainingTokens = rl.maxTokens - 1
	} else {
		// Otherwise, update the bucket and decrease the number of tokens by 1.
		deltaTokens := currentTokens - b.remainingTokens
		deltaRefills := deltaTokens / rl.maxTokens
		deltaTime := time.Duration(deltaRefills) * rl.refillInterval
		b.lastRefillTime = b.lastRefillTime.Add(deltaTime)
		b.remainingTokens = currentTokens - 1
	}

	// Allow the request.
	return false
}

type Handler struct {
	rl *RateLimiter
}

func NewHandler(rl *RateLimiter) *Handler {
	return &Handler{rl: rl}
}

func (h *Handler) Handler(w http.ResponseWriter, r *http.Request) {
	// Here should be the logic to get the client ID from the request 
	// (it could be a user ID, an IP address, an API key, etc.)
	clientID := "some-client-id"
	if h.rl.IsLimitReached(clientID) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, http.StatusText(http.StatusTooManyRequests))
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, http.StatusText(http.StatusOK))
}

func main() {
	// We allow 1000 requests per second per client to our service.
	rl := NewRateLimiter(1000, 1*time.Second)
	h := NewHandler(rl)
	http.HandleFunc("/", h.Handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
