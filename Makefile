.PHONY: build build-admin dev test clean

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

# Run all tests
test:
	go test -race ./...

# Clean build artifacts
clean:
	rm -rf bin/ web/admin/dist web/admin/node_modules
