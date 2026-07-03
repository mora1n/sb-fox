BINARY := sb-fox
PKG := ./cmd/sb-fox
FRONTEND_DIR := frontend
DIST_DIR := internal/assets/dist

.PHONY: all build frontend test parity run cross clean tidy dev

all: build

frontend:
	cd $(FRONTEND_DIR) && npm ci && npm run build

# build depends on the embedded dist existing; frontend target produces it.
build:
	@if [ ! -f $(DIST_DIR)/index.html ]; then \
		echo "warning: $(DIST_DIR)/index.html missing — run 'make frontend' first for a full binary"; \
	fi
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BINARY) $(PKG)

test:
	go test ./...

# Cross-language parity: run the JS oracle to (re)generate goldens, then Go tests assert against them.
parity:
	go test ./internal/merge/ -run TestRegression -v

run: build
	./$(BINARY) --addr 127.0.0.1:7878 --data-dir ./data

dev:
	go run $(PKG) --addr 127.0.0.1:7878 --data-dir ./data --dev

cross:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64   $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/$(BINARY)-linux-arm64   $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/$(BINARY)-darwin-arm64  $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/$(BINARY)-windows-amd64.exe $(PKG)

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
	rm -rf dist $(DIST_DIR)
