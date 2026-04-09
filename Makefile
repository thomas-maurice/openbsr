.PHONY: frontend build test clean

# Build the Vue SPA (outputs to internal/web/dist/)
frontend:
	cd web && npm ci && npm run build

# Build the Go binary (builds frontend first)
build: frontend
	CGO_ENABLED=1 go build -o bsr ./cmd/bsr

test:
	CGO_ENABLED=1 go test ./...

clean:
	rm -f bsr
	rm -rf internal/web/dist
