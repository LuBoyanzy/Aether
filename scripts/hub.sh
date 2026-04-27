#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

resolve_env_file() {
  if [[ -n "${AETHER_HUB_ENV_FILE:-}" ]]; then
    echo "${AETHER_HUB_ENV_FILE}"
    return 0
  fi

  local candidates=(
    "${ROOT_DIR}/.env"
    "${ROOT_DIR}/../.env"
    "${ROOT_DIR}/../../.env"
    "${ROOT_DIR}/local-dev.env"
  )
  local path
  for path in "${candidates[@]}"; do
    if [[ -f "${path}" ]]; then
      echo "${path}"
      return 0
    fi
  done

  echo "${ROOT_DIR}/local-dev.env"
}

ENV_FILE="$(resolve_env_file)"

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

export_ingest_monitor_defaults() {
  export AETHER_HUB_INGEST_MONITOR_PG_HOST="${AETHER_HUB_INGEST_MONITOR_PG_HOST:-${INGEST_MONITOR_PG_HOST:-${POSTGRES_HOST:-${RELEASE_HOST:-127.0.0.1}}}}"
  export AETHER_HUB_INGEST_MONITOR_PG_PORT="${AETHER_HUB_INGEST_MONITOR_PG_PORT:-${INGEST_MONITOR_PG_PORT:-${POSTGRES_HOST_PORT:-5432}}}"
  export AETHER_HUB_INGEST_MONITOR_PG_USER="${AETHER_HUB_INGEST_MONITOR_PG_USER:-${INGEST_MONITOR_PG_USER:-${POSTGRES_USER:-}}}"
  export AETHER_HUB_INGEST_MONITOR_PG_PASSWORD="${AETHER_HUB_INGEST_MONITOR_PG_PASSWORD:-${INGEST_MONITOR_PG_PASSWORD:-${POSTGRES_PASSWORD:-}}}"
  export AETHER_HUB_INGEST_MONITOR_PG_DATABASE="${AETHER_HUB_INGEST_MONITOR_PG_DATABASE:-${INGEST_MONITOR_PG_DATABASE:-${POSTGRES_DB:-}}}"
  export AETHER_HUB_INGEST_MONITOR_PG_TENANT="${AETHER_HUB_INGEST_MONITOR_PG_TENANT:-${INGEST_MONITOR_PG_TENANT:-${INFERENGINEER_TENANT_ID:-}}}"
  export AETHER_HUB_INGEST_MONITOR_PG_SSLMODE="${AETHER_HUB_INGEST_MONITOR_PG_SSLMODE:-${INGEST_MONITOR_PG_SSLMODE:-disable}}"
}

export_ingest_monitor_defaults

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    FreeBSD) echo "freebsd" ;;
    *)
      echo "unsupported os: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    armv7l|armv7) echo "arm" ;;
    *)
      echo "unsupported arch: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

OS_NAME="$(detect_os)"
ARCH_NAME="$(detect_arch)"

HUB_BIN="${AETHER_HUB_BIN:-${ROOT_DIR}/build/aether_${OS_NAME}_${ARCH_NAME}}"
LOCAL_AGENT_BIN="${AETHER_HUB_LOCAL_AGENT_BIN:-${ROOT_DIR}/build/aether-agent_${OS_NAME}_${ARCH_NAME}}"
RUN_DIR="${AETHER_HUB_RUN_DIR:-${ROOT_DIR}/run/hub}"
PID_FILE="${AETHER_HUB_PID_FILE:-${RUN_DIR}/aether.pid}"
LOG_FILE="${AETHER_HUB_LOG_FILE:-${RUN_DIR}/aether.log}"
BESZEL_HOST_PORT="${BESZEL_HOST_PORT:-19090}"
HTTP_ADDR="${AETHER_HUB_HTTP_ADDR:-0.0.0.0:${BESZEL_HOST_PORT}}"
APP_URL="${APP_URL:-${BESZEL_APP_URL:-http://${RELEASE_HOST:-127.0.0.1}:${BESZEL_HOST_PORT}}}"

extract_port_from_addr() {
  local addr="${1:-}"
  local fallback_port="${2:-19090}"
  local port="${addr##*:}"
  if [[ "${port}" =~ ^[0-9]+$ ]]; then
    echo "${port}"
    return 0
  fi
  echo "${fallback_port}"
}

LOCAL_HEALTH_PORT="$(extract_port_from_addr "${HTTP_ADDR}" "${BESZEL_HOST_PORT}")"
HEALTH_URL="${AETHER_HUB_HEALTH_URL:-http://127.0.0.1:${LOCAL_HEALTH_PORT}/api/health}"

mkdir -p "${RUN_DIR}"

require_file() {
  local path="$1"
  local desc="$2"
  if [[ ! -f "${path}" ]]; then
    echo "${desc}不存在: ${path}" >&2
    exit 1
  fi
  if [[ ! -x "${path}" ]]; then
    echo "${desc}不可执行: ${path}" >&2
    exit 1
  fi
}

is_running() {
  if [[ ! -f "${PID_FILE}" ]]; then
    return 1
  fi
  local pid
  pid="$(tr -d '[:space:]' < "${PID_FILE}")"
  if [[ -z "${pid}" ]]; then
    rm -f "${PID_FILE}"
    return 1
  fi
  if kill -0 "${pid}" >/dev/null 2>&1; then
    return 0
  fi
  rm -f "${PID_FILE}"
  return 1
}

