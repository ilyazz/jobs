GO ?= go
OUT ?= build

.PHONY: all
all:
	@echo "coming soon..."

.PHONY: test
test:
	$(GO) test -v ./pkg/...

.PHONY: lint
lint:
	golangci-lint run

.PNONY: linter
linter:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.50.1

.PHONY: libtest
libtest: test
	$(GO) build -o $(OUT)/jtest ./cmd/test/main.go

.PHONY: test
test:
	$(GO) test -v ./pkg/...
