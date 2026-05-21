.PHONY: up down logs test fmt seed-demo e2e-smoke

up:
	docker compose -f deploy/docker-compose.yml up --build

down:
	docker compose -f deploy/docker-compose.yml down

logs:
	docker compose -f deploy/docker-compose.yml logs -f

test:
	cd packages/go-shared && go test ./...
	cd apps/api-service && go test ./...
	cd apps/traffic-service && go test ./...
	cd apps/postback-service && go test ./...

fmt:
	cd packages/go-shared && gofmt -w .
	cd apps/api-service && gofmt -w .
	cd apps/traffic-service && gofmt -w .
	cd apps/postback-service && gofmt -w .

seed-demo:
	./scripts/seed-demo.sh

e2e-smoke:
	./scripts/e2e-smoke.sh
