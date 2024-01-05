start-rate-limiting:
	docker compose -f ./cmd/rate_limiting/docker-compose.yml up --build --force-recreate -d

stop-rate-limiting:
	docker compose -f ./cmd/rate_limiting/docker-compose.yml down --remove-orphans --timeout 1 --volumes

start-load-shedding:
	docker compose -f ./cmd/load_shedding/docker-compose.yml up --build --force-recreate -d

stop-load-shedding:
	docker compose -f ./cmd/load_shedding/docker-compose.yml down --remove-orphans --timeout 1 --volumes

show-stats:
	docker stats

run-bombardier-1:
	bombardier -c 1 -n 10000 http://127.0.0.1:8080/

run-bombardier-10:
	bombardier -c 10 -n 10000 http://127.0.0.1:8080/

run-bombardier-100:
	bombardier -c 100 -n 10000 http://127.0.0.1:8080/

run-bombardier-1000:
	bombardier -c 1000 -n 10000 http://127.0.0.1:8080/

test:
	go test -v --race ./...
