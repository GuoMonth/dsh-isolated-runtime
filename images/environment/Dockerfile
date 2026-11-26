FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea AS launcher
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/environment-launcher ./cmd/environment-launcher
COPY internal/accesscontract ./internal/accesscontract
COPY internal/dshcompat ./internal/dshcompat
COPY compat/dsh/baseline.json compat/dsh/pin.go ./compat/dsh/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w -buildid=' -o /out/environment-launcher ./cmd/environment-launcher

FROM node:24.22.3-bookworm-slim@sha256:873f586f3794f52281bd039bf02c048a0be08fde619338f9941bad1eb6b34530 AS dsh
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates python3 make g++ \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/dsh
COPY images/environment/package.json images/environment/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --engine-strict --omit=dev --no-audit --no-fund \
    && node node_modules/@deepseek-ai/dsh/lib/bin.js --version

FROM node:24.22.3-bookworm-slim@sha256:873f586f3794f52281bd039bf02c048a0be08fde619338f9941bad1eb6b34530
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=dsh /opt/dsh /opt/dsh
COPY --from=launcher /out/environment-launcher /usr/local/bin/environment-launcher
COPY images/environment/environment.patch.yml /etc/dsh/environment.patch.yml
ARG SOURCE_REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/GuoMonth/dsh-isolated-runtime" \
      org.opencontainers.image.revision=$SOURCE_REVISION \
      io.dsh.isolated.contract.version="dsh-021-single-pvc-v1" \
      io.dsh.isolated.dsh.version="0.2.2-rc.5"
ENV HOME=/var/lib/dsh/data/home DSH_HOME=/var/lib/dsh/data/dsh \
    DSH_TELEMETRY_DISABLED=1 XDG_CACHE_HOME=/tmp/.cache \
    NPM_CONFIG_PREFIX=/var/lib/dsh/data/home/.local NPM_CONFIG_CACHE=/tmp/.npm \
    PATH=/var/lib/dsh/data/home/.local/bin:$PATH
USER 1000:1000
WORKDIR /var/lib/dsh/data
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/environment-launcher"]
