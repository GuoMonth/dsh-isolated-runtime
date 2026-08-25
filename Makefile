GO ?= go
IMAGE ?= ghcr.io/guomonth/dsh-isolated-runtime:dev

.PHONY: build test vet fmt verify image deploy undeploy clean

build:
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

verify: fmt vet test build

image:
	docker build -t $(IMAGE) .

deploy:
	kubectl apply -k config

undeploy:
	kubectl delete -k config

clean:
	rm -rf bin/
