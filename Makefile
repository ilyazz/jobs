GO ?= CGO_ENABLED=0 go
OUT ?= build

PROTO_GEN_DIR = ./pkg/api/grpc
CERTS ?= cert

CLIENT ?= $(shell whoami)

SNI ?= $(shell hostname) 127.0.0.1 localhost

CERTNAME ?= server

.PHONY: all
all: clean client server integration

.PHONY: test
test:
	$(GO) test -v ./pkg/...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: linter
linter:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.50.1

.PHONY: libtest
libtest: test
	$(GO) build -o $(OUT)/jtest ./cmd/test/main.go

.PHONY: proto
proto:
	@buf generate

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

.PHONY: buf
buf:
	@buf --version >/dev/null 2>/dev/null || (echo "please install buf (https://buf.build/)"; exit 1)

.PHONY: clean
clean: proto_clean
	rm -rf $(OUT)
	

.PHONY: proto_clean
proto_clean:
	rm -rf $(PROTO_GEN_DIR)

.PHONY: mkcert
mkcert:
	@mkcert -version || (echo "please install mkcert. (https://github.com/FiloSottile/mkcert#installation) "; exit 1)

.PHONY: client-cert
client-cert: mkcert certdir
	CAROOT=$(CERTS)/CA mkcert -cert-file $(CERTS)/client/$(CLIENT)-cert.pem -key-file $(CERTS)/client/$(CLIENT)-key.pem -ecdsa -client $(CLIENT) 

.PHONY: server-cert
server-cert: mkcert certdir
	CAROOT=$(CERTS)/CA mkcert -cert-file $(CERTS)/server/$(CERTNAME)-cert.pem -key-file $(CERTS)/server/$(CERTNAME)-key.pem -ecdsa $(SNI)

.PHONY: certdir
certdir:
	mkdir -p $(CERTS)/server
	mkdir -p $(CERTS)/client
