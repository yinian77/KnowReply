#!/usr/bin/env bash
# =============================================================================
# knowreply-gateway 启停脚本（本服务自带，不依赖仓库根目录的任何脚本）
#
#   ./scripts/service.sh start     # 编译（缺二进制时）+ 后台启动
#   ./scripts/service.sh stop      # 优雅停止（TERM，15s 后 KILL）
#   ./scripts/service.sh restart
#   ./scripts/service.sh status    # 进程 + /readyz 健康检查
#   ./scripts/service.sh logs      # 跟随日志
#   ./scripts/service.sh build     # 仅编译
#
# 产物：bin/gateway ｜ 日志：logs/gateway.log ｜ PID：run/gateway.pid
# 配置：优先读取仓库根目录的 .env（已被 .gitignore 忽略），其次使用下面的默认值。
# =============================================================================

set -euo pipefail

SERVICE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_DIR="$(cd "$SERVICE_DIR/.." && pwd)"

BIN="$SERVICE_DIR/bin/gateway"
PID_FILE="$SERVICE_DIR/run/gateway.pid"
LOG_FILE="$SERVICE_DIR/logs/gateway.log"
HEALTH_URL="http://127.0.0.1:8080/readyz"

# ---------- 环境变量 ----------
load_env() {
  if [[ -f "$REPO_DIR/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "$REPO_DIR/.env"
    set +a
  fi

  : "${APP_ENV:=dev}"
  : "${HTTP_ADDR:=:8080}"

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
}

is_running() {
  [[ -f "$PID_FILE" ]] || return 1
  local pid
  pid="$(cat "$PID_FILE")"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

do_build() {
  mkdir -p "$SERVICE_DIR/bin"
  echo "  编译 gateway ..."
  # 目录属主可能不是当前用户，-buildvcs=false 规避 git dubious ownership 报错
  (cd "$SERVICE_DIR" && go build -buildvcs=false -o "$BIN" ./cmd/gateway)
  echo "  完成：$BIN"
}

do_start() {
  load_env
  mkdir -p "$SERVICE_DIR/bin" "$SERVICE_DIR/run" "$SERVICE_DIR/logs" "$DOC_STORAGE_ROOT"

  if is_running; then
    echo "  gateway 已在运行 (pid $(cat "$PID_FILE"))"
    return 0
  fi

  [[ -x "$BIN" ]] || do_build

  if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^knowreply-postgres$'; then
    echo "  警告：knowreply-postgres 未运行，知识库相关功能会失败"
  fi

  nohup "$BIN" >>"$LOG_FILE" 2>&1 &
  echo "$!" >"$PID_FILE"

  sleep 1
  if kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "  gateway started  pid=$(cat "$PID_FILE")  addr=$HTTP_ADDR  log=$LOG_FILE"
  else
    echo "  gateway 启动失败，最近日志："
    tail -n 15 "$LOG_FILE" | sed 's/^/    /'
    rm -f "$PID_FILE"
    return 1
  fi
}

do_stop() {
  if ! is_running; then
    echo "  gateway 未在运行"
    rm -f "$PID_FILE"
    return 0
  fi

  local pid
  pid="$(cat "$PID_FILE")"
  kill -TERM "$pid" 2>/dev/null || true

  for _ in $(seq 1 30); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.5
  done

  if kill -0 "$pid" 2>/dev/null; then
    echo "  15s 内未退出，强制结束"
    kill -KILL "$pid" 2>/dev/null || true
  fi

  rm -f "$PID_FILE"
  echo "  gateway stopped"
}

do_status() {
  load_env
  if is_running; then
    local health="fail"
    if command -v curl >/dev/null 2>&1 && curl -fsS -m 2 "$HEALTH_URL" >/dev/null 2>&1; then
      health="ok"
    fi
    printf '  gateway  running  pid=%s  addr=%s  health=%s\n' "$(cat "$PID_FILE")" "$HTTP_ADDR" "$health"
  else
    echo "  gateway  stopped"
  fi
}

do_logs() {
  mkdir -p "$SERVICE_DIR/logs"
  touch "$LOG_FILE"
  tail -f "$LOG_FILE"
}

usage() {
  sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  start) do_start ;;
  stop) do_stop ;;
  restart)
    do_stop
    sleep 1
    do_start
    ;;
  status) do_status ;;
  logs) do_logs ;;
  build) do_build ;;
  *) usage; exit 1 ;;
esac
