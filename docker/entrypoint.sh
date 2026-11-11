#!/bin/sh
set -eu
# Initialize the writable mounted configuration directory without copying any
# credentials or using a fixed database password. Complete setup through a TLS
# proxy using the separately managed database.
if [ ! -f /opt/edge-admin/configs/server.yaml ]; then
    umask 077
    cp /opt/edge-admin/server.template.yaml /opt/edge-admin/configs/server.yaml
fi
exec /opt/edge-admin/bin/edge-admin "$@"
