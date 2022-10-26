.PHONY: all
all:
	@echo "coming soon..."

.PHONY: proto
proto: buf
	buf generate

.PHONY: buf
buf:
	@buf --version >/dev/null 2>/dev/null || (echo "please install buf (https://buf.build/)"; exit 1)


