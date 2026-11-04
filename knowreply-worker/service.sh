#!/usr/bin/env bash
# =============================================================================
# knowreply-worker 启停脚本（本服务自带，不依赖仓库根目录的任何脚本）
#
# 本模块有两个可运行进程：
#   worker —— asynq 消费者，文档入库/下线流水线
#   kbapi  —— 检索预览 HTTP 服务（内部工具，默认只监听 127.0.0.1:8081）
#
#   ./service.sh start [worker|kbapi|all]   默认 all
#   ./service.sh stop  [worker|kbapi|all]   默认 all
#   ./service.sh restart [worker|kbapi|all]
#   ./service.sh status
#   ./service.sh logs <worker|kbapi>
#   ./service.sh build [worker|kbapi|all]
#
# 产物：bin/<name> ｜ 日志：logs/<name>.log ｜ PID：run/<name>.pid
# 配置：优先读取仓库根目录的 .env（已被 .gitignore 忽略），其次使用下面的默认值。
# =============================================================================

set -euo pipefail

SERVICE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SERVICE_DIR/.." && pwd)"

ALL_TARGETS=(worker kbapi)

# ---------- 环境变量 ----------
load_env() {
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

  : "${KB_API_ADDR:=127.0.0.1:8081}"

  export REDIS_ADDR REDIS_PASSWORD REDIS_DB_QUEUE \
    MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE \
    PG_DSN EMBEDDING_PROVIDER EMBEDDING_DIM EMBEDDING_BATCH_SIZE \
    CHUNK_MIN_RUNES CHUNK_MAX_RUNES WORKER_QUEUE WORKER_CONCURRENCY \
    DOC_STORAGE_ROOT KB_API_ADDR
}

# ---------- 目标元数据 ----------
target_pkg() {
  case "$1" in
    worker) echo "./cmd/worker" ;;
    kbapi) echo "./cmd/kbapi" ;;
    *) return 1 ;;
  esac
}

target_health() {
  case "$1" in
    worker) echo "" ;;
    kbapi) echo "http://127.0.0.1:8081/healthz" ;;
  esac
}

bin_file() { echo "$SERVICE_DIR/bin/$1"; }
pid_file() { echo "$SERVICE_DIR/run/$1.pid"; }
log_file() { echo "$SERVICE_DIR/logs/$1.log"; }

is_running() {
  local pf
  pf="$(pid_file "$1")"
  [[ -f "$pf" ]] || return 1
  local pid
  pid="$(cat "$pf")"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

# ---------- 构建 ----------
do_build() {
  local targets=("$@")
  [[ ${#targets[@]} -eq 0 ]] && targets=("${ALL_TARGETS[@]}")
  mkdir -p "$SERVICE_DIR/bin"

  for name in "${targets[@]}"; do
    echo "  编译 $name ..."
    (cd "$SERVICE_DIR" && go build -buildvcs=false -o "$(bin_file "$name")" "$(target_pkg "$name")")
    echo "  完成：$(bin_file "$name")"
  done
}

# ---------- 启动 ----------
start_one() {
  local name="$1" bin pf log pid

  bin="$(bin_file "$name")"
  pf="$(pid_file "$name")"
  log="$(log_file "$name")"

  if is_running "$name"; then
    echo "  $name 已在运行 (pid $(cat "$pf"))"
    return 0
  fi

  [[ -x "$bin" ]] || do_build "$name"

  nohup "$bin" >>"$log" 2>&1 &
  pid=$!
  echo "$pid" >"$pf"

  sleep 1
  if kill -0 "$pid" 2>/dev/null; then
    echo "  $name started  pid=$pid  log=$log"
  else
    echo "  $name 启动失败，最近日志："
    tail -n 15 "$log" | sed 's/^/    /'
    rm -f "$pf"
    return 1
  fi
}

do_start() {
  load_env
  mkdir -p "$SERVICE_DIR/bin" "$SERVICE_DIR/run" "$SERVICE_DIR/logs" "$DOC_STORAGE_ROOT"

  local targets=("$@")
  if [[ ${#targets[@]} -eq 0 || "${targets[0]}" == "all" ]]; then
    targets=("${ALL_TARGETS[@]}")
  fi

  if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^knowreply-postgres$'; then
    echo "  警告：knowreply-postgres 未运行，向量写入会失败"
  fi

  for name in "${targets[@]}"; do
    start_one "$name"
  done
}

# ---------- 停止 ----------
stop_one() {
  local name="$1" pf pid
  pf="$(pid_file "$name")"

  if ! is_running "$name"; then
    echo "  $name 未在运行"
    rm -f "$pf"
    return 0
  fi

  pid="$(cat "$pf")"
  kill -TERM "$pid" 2>/dev/null || true

  for _ in $(seq 1 30); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.5
  done

  if kill -0 "$pid" 2>/dev/null; then
    echo "  $name 15s 内未退出，强制结束"
    kill -KILL "$pid" 2>/dev/null || true
  fi

  rm -f "$pf"
  echo "  $name stopped"
}

do_stop() {
  local targets=("$@")
  if [[ ${#targets[@]} -eq 0 || "${targets[0]}" == "all" ]]; then
    targets=("${ALL_TARGETS[@]}")
  fi

  for name in "${targets[@]}"; do
    stop_one "$name"
  done
}

# ---------- 状态 ----------
do_status() {
  load_env
  printf '  %-8s %-8s %-8s %s\n' "服务" "状态" "PID" "健康"

  local name state pid url health pf
  for name in "${ALL_TARGETS[@]}"; do
    state="stopped"
    pid="-"
    health="-"
    url="$(target_health "$name")"
    pf="$(pid_file "$name")"

    if is_running "$name"; then
      state="running"
      pid="$(cat "$pf")"
      if [[ -n "$url" ]] && command -v curl >/dev/null 2>&1; then
        curl -fsS -m 2 "$url" >/dev/null 2>&1 && health="ok" || health="fail"
      fi
    fi

    printf '  %-8s %-8s %-8s %s\n' "$name" "$state" "$pid" "$health"
  done
}

do_logs() {
  local name="${1:-}"
  if [[ -z "$name" ]]; then
    echo "用法：$0 logs <worker|kbapi>"
    exit 1
  fi
  mkdir -p "$SERVICE_DIR/logs"
  touch "$(log_file "$name")"
  tail -f "$(log_file "$name")"
}

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//' | grep -v '^=\{10,\}$'
}

ACTION="${1:-}"
shift || true

case "$ACTION" in
  start) do_start "$@" ;;
  stop) do_stop "$@" ;;
  restart)
    do_stop "$@"
    sleep 1
    do_start "$@"
    ;;
  status) do_status ;;
  logs) do_logs "$@" ;;
  build) do_build "$@" ;;
  *) usage; exit 1 ;;
esac
