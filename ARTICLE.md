# Prevent server overload in Go

## Introduction

In developing large-scale systems, it is always essential to think about resilience. The load on an application typically varies over time based on the number of active users or the types of activities they are performing.

One of the most common situations you will encounter sooner or later is server overload caused by unanticipated bursts in traffic. It happens because every server has functional limitations. That means the server has limited network and computational resources like CPU and memory, so the server can effectively handle only a certain number of requests. When the number of requests reaches some threshold and continues growing, it increases resource utilization, leading to higher latency and reducing availability.

There are many strategies to handle that situation. For instance, you can run several instances of your application and use autoscaling to match the provisioned resources to the users's needs at any time. However, when the amount of traffic is incredibly huge, scaling out may take time, and servers may be overloaded or fail. I will discuss common strategies to prevent such a situation in this article.

## Causes leading to traffic spikes

There may be many reasons leading to the sudden traffic spikes. Let's consider some of the common.

An application can experience sudden spikes in traffic due to events like breaking news, product launches, or promotional campaigns. If the servers are not prepared to handle this surge of traffic, they can quickly become overloaded.

Also, when the traffic to the application is dropped temporarily for some reason, the application scales down. Then, when the traffic returns, the application can't scale out fast, leading to server overload and even failure.

Another common reason is that malicious bots, or malware, can flood a server with requests, effectively overwhelming its resources. It is known as a distributed denial-of-service (DDoS) attack.

## Strategies to prevent service overload

There are two main strategies to prevent server overload:

- Throttling,
- Load shedding.

Throttling controls the consumption of resources used by an instance of an application, an individual user, or an entire service. For example, a server experiencing high demand may start throttling requests, slowing them down, or rejecting them until the load subsides. It allows the system to handle the incoming traffic without crashing or becoming unresponsive. 

One common technique for throttling is rate limiting, which defines the maximum number of requests a user, a client, or an entire group can make within a specific time frame, ensuring no single client or user can monopolize system resources. It is a proactive measure that aims to prevent the system from reaching its capacity in the first place. For instance, a rate limiter may allow 100 requests from a given IP address per minute. Exceeding this limit could result in the user being temporarily blocked or having their requests queued until they are within the allowable rate.

Load shedding is a more drastic technique. It involves intentionally dropping requests to prevent a server from becoming overloaded. It is only done as a last resort when other measures, such as rate limiting, have proven insufficient. By shedding requests from one application instance, the load can be redistributed to other application instances in the cluster, preventing any single instance from reaching its limits.

## An example of rate limiting

Several algorithms can be used to implement rate limiters. Here, let's consider a basic implementation of a rate limiter using a token bucket algorithm, where tokens are added to the bucket at a fixed rate. The request is allowed if a token is available when a request is made. Otherwise, it is denied. This implementation can be applied to throttling requests from particular users, IP addresses, API keys, or other groups of users.

`main.go`

```go
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
```

Let's run our example using Docker Compose with configured [resource constraints](https://docs.docker.com/compose/compose-file/deploy/#resources) for a container. Also, we will use [bombardier](https://github.com/codesenberg/bombardier), a benchmarking tool, to check how our application works on different loads. We will use the following `Dockerfile` and `docker-compose.yml`:

`Dockerfile`

```dockerfile
FROM golang:1.21 AS build-stage
WORKDIR /code
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /service main.go
FROM gcr.io/distroless/base AS build-release-stage
WORKDIR /
COPY --from=build-stage /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
```

`docker-compose.yml`

```yml
services:
  rate_limiting:
    build: .
    ports:
      - "8080:8080"
    deploy:
      resources:
        limits:
          cpus: '.20'
          memory: 100M
        reservations:
          cpus: '0.10'
          memory: 50M

```

Let's run our application using the following command:

```bash
docker compose -f ./cmd/rate_limiting/docker-compose.yml up --build --force-recreate -d
```

You can monitor the container resource usage using the following command:

```bash
dcocker stats
```

Let's run the bombardier with different settings and check the results in a separate terminal window.

First, let's try sending 10000 requests with one concurrent connection.

```bash
$ bombardier -c 1 -n 10000 http://127.0.0.1:8080/
Bombarding http://127.0.0.1:8080/ with 10000 request(s) using 1 connection(s)
 10000 / 10000 [===============================================================] 100.00% 859/s 11s
Done!
Statistics        Avg      Stdev        Max
  Reqs/sec       868.33     833.53    2684.45
  Latency        1.15ms     6.93ms    75.42ms
  HTTP codes:
    1xx - 0, 2xx - 10000, 3xx - 0, 4xx - 0, 5xx - 0
    others - 0
  Throughput:   151.93KB/s
```

From the results, you can see that the request rate on average is less than 1000 requests per second, which is the maximum allowed rate, and all requests were processed successfully. Next,  let's try sending 10000 requests with 100 concurrent connections.


