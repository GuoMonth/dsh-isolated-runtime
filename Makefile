GO ?= go
SETUP_ENVTEST_VERSION ?= v0.0.0-20260125163108-a19ec76a3c5d
ENVTEST_K8S_VERSION ?= 1.34.x

.PHONY: build fmt fmt-check generate images lint test test-envtest vet verify verify-phase1 verify-phase2 verify-phase3 verify-cell verify-dsh verify-generated verify-images verify-kind verify-kind-phase2 verify-kind-phase3

build:
	$(GO) build ./...

images:
	docker buildx build --platform linux/amd64 --load -f images/operator/Dockerfile -t dsh-operator:test .
	docker buildx build --platform linux/amd64 --load -f images/cell/Dockerfile -t dsh-cell:test .

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

test-envtest:
	KUBEBUILDER_ASSETS="$$($(GO) run sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION) use -p path $(ENVTEST_K8S_VERSION))" \
		$(GO) test -count=1 -run TestEnvtest ./internal/controller

vet:
	$(GO) vet ./...

verify-generated:
	./hack/verify-generated.sh

verify: fmt-check verify-generated vet test build

verify-phase1: verify test-envtest

verify-phase2: verify test-envtest

verify-phase3: verify test-envtest

verify-cell:
	./hack/verify-cell-contract.sh

verify-dsh:
	./hack/verify-dsh-compat.sh

verify-images:
	./hack/verify-images.sh

verify-kind:
	./hack/verify-phase1-kind.sh

verify-kind-phase2:
	./hack/verify-phase2-kind.sh

verify-kind-phase3:
	./hack/verify-phase3-kind.sh
