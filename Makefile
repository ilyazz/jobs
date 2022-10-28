GO ?= go
OUT ?= build
PROTO_GEN_DIR = ./pkg/api/grpc
CERTS ?= cert
CLIENT ?= $(shell whoami)
SNI ?= $(shell hostname) 127.0.0.1 localhost
CERTNAME ?= server

.PHONY: test
test:
	$(GO) test -race -v ./pkg/...

.PHONY: lint
lint:
	golangci-lint run

.PNONY: linter
linter:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.50.1

.PHONY: libtest
libtest: test
	$(GO) build -o $(OUT)/jtest ./cmd/test/main.go

.PHONY: proto
proto: buf
	@buf -v generate

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
