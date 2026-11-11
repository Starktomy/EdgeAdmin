# Build and run the administrator container

This image builds community components from the four sibling source checkouts.
It no longer downloads the old unsigned Plus ZIP or installs a database with a
fixed root password. The runtime uses UID/GID `10001`, contains no Go compiler or
MySQL server, and only declares the administrator HTTP port `7788`.

Build from the workspace (`EdgeAdmin/`, `EdgeAPI/`, `EdgeNode/`, `EdgeCommon/`):

```sh
./EdgeAdmin/docker/build.sh
```

The official `golang:1.27.4-bookworm` image was not available when this change was
prepared. The builder uses Debian Bookworm plus the official Go 1.27.4 release,
verified against pinned SHA-256 for amd64/arm64. Build for the native platform
(or use Docker's explicit platform/emulation support). The Dockerfile-specific
ignore file excludes live configuration, keys, logs, data, archives and Git
metadata from the workspace context. Review additions before building.

## State and external database

Create a new state location, keeping old installations/backups separate:

```sh
state=/srv/edge-admin
sudo install -d -m 0700 -o 10001 -g 10001 "$state" \
  "$state/configs" "$state/logs" "$state/data" "$state/api-configs" \
  "$state/api-logs" "$state/api-data" "$state/home"
sudo cp EdgeAdmin/build/configs/server.template.yaml "$state/configs/server.yaml"
sudo chown -R 10001:10001 "$state"
sudo chmod 0600 "$state/configs/server.yaml"
```

For an existing installation, migrate its administrator/API credential and
database YAML files into the corresponding mounted directories. For a fresh
installation, keep credentials absent and complete the setup wizard through the
local TLS proxy, selecting an existing external database (not the MySQL installer).
Edit the local configuration before exposure;
use a dedicated database account, unique password and a separately managed
MySQL-compatible database. The container cannot install/manage host systemd,
firewall or MySQL. Do not copy the old image's root/123456 database credentials.
The setup wizard can configure the bundled API for `127.0.0.1` on an
unprivileged port; do not expose its RPC port. The image sets
`EDGE_ALLOW_LOOPBACK_RPC=true` for that connection: plaintext RPC accepts literal
loopback IPs (`127.0.0.1` or `::1`) only, and the API must bind a loopback IP.
External HTTP APIs and DNS names (including `localhost`) are rejected. External
APIs require verified HTTPS; mount a private CA bundle and set `EDGE_RPC_CA_FILE`
when needed. The broad `EDGE_ALLOW_PLAINTEXT_RPC` migration flag is not set by the
image. No database port is published.

Run with the prepared state:

```sh
EDGE_ADMIN_STATE_DIR=/srv/edge-admin ./EdgeAdmin/docker/run.sh
```

The script binds `7788` only to host loopback for a TLS reverse proxy, drops Linux
capabilities and enables no-new-privileges. Configure
`EDGE_TRUSTED_PROXY_CIDRS` explicitly when forwarding client headers; the provided
script does not automatically trust the proxy. Add appropriate environment
options when adapting the Docker command for your deployment. Use nonprivileged
HTTPS/RPC listening ports in the mounted configs.

Existing configuration should be migrated to the new state paths with ownership
`10001:10001` and `0600` modes; inspect and rotate exposed historical credentials.
Take a database backup before deploying a schema upgrade. Automatic upgrades
inside this immutable container are unsupported: rebuild and replace the image
with verified source instead. Data volumes and the external database must survive
container replacement; the image's bundled executable tree can be recreated.
