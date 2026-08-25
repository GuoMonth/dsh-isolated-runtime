FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/GuoMonth/dsh-isolated-runtime/internal/version.Version=${VERSION} -X github.com/GuoMonth/dsh-isolated-runtime/internal/version.Commit=${COMMIT} -X github.com/GuoMonth/dsh-isolated-runtime/internal/version.Date=${DATE}" \
    -o /out/dsh-isolated-control-plane ./cmd/controller

FROM scratch
COPY --from=build /out/dsh-isolated-control-plane /dsh-isolated-control-plane
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/dsh-isolated-control-plane"]
