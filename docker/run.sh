#!/usr/bin/env bash
set -euo pipefail
: "${EDGE_ADMIN_STATE_DIR:?Set EDGE_ADMIN_STATE_DIR to an absolute state directory prepared for UID/GID 10001}"
image_tag=${EDGE_ADMIN_IMAGE:-goedge/edge-admin:source-go1.27.4}
docker run --detach --init --name edge-admin \
  --publish 127.0.0.1:7788:7788 \
  --cap-drop=ALL --security-opt=no-new-privileges \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/configs,dst=/opt/edge-admin/configs" \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/logs,dst=/opt/edge-admin/logs" \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/data,dst=/opt/edge-admin/data" \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/api-configs,dst=/opt/edge-admin/edge-api/configs" \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/api-logs,dst=/opt/edge-admin/edge-api/logs" \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/api-data,dst=/opt/edge-admin/edge-api/data" \
  --mount "type=bind,src=$EDGE_ADMIN_STATE_DIR/home,dst=/var/lib/edge-admin" \
  "$image_tag"
