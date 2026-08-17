GO ?= go

.PHONY: build test vet fmt verify clean

build:
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

verify: fmt vet test build

clean:
	rm -rf bin/
