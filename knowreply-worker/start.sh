#!/usr/bin/env bash
# 启动 worker 模块（前台运行 go run，Ctrl-C 停止）
#
#   ./start.sh              # 默认同时启动 worker + kbapi
#   ./start.sh worker       # 只启动 asynq 消费者（无端口）
#   ./start.sh kbapi        # 只启动检索预览服务（127.0.0.1:6702）
#   # 也可在仓库根目录放 .env，脚本会自动加载
set -euo pipefail

SERVICE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SERVICE_DIR/.." && pwd)"
NAME="${1:-all}"

case "$NAME" in
  all | worker | kbapi) ;;
  *)
    echo "用法：$0 [all|worker|kbapi]" >&2
    exit 1
    ;;
esac

# ---------- 环境变量：优先仓库根 .env，其次默认值 ----------
if [[ -f "$REPO_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$REPO_DIR/.env"
  set +a
fi

: "${REDIS_ADDR:=127.0.0.1:16479}"
: "${REDIS_PASSWORD:=localdev_only_change_me}"
: "${REDIS_DB_QUEUE:=2}"

: "${MYSQL_HOST:=127.0.0.1}"
: "${MYSQL_PORT:=13406}"
: "${MYSQL_USER:=knowreply}"
: "${MYSQL_PASSWORD:=knowreply_2026_mysql}"
: "${MYSQL_DATABASE:=knowreply}"

: "${PG_DSN:=postgres://knowreply:knowreply_2026_pg@127.0.0.1:15432/knowreply?sslmode=disable}"

: "${EMBEDDING_PROVIDER:=mock}"
: "${EMBEDDING_DIM:=1536}"
: "${EMBEDDING_BATCH_SIZE:=16}"

: "${CHUNK_MIN_RUNES:=300}"
: "${CHUNK_MAX_RUNES:=600}"

: "${WORKER_QUEUE:=ingest}"
: "${WORKER_CONCURRENCY:=4}"
: "${DOC_STORAGE_ROOT:=$REPO_DIR/data/docs}"

: "${KB_API_ADDR:=127.0.0.1:6702}"

export REDIS_ADDR REDIS_PASSWORD REDIS_DB_QUEUE \
  MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE \
  PG_DSN EMBEDDING_PROVIDER EMBEDDING_DIM EMBEDDING_BATCH_SIZE \
  CHUNK_MIN_RUNES CHUNK_MAX_RUNES WORKER_QUEUE WORKER_CONCURRENCY \
  DOC_STORAGE_ROOT KB_API_ADDR

mkdir -p "$DOC_STORAGE_ROOT"
cd "$SERVICE_DIR"

# 目录属主可能不是当前用户，-buildvcs=false 规避 git dubious ownership 报错
run_cmd() { env GOFLAGS=-buildvcs=false go run "./cmd/$1"; }

if [[ "$NAME" != "all" ]]; then
  echo "启动 $NAME"
  exec env GOFLAGS=-buildvcs=false go run "./cmd/$NAME"
fi

# 默认：两个进程一起前台运行，Ctrl-C 一起停
echo "启动 worker + kbapi  (kbapi 监听 $KB_API_ADDR，Ctrl-C 同时停止)"

run_cmd worker &
WORKER_PID=$!
run_cmd kbapi &
KBAPI_PID=$!

cleanup() {
  trap - INT TERM EXIT
  # go run 会再派生子进程，先按父进程精确回收自己的子进程，再收 go 进程本身，
  # 避免误杀其他终端里跑的同名进程
  pkill -TERM -P "$WORKER_PID" 2>/dev/null || true
  pkill -TERM -P "$KBAPI_PID" 2>/dev/null || true
  kill "$WORKER_PID" "$KBAPI_PID" 2>/dev/null || true
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

wait
