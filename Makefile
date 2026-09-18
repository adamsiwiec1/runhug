.PHONY: help build test test-heretic tidy check build-index-packs

help:
	@echo "build              Go binary → bin/runhug"
	@echo "test               go test ./..."
	@echo "test-heretic       unit tests for the heretic pod container scripts"
	@echo "tidy               go mod tidy"
	@echo "build-index-packs  Category SQLite packs → dist/index (needs HF_TOKEN)"
	@echo "check              tests + vet + build"

build:
	go build -o bin/runhug ./cmd/runhug

test:
	go test ./...

test-heretic:
	go test ./internal/cli/ -run 'Heretic|Dashboard|PoolGPUName|DateTimeFormat' -count=1
	python3 -m unittest discover -s containers/heretic/tests -p 'test_run_pipeline.py'

tidy:
	go mod tidy

check: test
	go vet ./...
	$(MAKE) build

build-index-packs:
	mkdir -p dist/index
	go run ./cmd/build-index-packs --out dist/index
