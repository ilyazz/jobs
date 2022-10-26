GO ?= go

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
