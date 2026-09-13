.PHONY: help build test tidy docs docs-build check

help:
	@echo "build        Go binary → bin/runpod-vllm-proxy"
	@echo "test         go test ./..."
	@echo "tidy         go mod tidy"
	@echo "docs         VitePress dev server"
	@echo "docs-build   Production docs build (CI / Pages)"
	@echo "check        tests + vet + docs-build"

build:
	go build -o bin/runpod-vllm-proxy ./cmd/runpod-vllm-proxy

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
