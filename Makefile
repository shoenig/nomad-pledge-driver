GOTAGS ?= osusergo

default: dev

.PHONY: clean
clean:
	@echo "==> Cleanup previous build"
	rm -rf /tmp/plugins dist
	mkdir /tmp/plugins

.PHONY: dev
dev: clean
	@echo "==> Compile pledge plugin"
	go build -race -tags=$(GOTAGS) -o /tmp/plugins/nomad-pledge-driver

.PHONY: test
test:
	@echo "==> Test pledge plugin"
	go test -race -v ./...

.PHONY: vet
vet:
	@echo "==> Vet pledge packages"
	go vet ./...

.PHONY: run
run: dev
run:
	@echo "==> Run in dev mode"
	sudo nomad agent -dev -config=hack/client.hcl

.PHONY: release
release: clean
	@echo "==> Create release"
	envy exec gh-release goreleaser release --clean
	$(MAKE) clean

