CERTS ?= cert
CLIENT ?= $(shell whoami)
SNI ?= $(shell hostname) 127.0.0.1 localhost
CERTNAME ?= server

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