```bash
$ bombardier -c 100 -n 10000 http://127.0.0.1:8080/
Bombarding http://127.0.0.1:8080/ with 10000 request(s) using 100 connection(s)
 10000 / 10000 [===============================================================] 100.00% 3320/s 3s
Done!
Statistics        Avg      Stdev        Max
  Reqs/sec      3395.87    6984.32   32322.59
  Latency       28.02ms    37.61ms   196.95ms
  HTTP codes:
    1xx - 0, 2xx - 3000, 3xx - 0, 4xx - 7000, 5xx - 0
    others - 0
  Throughput:   675.35KB/s
```

You can see that the server started to return HTTP 429 status code, which means the server is overloaded due to many requests.


Clean up your environment using the following command after all tests are done:


```bash
docker compose -f ./cmd/rate_limiting/docker-compose.yml down --remove-orphans --timeout 1 --volumes
```

## An example of load shedding

Depending on your particular case, you can use different approaches to load shedding: 

- Dropping random requests when the system becomes overloaded,
- Using a load balancer to distribute the load evenly,
- Monitoring server load continuously and dropping or rejecting requests when a certain load level is reached, 
- Monitoring system metrics like CPU utilization, network I/O, or memory usage and dropping requests when these metrics reach certain thresholds,
- Set up priority levels for requests and start refusing low-priority requests when the system is under load,
- Etc.

Load shedding can be implemented at various levels, including load balancers, servers, and software libraries. It requires careful planning and implementation, as poorly implemented load shedding can result in important requests being dropped, causing a negative impact on user experience or system functionality.

Let's implement a simple load-shedding mechanism by detecting request overload conditions and responding with an HTTP 503 status code.

`main .go`

```go
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
```

We define the `LoadShedder` struct to run the load detector used in our request handler to check if we should return the HTTP 503 status code or process the request. The `LoadShedder` struct has an atomic flag indicating if the system is overloaded. We use atomic boolean value to make it thread-safe. We also have the `IsOverloaded` method that returns the current value of the flag. In the `NewLoadShedder` function, we create a new `LoadShedder` and run the overload detector in a goroutine that checks according to the specified interval if the system is overloaded based on the overload factor. We check how much time has passed since the last check. If it is higher than the overload factor, the system is overloaded. That means that our request handler used resources for too long, and we need more capacity to handle the requests.

Let's run our example using. We will use the following `Dockerfile` and `docker-compose.yml`:

`Dockerfile`

```dockerfile
FROM golang:1.21 AS build-stage
WORKDIR /code
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /service main.go
FROM gcr.io/distroless/base AS build-release-stage
WORKDIR /
COPY --from=build-stage /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
```

`docker-compose.yml`

```yml
services:
  load_shedding:
    build: .
    ports:
      - "8080:8080"
    deploy:
      resources:
        limits:
          cpus: '.20'
          memory: 100M
        reservations:
          cpus: '0.10'
          memory: 50M

```

Let's run our application using the following command:

```bash
docker compose -f ./cmd/load_shedding/docker-compose.yml up --build --force-recreate -d
```

You can monitor the container resource usage using the following command:

```bash
dcocker stats
```

Let's run the bombardier with different settings and check the results in a separate terminal window.

First, let's try sending 10000 requests with 10 concurrent connections.

```bash
$ bombardier -c 10 -n 10000 http://127.0.0.1:8080/
Bombarding http://127.0.0.1:8080/ with 10000 request(s) using 10 connection(s)
 10000 / 10000 [===============================================================] 100.00% 1346/s 7s
Done!
Statistics        Avg      Stdev        Max
  Reqs/sec      1389.49    1582.00    6284.42
  Latency        7.24ms    22.41ms    98.43ms
  HTTP codes:
    1xx - 0, 2xx - 10000, 3xx - 0, 4xx - 0, 5xx - 0
    others - 0
  Throughput:   242.78KB/s
```

The results show that the maximum latency is less than 100 ms, and all requests were processed successfully. Next, let's try sending 10000 requests with 1000 concurrent connections.


```bash
$ bombardier -c 1000 -n 10000 http://127.0.0.1:8080/
Bombarding http://127.0.0.1:8080/ with 10000 request(s) using 1000 connection(s)
 10000 / 10000 [===============================================================] 100.00% 3791/s 2s
Done!
Statistics        Avg      Stdev        Max
  Reqs/sec      4242.28   11985.30   58592.64
  Latency      211.67ms   210.89ms      1.54s
  HTTP codes:
    1xx - 0, 2xx - 8823, 3xx - 0, 4xx - 0, 5xx - 1177
    others - 0
  Throughput:   696.96KB/s
```

You can see that the server started to return HTTP 503 status code, which means the server is overloaded due to many requests, and you can see the maximum latency is more than 1 second.

Clean up your environment using the following command after all tests are done:


```bash
docker compose -f ./cmd/load_shedding/docker-compose.yml down --remove-orphans --timeout 1 --volumes
```

## Conclusion

In this article, we discussed the causes leading to traffic spikes and strategies to prevent server overload. We also implemented a simple rate limiter and load shedder to demonstrate how they work. You can find all the code from the article here [https://github.com/ivanlemeshev/resilience](https://github.com/ivanlemeshev/resilience).

For the production environment, you can use ready-to-use rate limiters like https://pkg.go.dev/golang.org/x/time/rate or https://github.com/uber-go/ratelimit.
