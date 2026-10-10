GO ?= go
export GOTOOLCHAIN := go1.27.1
.PHONY: build test verify verify-dsh images docs-check
build:
	$(GO) build ./...
	npm run build --prefix packages/environment-connector

test:
	$(GO) test -race ./...
	npm test --prefix packages/environment-connector

verify: build test
	$(GO) vet ./...
	node hack/check-standards.mjs
	$(MAKE) docs-check

docs-check:
	node hack/check-docs.mjs
	node --test hack/check-docs.test.mjs

verify-dsh:
	bash hack/verify-dsh-compat.sh

images:
	docker build --platform linux/amd64 -f images/environment/Dockerfile -t dsh-environment:test .
