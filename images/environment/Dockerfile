FROM --platform=$BUILDPLATFORM golang:1.27.1-bookworm@sha256:648f440f42a0958804efb24df176f806f9d353b41f1c0627f666428e40310f6b AS launcher
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/environment-launcher ./cmd/environment-launcher
COPY internal/accesscontract ./internal/accesscontract
COPY internal/dshcompat ./internal/dshcompat
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w -buildid=' -o /out/environment-launcher ./cmd/environment-launcher

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS dsh
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates python3 make g++ \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/dsh
COPY images/environment/package.json images/environment/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --omit=dev --no-audit --no-fund \
    && node node_modules/@deepseek-ai/dsh/lib/bin.js --version
COPY compat/dsh /compat/dsh
RUN node /compat/dsh/apply-patches.mjs /opt/dsh

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=dsh /opt/dsh /opt/dsh
COPY --from=launcher /out/environment-launcher /usr/local/bin/environment-launcher
COPY images/environment/environment.patch.yml /etc/dsh/environment.patch.yml
ARG SOURCE_REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/GuoMonth/dsh-isolated-runtime" \
      org.opencontainers.image.revision=$SOURCE_REVISION \
      io.dsh.isolated.contract.version="dsh-rc2-single-pvc-v1" \
      io.dsh.isolated.dsh.version="0.2.0-rc.2"
ENV HOME=/var/lib/dsh/data/home DSH_HOME=/var/lib/dsh/data/dsh \
    DSH_TELEMETRY_DISABLED=1 XDG_CACHE_HOME=/tmp/.cache \
    NPM_CONFIG_PREFIX=/var/lib/dsh/data/home/.local NPM_CONFIG_CACHE=/tmp/.npm \
    PATH=/var/lib/dsh/data/home/.local/bin:$PATH
USER 1000:1000
WORKDIR /var/lib/dsh/data
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/environment-launcher"]
