#!/usr/bin/env bash
# 启动网关（前台运行 go run，Ctrl-C 停止）
#
#   ./start.sh                 # 使用下面的默认配置
#   HTTP_ADDR=:6705 ./start.sh # 覆盖任意环境变量
#   # 也可在仓库根目录放 .env，脚本会自动加载
set -euo pipefail

SERVICE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SERVICE_DIR/.." && pwd)"

# ---------- 环境变量：优先仓库根 .env，其次默认值 ----------
if [[ -f "$REPO_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$REPO_DIR/.env"
  set +a
fi

: "${APP_ENV:=dev}"
: "${HTTP_ADDR:=:6701}"

: "${MYSQL_HOST:=127.0.0.1}"
: "${MYSQL_PORT:=13406}"
: "${MYSQL_USER:=knowreply}"
: "${MYSQL_PASSWORD:=knowreply_2026_mysql}"
: "${MYSQL_DATABASE:=knowreply}"

: "${REDIS_ADDR:=127.0.0.1:16479}"
: "${REDIS_PASSWORD:=localdev_only_change_me}"
: "${REDIS_DB_CACHE:=1}"
: "${REDIS_DB_QUEUE:=2}"

: "${JWT_SECRET:=dev_only_change_me}"
: "${DOC_STORAGE_ROOT:=$REPO_DIR/data/docs}"
: "${DOC_QUEUE:=ingest}"

export APP_ENV HTTP_ADDR MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE \
  REDIS_ADDR REDIS_PASSWORD REDIS_DB_CACHE REDIS_DB_QUEUE \
  JWT_SECRET DOC_STORAGE_ROOT DOC_QUEUE

mkdir -p "$DOC_STORAGE_ROOT"

echo "启动 gateway  addr=$HTTP_ADDR  env=$APP_ENV"
cd "$SERVICE_DIR"
# 目录属主可能不是当前用户，-buildvcs=false 规避 git dubious ownership 报错
exec env GOFLAGS=-buildvcs=false go run ./cmd/gateway
