default: dev

GOTAGS ?= osusergo

.PHONY: clean
	@echo "==> Cleanup previous build from ./output"
	@rm -f ./output/pledge

.PHONY: dev
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: clean
	@echo "==> Compile pledge plugin"
	@go build -tags=$(GOTAGS) -o output/pledge
