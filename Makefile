all: check test
export GOPROXY := proxy.golang.org,go-proxy.oss.wandera.net,direct
export GONOSUMDB := github.com/wandera/*,github.com/jamf/*

prepare:
ifeq (, $(shell which tparse))
	@echo "tparse is missing on your system. Install it first: go install github.com/mfridman/tparse@latest"; exit 1
endif
ifeq (, $(shell which golangci-lint))
	@echo "golangci-lint is missing on your system. Install it first: brew install golangci-lint"; exit 1
endif

check: prepare
	@echo "Running check"
	golangci-lint run
	go mod tidy

test:
	@echo "Running tests"
	go test -race -json -cover -v ./... | tparse -all

clean:
	@echo "Running clean"
	rm -rf "report/"

update: prepare
	@echo "Updating go dependencies and tools"
	go get -u ./...
	go mod tidy

.PHONY: all prepare check test update
