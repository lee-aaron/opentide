.PHONY: build build-admin dev test clean docker docker-up docker-down

# Build the React admin UI
build-admin:
	cd web/admin && npm ci && npm run build
	rm -rf internal/admin/dist
	cp -r web/admin/dist internal/admin/dist

# Build the Go binary (includes embedded admin UI)
build: build-admin
	go build -o bin/opentide ./cmd/opentide
	go build -o bin/tide-cli ./cmd/tide-cli

# Run in dev mode (frontend dev server + Go backend)
dev:
	@echo "Start frontend: cd web/admin && npm run dev"
	@echo "Start backend:  go run ./cmd/opentide --demo"

# Run all tests (Go + frontend)
test:
	go test -race ./...
	cd web/admin && npm run test

# Build Docker image
docker:
	docker build -f deploy/docker/Dockerfile -t opentide:latest .

# Start with docker compose (requires .env with secrets)
docker-up:
	docker compose up --build -d

# Stop docker compose
docker-down:
	docker compose down

# Clean build artifacts
clean:
	rm -rf bin/ web/admin/dist web/admin/node_modules
