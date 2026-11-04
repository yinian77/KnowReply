#!/usr/bin/env bash
# 编译网关产物到 bin/gateway
set -euo pipefail

SERVICE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "$SERVICE_DIR/bin"
echo "编译 gateway ..."
cd "$SERVICE_DIR"
go build -buildvcs=false -o bin/gateway ./cmd/gateway
echo "产物：$SERVICE_DIR/bin/gateway"
