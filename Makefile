.PHONY: up down logs test fmt seed-demo e2e-smoke prod-up prod-down prod-restart prod-logs prod-status

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

prod-up:
	docker compose --env-file .env.production -f deploy/docker-compose.prod.yml pull
	docker compose --env-file .env.production -f deploy/docker-compose.prod.yml up -d --no-build

prod-down:
	docker compose --env-file .env.production -f deploy/docker-compose.prod.yml down

prod-restart: prod-up

prod-logs:
	docker compose --env-file .env.production -f deploy/docker-compose.prod.yml logs -f

prod-status:
	docker compose --env-file .env.production -f deploy/docker-compose.prod.yml ps
