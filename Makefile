export GOCACHE ?= $(CURDIR)/.cache/go-build
export GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
export GOMODCACHE ?= $(CURDIR)/.gomodcache

ifeq ($(OS),Windows_NT)
MKDIR_BIN = powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path 'bin' | Out-Null"
else
MKDIR_BIN = mkdir -p bin
endif

build:
	$(MKDIR_BIN)
	go build -buildvcs=false -o bin/app .

run:
	go run main.go

tidy:
	go mod tidy

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
