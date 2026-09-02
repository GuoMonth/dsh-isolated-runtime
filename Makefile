GO ?= go

.PHONY: build fmt fmt-check generate lint test vet verify verify-cell verify-dsh verify-generated

build:
	$(GO) build ./...

fmt:
	gofmt -w .

fmt-check:
	test -z "$$(gofmt -l .)"

generate:
	$(GO) generate ./...

lint:
	golangci-lint run

test:
	$(GO) test -race -cover ./...

vet:
	$(GO) vet ./...

verify-generated:
	./hack/verify-generated.sh

verify: fmt-check verify-generated vet test build

verify-cell:
	./hack/verify-cell-contract.sh

verify-dsh:
	./hack/verify-dsh-compat.sh
