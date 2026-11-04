#!/usr/bin/env bash
# 编译 worker 产物到 bin/（默认编译 worker + kbapi）
#
#   ./build.sh          # 编译全部
#   ./build.sh worker   # 只编译 worker
#   ./build.sh kbapi    # 只编译 kbapi
set -euo pipefail

SERVICE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGETS=("$@")

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  TARGETS=(worker kbapi)
fi

mkdir -p "$SERVICE_DIR/bin"
cd "$SERVICE_DIR"

for name in "${TARGETS[@]}"; do
  echo "编译 $name ..."
  go build -buildvcs=false -o "bin/$name" "./cmd/$name"
  echo "产物：$SERVICE_DIR/bin/$name"
done
