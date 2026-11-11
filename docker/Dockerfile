# Build with the workspace as context: docker build -f EdgeAdmin/docker/Dockerfile .
# The official golang:1.27.4-bookworm image is not published yet. Fetch the
# official Go release into Debian and verify the published release SHA-256.
FROM debian:bookworm-slim AS build
ARG TARGETARCH
ENV GOTOOLCHAIN=local
ENV PATH=/usr/local/go/bin:${PATH}
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl gcc libc6-dev git zip && rm -rf /var/lib/apt/lists/*
RUN set -eu; \
    case "$TARGETARCH" in \
      amd64) checksum=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840 ;; \
      arm64) checksum=3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92 ;; \
      *) echo "Unsupported build architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl --fail --location --proto '=https' --tlsv1.2 "https://go.dev/dl/go1.27.4.linux-${TARGETARCH}.tar.gz" -o /tmp/go.tar.gz; \
    echo "$checksum  /tmp/go.tar.gz" | sha256sum -c -; \
    tar -C /usr/local -xzf /tmp/go.tar.gz; rm /tmp/go.tar.gz
WORKDIR /src
COPY EdgeCommon/ /src/EdgeCommon/
COPY EdgeAdmin/ /src/EdgeAdmin/
WORKDIR /src/EdgeAdmin
RUN go mod download && go build -trimpath -tags community -o /out/admin/bin/edge-admin ./cmd/edge-admin
COPY EdgeAPI/ /src/EdgeAPI/
WORKDIR /src/EdgeAPI
RUN go mod download && go build -trimpath -tags community -o /out/admin/edge-api/bin/edge-api ./cmd/edge-api
RUN set -eu; mkdir -p /out/admin/edge-api/installers /out/admin/edge-api/deploy; \
    for arch in amd64 arm64; do \
      CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -tags community -o "/out/admin/edge-api/installers/edge-installer-helper-linux-$arch" ./cmd/installer-helper; \
    done
COPY EdgeNode/ /src/EdgeNode/
WORKDIR /src/EdgeNode
RUN go mod download && go build -trimpath -tags community -o /out/node/bin/edge-node ./cmd/edge-node
RUN set -eu; mkdir -p /out/node/configs /out/node/logs /out/node/data /out/node/doc/licenses; \
    cp build/configs/*.template.yaml /out/node/configs/; \
    cp build/configs/README.md /out/node/doc/security-configuration.md; \
    cp internal/delivery/README.md /out/node/doc/durable-delivery.md; \
    cp internal/waf/injectionutils/libinjection/COPYING /out/node/doc/licenses/libinjection.txt; \
    webp_dir="$(go list -m -f '{{.Dir}}' github.com/iwind/gowebp)"; \
    cp "$webp_dir/LICENSE" /out/node/doc/licenses/gowebp.txt; \
    cp "$webp_dir/internal/libwebp-1.3.2/COPYING" /out/node/doc/licenses/libwebp.txt; \
    cp -r build/pages /out/node/pages; cp -r build/www /out/node/www; \
    version=$(sed -n 's/.*Version[[:space:]]*=[[:space:]]*"\([0-9.]*\)".*/\1/p' internal/const/const.go | head -n 1); \
    test -n "$version"; \
    cd /out; mv node edge-node; zip -qr "admin/edge-api/deploy/edge-node-linux-${TARGETARCH}-v${version}.zip" edge-node; rm -rf edge-node
WORKDIR /src/EdgeAdmin
RUN set -eu; GithubDIR=/src CGO_ENABLED=0 go run -tags community ./cmd/edge-admin generate; mkdir -p /out/admin/configs /out/admin/logs /out/admin/data /out/admin/doc /out/admin/edge-api/configs /out/admin/edge-api/logs /out/admin/edge-api/data /out/admin/edge-api/docs; \
    cp -r web /out/admin/web; \
    cp web/public/js/components.src.js /out/admin/web/public/js/components.js; cp web/public/js/utils.js /out/admin/web/public/js/utils.min.js; \
    cp build/configs/*.template.yaml build/configs/upgrade-trust.template.json /out/admin/configs/; \
    cp build/configs/server.template.yaml /out/admin/server.template.yaml; \
    cp /src/EdgeAPI/build/configs/*.template.yaml /out/admin/edge-api/configs/; \
    cp /src/EdgeAPI/docs/authentication-migration.md /out/admin/edge-api/docs/; \
    cp doc/admin-security.md doc/signed-upgrades.md /out/admin/doc/; \
    chmod -R u=rwX,go=rX /out/admin

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 edge && useradd --uid 10001 --gid edge --home-dir /var/lib/edge-admin --create-home --shell /usr/sbin/nologin edge
ENV TZ=Asia/Shanghai
ENV HOME=/var/lib/edge-admin
ENV EDGE_ALLOW_LOOPBACK_RPC=true
WORKDIR /opt/edge-admin
COPY --from=build --chown=10001:10001 /out/admin/ /opt/edge-admin/
COPY --chown=10001:10001 EdgeAdmin/docker/entrypoint.sh /usr/local/bin/edge-admin-entrypoint
RUN chmod 0755 /usr/local/bin/edge-admin-entrypoint \
    && install -d -m 0700 -o edge -g edge /var/lib/edge-admin/.edge-admin /var/lib/edge-admin/.edge-api
USER 10001:10001
# Plaintext RPC is restricted to literal loopback IPs for the bundled API.
# Database and API ports are not published.
EXPOSE 7788
ENTRYPOINT ["/usr/local/bin/edge-admin-entrypoint"]
