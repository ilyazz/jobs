
PROTO_GEN_DIR = ./pkg/api/grpc

.PHONY: proto
proto: buf
	@buf -v generate

.PHONY: buf
buf:
	@buf --version >/dev/null 2>/dev/null || (echo "please install buf (https://buf.build/)"; exit 1)


.PHONY: clean
clean: proto_clean
	

.PHONY: proto_clean
proto_clean:
	rm -rf $(PROTO_GEN_DIR)

