.PHONY: dev build run test test-backend test-frontend docker-build docker-up docker-down docker-logs deploy watch-deploy

# Run backend (:8080) and frontend dev (:3000) together.
dev:
	@echo "Starting backend on :8080 and frontend on :3000"
	@(cd backend && go run ./cmd/server) & \
	(cd frontend && npm run dev) ; \
	kill %1 2>/dev/null || true

# Build single binary: export frontend -> copy into embed dir -> go build.
build:
	cd frontend && npm install && npm run build
	rm -rf backend/internal/web/dist
	mkdir -p backend/internal/web/dist
	cp -r frontend/out/. backend/internal/web/dist/
	cd backend && go build -o bin/sing-box-admin ./cmd/server
	@echo "Built backend/bin/sing-box-admin"

run: build
	./backend/bin/sing-box-admin

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm test

# --- Docker (single container: embedded frontend + bundled sing-box) ---
docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# --- Deploy loop (rsync to VPS + remote docker rebuild) ---
deploy:
	./scripts/deploy.sh

watch-deploy:
	./scripts/watch-deploy.sh
