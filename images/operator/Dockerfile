FROM --platform=$BUILDPLATFORM golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY cmd/cell-authorizer ./cmd/cell-authorizer
COPY cmd/cell-operator ./cmd/cell-operator
COPY internal/accesscontract ./internal/accesscontract
COPY internal/authorizer ./internal/authorizer
COPY internal/cellcontract ./internal/cellcontract
COPY internal/controller ./internal/controller
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w -buildid=' -o /out/cell-operator ./cmd/cell-operator && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w -buildid=' -o /out/cell-authorizer ./cmd/cell-authorizer

FROM scratch
ARG SOURCE_REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/GuoMonth/dsh-isolated-runtime" \
      org.opencontainers.image.revision=$SOURCE_REVISION
COPY --from=builder /out/cell-operator /cell-operator
COPY --from=builder /out/cell-authorizer /cell-authorizer
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 65532:65532
EXPOSE 8080 8081 9001
ENTRYPOINT ["/cell-operator"]
