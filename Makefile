GO ?= go
export GOTOOLCHAIN := go$(shell sed -n 's/^go //p' go.mod)
.PHONY: build test lint fmt verify verify-dsh security images docs-check
build:
	$(GO) build ./...
	pnpm build

test:
	$(GO) test -race ./...
	pnpm test
	pnpm test:tooling

lint:
	node hack/check-standards.mjs
	pnpm fmt:check
	pnpm lint

fmt:
	pnpm fmt

verify: lint build test
	$(GO) vet ./...
	$(MAKE) docs-check
	$(MAKE) security

security:
	$(GO) mod verify
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -test ./...
	pnpm security
	npm audit --prefix images/environment --omit=dev --audit-level=low

docs-check:
	node hack/check-docs.mjs
	node --test hack/check-docs.test.mjs

verify-dsh:
	bash hack/verify-dsh-compat.sh

images:
	docker build --platform linux/amd64 -f images/environment/Dockerfile -t dsh-environment:test .
