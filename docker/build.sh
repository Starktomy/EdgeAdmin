#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
workspace_dir=$(cd -- "$script_dir/../.." && pwd)
image_tag=${EDGE_ADMIN_IMAGE:-goedge/edge-admin:source-go1.27.4}
for component in EdgeCommon EdgeAPI EdgeNode EdgeAdmin; do
  test -f "$workspace_dir/$component/go.mod" || { printf 'Missing sibling %s checkout\n' "$component" >&2; exit 1; }
done
docker build --pull --file "$script_dir/Dockerfile" --tag "$image_tag" "$workspace_dir"
