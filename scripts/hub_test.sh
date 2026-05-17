#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    echo "expected output to contain: ${needle}" >&2
    echo "--- output ---" >&2
    echo "${haystack}" >&2
    exit 1
  fi
}

test_status_ignores_pid_from_other_binary() {
  local tmp_dir=""
  tmp_dir="$(mktemp -d)"
  local other_pid=""
  trap '[[ -n "${other_pid:-}" ]] && kill "${other_pid}" >/dev/null 2>&1 || true; rm -rf "${tmp_dir}"' RETURN

  local env_file="${tmp_dir}/env"
  local hub_bin="${tmp_dir}/aether_linux_amd64"
  local agent_bin="${tmp_dir}/aether-agent_linux_amd64"
  local pid_file="${tmp_dir}/aether.pid"
  local log_file="${tmp_dir}/aether.log"

  : > "${env_file}"
  printf '#!/usr/bin/env bash\nexit 0\n' > "${hub_bin}"
  printf '#!/usr/bin/env bash\nexit 0\n' > "${agent_bin}"
  chmod +x "${hub_bin}" "${agent_bin}"

  sleep 60 &
  other_pid="$!"
  printf '%s\n' "${other_pid}" > "${pid_file}"

  local output=""
  output="$(
    AETHER_HUB_ENV_FILE="${env_file}" \
    AETHER_HUB_BIN="${hub_bin}" \
    AETHER_HUB_LOCAL_AGENT_BIN="${agent_bin}" \
    AETHER_HUB_RUN_DIR="${tmp_dir}" \
    AETHER_HUB_PID_FILE="${pid_file}" \
    AETHER_HUB_LOG_FILE="${log_file}" \
    BESZEL_HOST_PORT=19100 \
    APP_URL=http://127.0.0.1:19100 \
      bash "${ROOT_DIR}/scripts/hub.sh" status
  )"

  assert_contains "${output}" "STATUS=stopped"
  if [[ -f "${pid_file}" ]]; then
    echo "expected stale pid file to be removed" >&2
    exit 1
  fi
}

test_status_ignores_pid_from_other_binary
echo "hub.sh tests passed"