wait_stopped() {
  local pid="$1"
  local i
  for i in $(seq 1 40); do
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
      rm -f "${PID_FILE}"
      return 0
    fi
    sleep 0.25
  done
  return 1
}

wait_healthy() {
  if ! command -v curl >/dev/null 2>&1; then
    sleep 1
    return 0
  fi

  local i
  for i in $(seq 1 20); do
    if curl -fsS "${HEALTH_URL}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

listener_pid() {
  local port="$1"

  if command -v ss >/dev/null 2>&1; then
    ss -ltnp "( sport = :${port} )" 2>/dev/null \
      | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' \
      | head -n 1
    return 0
  fi

  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null \
      | awk 'NR == 2 { print $2 }'
    return 0
  fi

  return 0
}

start_hub() {
  require_file "${HUB_BIN}" "Hub二进制"
  require_file "${LOCAL_AGENT_BIN}" "本机Agent二进制"

  if is_running; then
    status_hub
    return 0
  fi

  : > "${LOG_FILE}"

  (
    cd "${ROOT_DIR}"
    nohup env \
      APP_URL="${APP_URL}" \
      AETHER_HUB_LOCAL_AGENT_BIN="${LOCAL_AGENT_BIN}" \
      "${HUB_BIN}" serve --http "${HTTP_ADDR}" >> "${LOG_FILE}" 2>&1 &
    echo $! > "${PID_FILE}"
  )

  if wait_healthy; then
    sleep 0.2
    if ! is_running; then
      local listener_pid_value
      listener_pid_value="$(listener_pid "${LOCAL_HEALTH_PORT}")"
      if [[ -n "${listener_pid_value}" ]]; then
        echo "Hub启动失败：端口 ${LOCAL_HEALTH_PORT} 已被其他进程占用（pid=${listener_pid_value}），当前 PID_FILE 中的进程已退出。" >&2
      else
        echo "Hub启动失败：健康检查命中了其他服务，当前 PID_FILE 中的进程已退出。" >&2
      fi
      return 1
    fi

    local pid listener_pid_value
    pid="$(tr -d '[:space:]' < "${PID_FILE}")"
    listener_pid_value="$(listener_pid "${LOCAL_HEALTH_PORT}")"
    if [[ -n "${listener_pid_value}" && "${listener_pid_value}" != "${pid}" ]]; then
      echo "Hub启动失败：端口 ${LOCAL_HEALTH_PORT} 当前监听进程为 pid=${listener_pid_value}，不是本次启动的 pid=${pid}。" >&2
      return 1
    fi

    echo "Hub已启动"
    status_hub
    return 0
  fi

  echo "Hub启动后健康检查失败，请查看日志: ${LOG_FILE}" >&2
  status_hub || true
  return 1
}

stop_hub() {
  if ! is_running; then
    echo "Hub未运行"
    return 0
  fi

  local pid
  pid="$(tr -d '[:space:]' < "${PID_FILE}")"
  kill "${pid}" >/dev/null 2>&1 || true
  if wait_stopped "${pid}"; then
    echo "Hub已停止"
    return 0
  fi

  echo "Hub停止超时，可执行: $0 kill" >&2
  return 1
}

kill_hub() {
  if ! is_running; then
    echo "Hub未运行"
    return 0
  fi

  local pid
  pid="$(tr -d '[:space:]' < "${PID_FILE}")"
  kill -9 "${pid}" >/dev/null 2>&1 || true
  rm -f "${PID_FILE}"
  echo "Hub已强制终止"
}

status_hub() {
  echo "ENV_FILE=${ENV_FILE}"
  echo "HUB_BIN=${HUB_BIN}"
  echo "LOCAL_AGENT_BIN=${LOCAL_AGENT_BIN}"
  echo "APP_URL=${APP_URL}"
  echo "HTTP_ADDR=${HTTP_ADDR}"
  echo "LOG_FILE=${LOG_FILE}"
  echo "PID_FILE=${PID_FILE}"
  echo "INGEST_MONITOR_PG_HOST=${AETHER_HUB_INGEST_MONITOR_PG_HOST:-}"
  echo "INGEST_MONITOR_PG_PORT=${AETHER_HUB_INGEST_MONITOR_PG_PORT:-}"
  echo "INGEST_MONITOR_PG_DATABASE=${AETHER_HUB_INGEST_MONITOR_PG_DATABASE:-}"
  echo "INGEST_MONITOR_PG_TENANT=${AETHER_HUB_INGEST_MONITOR_PG_TENANT:-}"
  if is_running; then
    echo "STATUS=running"
    echo "PID=$(tr -d '[:space:]' < "${PID_FILE}")"
  else
    echo "STATUS=stopped"
  fi
}

show_logs() {
  if [[ ! -f "${LOG_FILE}" ]]; then
    echo "日志文件不存在: ${LOG_FILE}"
    return 0
  fi

  if [[ "${2:-}" == "--follow" ]]; then
    tail -n 100 -f "${LOG_FILE}"
    return 0
  fi
  tail -n 100 "${LOG_FILE}"
}

usage() {
  cat <<EOF
用法: $(basename "$0") {start|stop|restart|status|kill|logs [--follow]}
EOF
}

case "${1:-}" in
  start)
    start_hub
    ;;
  stop)
    stop_hub
    ;;
  restart)
    stop_hub || true
    start_hub
    ;;
  status)
    status_hub
    ;;
  kill)
    kill_hub
    ;;
  logs)
    show_logs "$@"
    ;;
  *)
    usage
    exit 1
    ;;
esac
