default: dev

GOTAGS ?= osusergo

.PHONY: clean
	@echo "==> Cleanup previous build from ./output"
	@rm -f ./output/pledge

.PHONY: dev
dev: clean
	@echo "==> Compile pledge plugin"
	@go build -race -tags=$(GOTAGS) -o output/pledge

.PHONY: test
test:
	@echo "==> Test pledge plugin"
	@go test -race -v ./...

.PHONY: vet
vet:
	@echo "==> Vet pledge packages"
	@go vet ./...
