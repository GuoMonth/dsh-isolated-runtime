# shellcheck shell=bash
# One exact reference stack shared by the demo and regression gates.
envoy_gateway_version="v1.9.1"
envoy_gateway_sha256="72b3971364f172eb0b9636c7142cc84ff695467bc065897958bde85a3c06cfd5"
envoy_gateway_image="envoyproxy/gateway:${envoy_gateway_version}"
playwright_image="mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e"
dex_image="ghcr.io/dexidp/dex@sha256:8499afd690c437f52301efd2b05b2455da5bd2dfc20332cd697dc9937f808462"
dex_cache_image="dsh-phase2-dex-cache:test"
envoy_data_plane_image="docker.io/envoyproxy/envoy:distroless-v1.39.1@sha256:eb2c01c13125d1629637cb4e4cce7207009fb7cc2c8027f9742758549d15b6f4"
envoy_data_plane_cache_image="dsh-phase2-envoy-cache:test"
envoy_shutdown_image="docker.io/envoyproxy/gateway-dev@sha256:a4a8e6e8135d61a91b6c57859929991bae67b1ddfe1f923ee786fbf4b4253331"
envoy_shutdown_cache_image="dsh-phase2-envoy-shutdown-cache:test"
