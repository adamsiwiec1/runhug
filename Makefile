.PHONY: help build test tidy docs docs-build check build-index-packs

help:
	@echo "build              Go binary → bin/runhug-cli"
	@echo "test               go test ./..."
	@echo "tidy               go mod tidy"
	@echo "build-index-packs  Category SQLite packs → dist/index (needs HF_TOKEN)"
	@echo "docs               VitePress dev server"
	@echo "docs-build         Production docs build (CI / Pages)"
	@echo "check              tests + vet + docs-build"

build:
	go build -o bin/runhug-cli ./cmd/runhug-cli

test:
	go test ./...

tidy:
	go mod tidy

docs:
	npm run docs:dev

docs-build:
	npm run docs:build

check: test
	go vet ./...
	$(MAKE) build
	$(MAKE) docs-build

build-index-packs:
	mkdir -p dist/index
	go run ./cmd/build-index-packs --out dist/index
