FROM --platform=$BUILDPLATFORM golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac AS launcher
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY cmd/cell-launcher ./cmd/cell-launcher
COPY internal/accesscontract ./internal/accesscontract
COPY internal/cellcontract ./internal/cellcontract
COPY internal/dshcompat ./internal/dshcompat
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w -buildid=' -o /out/cell-launcher ./cmd/cell-launcher

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e AS dsh
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates git python3 make g++ \
    && rm -rf /var/lib/apt/lists/* && corepack enable
COPY compat/dsh/baseline.json /baseline.json
WORKDIR /src
RUN node -e 'const b=require("/baseline.json"); process.stdout.write(b.distribution.archive)' > /archive-url \
    && curl -fsSL --retry 5 "$(cat /archive-url)" -o /source.tgz \
    && node -e 'const b=require("/baseline.json"); process.stdout.write(b.distribution.sha256+"  /source.tgz\n")' | sha256sum -c - \
    && tar -xzf /source.tgz --strip-components=1 && rm /source.tgz \
    && node -e 'const fs=require("fs"),c=require("crypto"),b=require("/baseline.json"); if(require("./package.json").version!==b.source.version || c.createHash("sha256").update(fs.readFileSync("pnpm-lock.yaml")).digest("hex")!==b.toolchain.lockfileSHA256)process.exit(1)'
ENV CI=1 DSH_TELEMETRY_DISABLED=1 npm_config_nodedir=/usr/local
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile --ignore-scripts \
    && pnpm --recursive rebuild esbuild fs-ext node-pty koffi
COPY compat/dsh /compat/dsh
RUN node /compat/dsh/apply-patches.mjs /src
RUN DSH_CLIENT_COMMIT_HASH="$(node -p 'require("/baseline.json").source.commit')" pnpm run build:official
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm --filter dsh-python-runtime-closure deploy --legacy --prod \
       --config.node-linker=hoisted --config.auto-install-peers=false --config.link-workspace-packages=true /opt/dsh
COPY images/cell/deploy.mjs /deploy.mjs
RUN node /deploy.mjs /src /opt/dsh \
    && node /opt/dsh/node_modules/@deepseek-ai/dsh/lib/bin.js --version

FROM node:24-bookworm-slim@sha256:ba849c60be29959425b8734d57b8b4b7d56f98edd9504c9af091d5281095a71e
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=dsh --chown=1000:1000 /opt/dsh /opt/dsh
COPY --from=launcher --chown=1000:1000 /out/cell-launcher /usr/local/bin/cell-launcher
COPY --chown=1000:1000 images/cell/cell.patch.yml /etc/dsh/cell.patch.yml
ARG SOURCE_REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/GuoMonth/dsh-isolated-runtime" \
      org.opencontainers.image.revision=$SOURCE_REVISION \
      io.dsh.isolated.contract.version="v1alpha1" \
      io.dsh.isolated.dsh.version="0.1.3-alpha.1"
ENV HOME=/var/lib/dsh/data/home \
    DSH_HOME=/var/lib/dsh/data/home \
    DSH_AGENTS_HOME=/var/lib/dsh/data/agents \
    DSH_TELEMETRY_DISABLED=1 \
    XDG_CACHE_HOME=/tmp/.cache
USER 1000:1000
WORKDIR /var/lib/dsh/data
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/cell-launcher"]
