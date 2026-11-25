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
	node hack/check-standards.ts
	pnpm fmt:check
	pnpm lint
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run

fmt:
	pnpm fmt

verify: lint build test
	pnpm typecheck
	$(GO) vet ./...
	$(MAKE) docs-check
	$(MAKE) security

security:
	$(GO) mod verify
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -test ./...
	pnpm security
	node hack/audit-node.ts --workload

docs-check:
	node hack/check-docs.mjs
	node --test hack/check-docs.test.mjs

verify-dsh:
	bash hack/verify-dsh-compat.sh

images:
	docker build --platform linux/amd64 -f images/environment/Dockerfile -t dsh-environment:test .
