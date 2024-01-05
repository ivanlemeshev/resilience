package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type LoadShedder struct {
	isOverloaded atomic.Bool
}

func NewLoadShedder(ctx context.Context, checkInterval, overloadFactor time.Duration) *LoadShedder {
	ls := LoadShedder{}

	go ls.runOverloadDetector(ctx, checkInterval, overloadFactor)

	return &ls
}

func (ls *LoadShedder) runOverloadDetector(ctx context.Context, checkInterval, overloadFactor time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Start with a fresh start time.
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check how long it took to process the last batch of requests.
			elapsed := time.Since(startTime)
			if elapsed > overloadFactor {
				// If it took longer than the overload factor, we're overloaded.
				ls.isOverloaded.Store(true)
			} else {
				// Otherwise, we're not overloaded.
				ls.isOverloaded.Store(false)
			}
			// Reset the start time.
			startTime = time.Now()
		}
	}
}

func (ls *LoadShedder) IsOverloaded() bool {
	return ls.isOverloaded.Load()
}

type Handler struct {
	ls *LoadShedder
}

func NewHandler(ls *LoadShedder) *Handler {
	return &Handler{ls: ls}
}

func (h *Handler) Handler(w http.ResponseWriter, r *http.Request) {
	if h.ls.IsOverloaded() {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, http.StatusText(http.StatusServiceUnavailable))
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, http.StatusText(http.StatusOK))
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The load shedder will check every 100ms if the last batch of requests
	// took longer than 200ms.
	ls := NewLoadShedder(ctx, 100*time.Millisecond, 200*time.Millisecond)

	h := NewHandler(ls)
	http.HandleFunc("/", h.Handler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
