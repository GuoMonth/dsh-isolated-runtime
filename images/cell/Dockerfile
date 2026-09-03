FROM --platform=$BUILDPLATFORM golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac AS launcher
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY cmd/cell-launcher ./cmd/cell-launcher
COPY internal/cellcontract ./internal/cellcontract
COPY internal/dshcompat ./internal/dshcompat
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w -buildid=' -o /out/cell-launcher ./cmd/cell-launcher

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS dsh
WORKDIR /opt/dsh
COPY images/cell/package.json images/cell/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts && npm cache clean --force

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e
ARG SOURCE_REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/GuoMonth/dsh-isolated-runtime" \
      org.opencontainers.image.revision=$SOURCE_REVISION \
      io.dsh.isolated.contract.version="v1alpha1" \
      io.dsh.isolated.dsh.version="0.1.2-rc.1"
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=dsh --chown=1000:1000 /opt/dsh /opt/dsh
COPY --from=launcher --chown=1000:1000 /out/cell-launcher /usr/local/bin/cell-launcher
COPY --chown=1000:1000 images/cell/cell.patch.yml /etc/dsh/cell.patch.yml
ENV HOME=/var/lib/dsh/data/home \
    DSH_HOME=/var/lib/dsh/data/home \
    DSH_AGENTS_HOME=/var/lib/dsh/data/agents \
    DSH_TELEMETRY_DISABLED=1 \
    XDG_CACHE_HOME=/tmp/.cache
USER 1000:1000
WORKDIR /var/lib/dsh/data
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/cell-launcher"]
