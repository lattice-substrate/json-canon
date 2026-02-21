.PHONY: test test-blackbox lint build check baseline

test:
	go test ./... -count=1 -v

test-blackbox:
	go test ./cmd/jcs-canon -run 'TestCLI' -count=1 -v

lint:
	golangci-lint run --timeout=5m ./...

build:
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o jcs-canon ./cmd/jcs-canon

check: lint test build

baseline:
	sha256sum golangci.yml golangci.base.yml > .github/lint-config-baseline.sha256
