default: dev

.PHONY: clean
	@echo "==> Cleanup previous build from ./output"
	@rm -f ./output/pledge

.PHONY: dev
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: clean
	@go build -o output/pledge
