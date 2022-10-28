GO ?= CGO_ENABLED=0 go
OUT ?= build

.PHONY: all
all: client server

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

.PHONY: clean
clean:
	rm -rf $(OUT)

.PHONY: proto
proto:
	buf generate

$(OUT):
	mkdir $@

.PHONY: client
client: $(OUT) proto
	$(GO) build -o $(OUT)/jctrl ./cmd/client/main.go

.PHONY: server
server: $(OUT) proto
	$(GO) build -o $(OUT)/jserver ./cmd/server/main.go


.PHONY: integration
integration: server client
	go test -v ./integration/integration_test.go --server ../$(OUT)/jserver --client ../$(OUT)/jctrl --uid $(shell id -u) --gid $(shell id -g)