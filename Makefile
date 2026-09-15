.PHONY: help build test tidy check build-index-packs

help:
	@echo "build              Go binary → bin/runhug"
	@echo "test               go test ./..."
	@echo "tidy               go mod tidy"
	@echo "build-index-packs  Category SQLite packs → dist/index (needs HF_TOKEN)"
	@echo "check              tests + vet + build"

build:
	go build -o bin/runhug ./cmd/runhug

test:
	go test ./...

tidy:
	go mod tidy

check: test
	go vet ./...
	$(MAKE) build

build-index-packs:
	mkdir -p dist/index
	go run ./cmd/build-index-packs --out dist/index
