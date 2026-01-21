#!/bin/sh

is_alpine() {
  [ -f /etc/alpine-release ]
}

is_openwrt() {
  grep -qi "OpenWrt" /etc/os-release
}

is_freebsd() {
  [ "$(uname -s)" = "FreeBSD" ]
}


# If SELinux is enabled, set the context of the binary
set_selinux_context() {
  # Check if SELinux is enabled and in enforcing or permissive mode
  if command -v getenforce >/dev/null 2>&1; then
    SELINUX_MODE=$(getenforce)
    if [ "$SELINUX_MODE" != "Disabled" ]; then
      echo "SELinux is enabled (${SELINUX_MODE} mode). Setting appropriate context..."

      # First try to set persistent context if semanage is available
      if command -v semanage >/dev/null 2>&1; then
        echo "Attempting to set persistent SELinux context..."
        if semanage fcontext -a -t bin_t "$BIN_PATH" >/dev/null 2>&1; then
          restorecon -v "$BIN_PATH" >/dev/null 2>&1
        else
          echo "Warning: Failed to set persistent context, falling back to temporary context."
        fi
      fi

      # Fall back to chcon if semanage failed or isn't available
      if command -v chcon >/dev/null 2>&1; then
        # Set context for both the directory and binary
        chcon -t bin_t "$BIN_PATH" || echo "Warning: Failed to set SELinux context for binary."
        chcon -R -t bin_t "$AGENT_DIR" || echo "Warning: Failed to set SELinux context for directory."
      else
        if [ "$SELINUX_MODE" = "Enforcing" ]; then
          echo "Warning: SELinux is in enforcing mode but chcon command not found. The service may fail to start."
          echo "Consider installing the policycoreutils package or temporarily setting SELinux to permissive mode."
        else
          echo "Warning: SELinux is in permissive mode but chcon command not found."
        fi
      fi
    fi
  fi
}

# Clean up SELinux contexts if they were set
cleanup_selinux_context() {
  if command -v getenforce >/dev/null 2>&1 && [ "$(getenforce)" != "Disabled" ]; then
    echo "Cleaning up SELinux contexts..."
    # Remove persistent context if semanage is available
    if command -v semanage >/dev/null 2>&1; then
      semanage fcontext -d "$BIN_PATH" 2>/dev/null || true
    fi
  fi
}

# Ensure the proxy URL ends with a /
ensure_trailing_slash() {
  if [ -n "$1" ]; then
    case "$1" in
    */) echo "$1" ;;
    *) echo "$1/" ;;
    esac
  else
    echo "$1"
  fi
}

# Generate FreeBSD rc service content
generate_freebsd_rc_service() {
  cat <<'EOF'
#!/bin/sh

# PROVIDE: aether_agent
# REQUIRE: DAEMON NETWORKING
# BEFORE: LOGIN
# KEYWORD: shutdown

# Add the following lines to /etc/rc.conf to configure Aether Agent:
#
# aether_agent_enable (bool):   Set to YES to enable Aether Agent
#                               Default: YES
# aether_agent_env_file (str):  Aether Agent env configuration file
#                               Default: /usr/local/etc/aether-agent/env
# aether_agent_user (str):      Aether Agent daemon user
#                               Default: aether
# aether_agent_bin (str):       Path to the aether-agent binary
#                               Default: /usr/local/sbin/aether-agent
# aether_agent_flags (str):     Extra flags passed to aether-agent command invocation
#                               Default:

. /etc/rc.subr

name="aether_agent"
rcvar=aether_agent_enable

load_rc_config $name
: ${aether_agent_enable:="YES"}
: ${aether_agent_user:="aether"}
: ${aether_agent_flags:=""}
: ${aether_agent_env_file:="/usr/local/etc/aether-agent/env"}
: ${aether_agent_bin:="/usr/local/sbin/aether-agent"}

logfile="/var/log/${name}.log"
pidfile="/var/run/${name}.pid"

procname="/usr/sbin/daemon"
start_precmd="${name}_prestart"
start_cmd="${name}_start"
stop_cmd="${name}_stop"

extra_commands="upgrade"
upgrade_cmd="aether_agent_upgrade"

aether_agent_prestart()
{
    if [ ! -f "${aether_agent_env_file}" ]; then
        echo WARNING: missing "${aether_agent_env_file}" env file. Start aborted.
        exit 1
    fi

    # Load env file and export variables so the daemon'ed process inherits them.
    # shellcheck disable=SC1090
    set -a
    . "${aether_agent_env_file}"
    set +a
}

aether_agent_start()
{
    echo "Starting ${name}"
    /usr/sbin/daemon -fc \
            -P "${pidfile}" \
            -o "${logfile}" \
            -u "${aether_agent_user}" \
            "${aether_agent_bin}" ${aether_agent_flags}
}

aether_agent_stop()
{
    pid="$(check_pidfile "${pidfile}" "${procname}")"
    if [ -n "${pid}" ]; then
        echo "Stopping ${name} (pid=${pid})"
        kill -- "-${pid}"
        wait_for_pids "${pid}"
    else
        echo "${name} isn't running"
    fi
}

aether_agent_upgrade()
{
    echo "Upgrading ${name}"
    if command -v sudo >/dev/null; then
        sudo -u "${aether_agent_user}" -- "${aether_agent_bin}" update
    else
        su -m "${aether_agent_user}" -c "${aether_agent_bin} update"
    fi
}

run_rc_command "$1"
EOF
}

# Detect system architecture
detect_architecture() {
  local arch=$(uname -m)

  if [ "$arch" = "mips" ]; then
    detect_mips_endianness
    return $?
  fi

  case "$arch" in
    x86_64)
      arch="amd64"
      ;;
    armv6l|armv7l)
      arch="arm"
      ;;
    aarch64)
      arch="arm64"
      ;;
  esac

  echo "$arch"
}

# Detect MIPS endianness using ELF header
detect_mips_endianness() {
  local bins="/bin/sh /bin/ls /usr/bin/env"
  local bin_to_check endian
  
  for bin_to_check in $bins; do
    if [ -f "$bin_to_check" ]; then
      # The 6th byte in ELF header: 01 = little, 02 = big
      endian=$(hexdump -n 1 -s 5 -e '1/1 "%02x"' "$bin_to_check" 2>/dev/null)
      if [ "$endian" = "01" ]; then
        echo "mipsle"
        return
      elif [ "$endian" = "02" ]; then
        echo "mips" 
        return
      fi
    fi
  done
  
  # Final fallback
  echo "mips"
}

# Default values
PORT=45876
UNINSTALL=false
GITHUB_URL="https://github.com"
GITHUB_PROXY_URL=""
KEY=""
TOKEN=""
HUB_URL=""
AUTO_UPDATE_FLAG="" # empty string means prompt, "true" means auto-enable, "false" means skip
VERSION="latest"

# Check for help flag
case "$1" in
-h | --help)
  printf "Aether Agent installation script\n\n"
  printf "Usage: ./install-agent.sh [options]\n\n"
  printf "Options: \n"
  printf "  -k                    : SSH key (required, or interactive if not provided)\n"
  printf "  -p                    : Port (default: $PORT)\n"
  printf "  -t                    : Token (optional for backwards compatibility)\n"
  printf "  -url                  : Hub URL (optional for backwards compatibility)\n"
  printf "  -v, --version         : Version to install (default: latest)\n"
  printf "  -u                    : Uninstall Aether Agent\n"
  printf "  --auto-update [VALUE] : Control automatic daily updates\n"
  printf "                          VALUE can be true (enable) or false (disable). If not specified, will prompt.\n"
  printf "  --mirror [URL]        : (Optional) Use a custom GitHub proxy URL\n"
  printf "  --china-mirrors       : Use built-in China mirror (gh.aether.dev)\n"
  printf "  --china-mirrors [URL] : Use a custom GitHub proxy URL (same as --mirror)\n"
  printf "  -h, --help            : Display this help message\n"
  exit 0
  ;;
esac

# Build sudo args by properly quoting everything
build_sudo_args() {
  QUOTED_ARGS=""
  while [ $# -gt 0 ]; do
    if [ -n "$QUOTED_ARGS" ]; then
      QUOTED_ARGS="$QUOTED_ARGS "
    fi
    QUOTED_ARGS="$QUOTED_ARGS'$(echo "$1" | sed "s/'/'\\\\''/g")'"
    shift
  done
  echo "$QUOTED_ARGS"
}

# Check if running as root and re-execute with sudo if needed
if [ "$(id -u)" != "0" ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO_ARGS=$(build_sudo_args "$@")
    eval "exec sudo $0 $SUDO_ARGS"
  else
    echo "This script must be run as root. Please either:"
    echo "1. Run this script as root (su root)"
    echo "2. Install sudo and run with sudo"
    exit 1
  fi
fi

# Parse arguments
while [ $# -gt 0 ]; do
  case "$1" in
  -k)
    shift
    KEY="$1"
    ;;
  -p)
    shift
    PORT="$1"
    ;;
  -t)
    shift
    TOKEN="$1"
    ;;
  -url)
    shift
    HUB_URL="$1"
    ;;
  -v | --version)
    shift
    VERSION="$1"
    ;;
  -u)
    UNINSTALL=true
    ;;
  --mirror* | --china-mirrors*)
    case "$1" in
    --china-mirrors | --china-mirrors=*)
      # For --china-mirrors with no URL, default to gh.aether.dev (domain replacement mirror).
      if echo "$1" | grep -q "="; then
        CUSTOM_PROXY=$(echo "$1" | cut -d'=' -f2)
        if [ -n "$CUSTOM_PROXY" ]; then
          GITHUB_PROXY_URL="$CUSTOM_PROXY"
          GITHUB_URL="$(ensure_trailing_slash "$CUSTOM_PROXY")https://github.com"
        else
          GITHUB_PROXY_URL="https://gh.aether.dev"
          GITHUB_URL="https://gh.aether.dev"
        fi
      elif [ "$2" != "" ] && ! echo "$2" | grep -q '^-'; then
        # Custom proxy URL provided as next argument (prefix-style, same behavior as --mirror).
        GITHUB_PROXY_URL="$2"
        GITHUB_URL="$(ensure_trailing_slash "$2")https://github.com"
        shift
      else
        GITHUB_PROXY_URL="https://gh.aether.dev"
        GITHUB_URL="https://gh.aether.dev"
      fi
      ;;
    *)
      # --mirror: expects a prefix-style proxy URL like https://ghproxy.com/
      if echo "$1" | grep -q "="; then
        CUSTOM_PROXY=$(echo "$1" | cut -d'=' -f2)
        if [ -n "$CUSTOM_PROXY" ]; then
          GITHUB_PROXY_URL="$CUSTOM_PROXY"
          GITHUB_URL="$(ensure_trailing_slash "$CUSTOM_PROXY")https://github.com"
        else
          echo "No proxy URL provided; using default GitHub"
        fi
      elif [ "$2" != "" ] && ! echo "$2" | grep -q '^-'; then
        GITHUB_PROXY_URL="$2"
        GITHUB_URL="$(ensure_trailing_slash "$2")https://github.com"
        shift
      else
        echo "No proxy URL provided; using default GitHub"
      fi
      ;;
    esac
    ;;
  --auto-update*)
    # Check if there's a value after the = sign
    if echo "$1" | grep -q "="; then
      # Extract the value after =
      AUTO_UPDATE_VALUE=$(echo "$1" | cut -d'=' -f2)
      if [ "$AUTO_UPDATE_VALUE" = "true" ]; then
        AUTO_UPDATE_FLAG="true"
      elif [ "$AUTO_UPDATE_VALUE" = "false" ]; then
        AUTO_UPDATE_FLAG="false"
      else
        echo "Invalid value for --auto-update flag: $AUTO_UPDATE_VALUE. Using default (prompt)."
      fi
    elif [ "$2" = "true" ] || [ "$2" = "false" ]; then
      # Value provided as next argument
      AUTO_UPDATE_FLAG="$2"
      shift
    else
      # No value specified, use true
      AUTO_UPDATE_FLAG="true"
    fi
    ;;
  *)
    echo "Invalid option: $1" >&2
    exit 1
    ;;
  esac
  shift
done

# Set paths based on operating system
if is_freebsd; then
  AGENT_DIR="/usr/local/etc/aether-agent"
  BIN_DIR="/usr/local/sbin"
  BIN_PATH="/usr/local/sbin/aether-agent"
else
  AGENT_DIR="/opt/aether-agent"
  BIN_DIR="/opt/aether-agent"
  BIN_PATH="/opt/aether-agent/aether-agent"
fi

# Uninstall process
if [ "$UNINSTALL" = true ]; then
  # Clean up SELinux contexts before removing files
  cleanup_selinux_context

  if is_alpine; then
    echo "Stopping and disabling the agent service..."
    rc-service aether-agent stop
    rc-update del aether-agent default

    echo "Removing the OpenRC service files..."
    rm -f /etc/init.d/aether-agent

    # Remove the daily update cron job if it exists
    echo "Removing the daily update cron job..."
    if crontab -u root -l 2>/dev/null | grep -q "aether-agent.*update"; then
      crontab -u root -l 2>/dev/null | grep -v "aether-agent.*update" | crontab -u root -
    fi

    # Remove log files
    echo "Removing log files..."
    rm -f /var/log/aether-agent.log /var/log/aether-agent.err
  elif is_openwrt; then
    echo "Stopping and disabling the agent service..."
    /etc/init.d/aether-agent stop
    /etc/init.d/aether-agent disable

    echo "Removing the OpenWRT service files..."
    rm -f /etc/init.d/aether-agent

    # Remove the update service if it exists
    echo "Removing the daily update service..."
    # Remove legacy aether account based crontab file
    rm -f /etc/crontabs/aether
    # Install root crontab job
    if crontab -u root -l 2>/dev/null | grep -q "aether-agent.*update"; then
      crontab -u root -l 2>/dev/null | grep -v "aether-agent.*update" | crontab -u root -
    fi

  elif is_freebsd; then
    echo "Stopping and disabling the agent service..."
    service aether-agent stop
    sysrc aether_agent_enable="NO"

    echo "Removing the FreeBSD service files..."
    rm -f /usr/local/etc/rc.d/aether-agent

    # Remove the daily update cron job if it exists
    echo "Removing the daily update cron job..."
    rm -f /etc/cron.d/aether-agent

    # Remove log files
    echo "Removing log files..."
    rm -f /var/log/aether_agent.log

    # Remove env file and directories
    echo "Removing environment configuration file..."
    rm -f "$AGENT_DIR/env"
    rm -f "$BIN_PATH"
    rmdir "$AGENT_DIR" 2>/dev/null || true

  else
    echo "Stopping and disabling the agent service..."
    systemctl stop aether-agent.service
    systemctl disable aether-agent.service

    echo "Removing the systemd service file..."
    rm /etc/systemd/system/aether-agent.service

    # Remove the update timer and service if they exist
    echo "Removing the daily update service and timer..."
    systemctl stop aether-agent-update.timer 2>/dev/null
    systemctl disable aether-agent-update.timer 2>/dev/null
    rm -f /etc/systemd/system/aether-agent-update.service
    rm -f /etc/systemd/system/aether-agent-update.timer

    systemctl daemon-reload
  fi

  echo "Removing the Aether Agent directory..."
  rm -rf "$AGENT_DIR"

  echo "Removing the dedicated user for the agent service..."
  killall aether-agent 2>/dev/null
  if is_alpine || is_openwrt; then
    deluser aether 2>/dev/null
  elif is_freebsd; then
    pw user del aether 2>/dev/null
  else
    userdel aether 2>/dev/null
  fi

  echo "Aether Agent has been uninstalled successfully!"
  exit 0
fi

# Check if a package is installed
package_installed() {
  command -v "$1" >/dev/null 2>&1
}

# Check for package manager and install necessary packages if not installed
if package_installed apk; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    apk update
    apk add tar curl coreutils shadow
  fi
elif package_installed opkg; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    opkg update
    opkg install tar curl coreutils
  fi
elif package_installed pkg && is_freebsd; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    pkg update
    pkg install -y gtar curl coreutils
  fi
elif package_installed apt-get; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    apt-get update
    apt-get install -y tar curl coreutils
  fi
elif package_installed yum; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    yum install -y tar curl coreutils
  fi
elif package_installed pacman; then
  if ! package_installed tar || ! package_installed curl || ! package_installed sha256sum; then
    pacman -Sy --noconfirm tar curl coreutils
  fi
else
  echo "Warning: Please ensure 'tar' and 'curl' and 'sha256sum (coreutils)' are installed."
fi

# If no SSH key is provided, ask for the SSH key interactively
if [ -z "$KEY" ]; then
  printf "Enter your SSH key: "
  read KEY
fi

# Remove newlines from KEY
KEY=$(echo "$KEY" | tr -d '\n')

# TOKEN and HUB_URL are optional for backwards compatibility - no interactive prompts
# They will be set as empty environment variables if not provided

# Verify checksum
if command -v sha256sum >/dev/null; then
  CHECK_CMD="sha256sum"
elif command -v sha256 >/dev/null; then
  # FreeBSD uses 'sha256' instead of 'sha256sum', with different output format
  CHECK_CMD="sha256 -q"
else
  echo "No SHA256 checksum utility found"
  exit 1
fi

# Create a dedicated user for the service if it doesn't exist
echo "Creating a dedicated user for the Aether Agent service..."
if is_alpine; then
  if ! id -u aether >/dev/null 2>&1; then
    addgroup aether
    adduser -S -D -H -s /sbin/nologin -G aether aether
  fi
  # Add the user to the docker group to allow access to the Docker socket if group docker exists
  if getent group docker; then
    echo "Adding aether to docker group"
    usermod -aG docker aether
  fi
  
elif is_openwrt; then
  # Create aether group first if it doesn't exist (check /etc/group directly)
  if ! grep -q "^aether:" /etc/group >/dev/null 2>&1; then
    echo "aether:x:999:" >> /etc/group
  fi
  
  # Create aether user if it doesn't exist (double-check to prevent duplicates)
  if ! id -u aether >/dev/null 2>&1 && ! grep -q "^aether:" /etc/passwd >/dev/null 2>&1; then
    echo "aether:x:999:999::/nonexistent:/bin/false" >> /etc/passwd
  fi
  
  # Add the user to the docker group if docker group exists and user is not already in it
  if grep -q "^docker:" /etc/group >/dev/null 2>&1; then
    echo "Adding aether to docker group"
    # Check if aether is already in docker group
    if ! grep "^docker:" /etc/group | grep -q "aether"; then
      # Add aether to docker group by modifying /etc/group
      # Handle both cases: group with existing members and group without members
      if grep "^docker:" /etc/group | grep -q ":.*:.*$"; then
        # Group has existing members, append with comma
        sed -i 's/^docker:\([^:]*:[^:]*:\)\(.*\)$/docker:\1\2,aether/' /etc/group
      else
        # Group has no members, just append
        sed -i 's/^docker:\([^:]*:[^:]*:\)$/docker:\1aether/' /etc/group
      fi
    fi
  fi

elif is_freebsd; then
  if ! id -u aether >/dev/null 2>&1; then
    pw user add aether -d /nonexistent -s /usr/sbin/nologin -c "aether user"
  fi
  # Add the user to the wheel group to allow self-updates
  if pw group show wheel >/dev/null 2>&1; then
    echo "Adding aether to wheel group for self-updates"
    pw group mod wheel -m aether
  fi

else
  if ! id -u aether >/dev/null 2>&1; then
    useradd --system --home-dir /nonexistent --shell /bin/false aether
  fi
  # Add the user to the docker group to allow access to the Docker socket if group docker exists
  if getent group docker; then
    echo "Adding aether to docker group"
    usermod -aG docker aether
  fi
  # Add the user to the disk group to allow access to disk devices if group disk exists
  if getent group disk; then
    echo "Adding aether to disk group"
    usermod -aG disk aether
  fi
fi

# Create the directory for the Aether Agent

if [ ! -d "$AGENT_DIR" ]; then
  echo "Creating the directory for the Aether Agent..."
  mkdir -p "$AGENT_DIR"
  chown aether:aether "$AGENT_DIR"
  chmod 755 "$AGENT_DIR"
fi

if [ ! -d "$BIN_DIR" ]; then
  mkdir -p "$BIN_DIR"
fi

# Download and install the Aether Agent
echo "Downloading and installing the agent..."

OS=$(uname -s | sed -e 'y/ABCDEFGHIJKLMNOPQRSTUVWXYZ/abcdefghijklmnopqrstuvwxyz/')
ARCH=$(detect_architecture)
FILE_NAME="aether-agent_${OS}_${ARCH}.tar.gz"

# Determine version to install
if [ "$VERSION" = "latest" ]; then
  API_RELEASE_URL="https://api.github.com/repos/LuBoyanzy/Aether/releases/latest"
  INSTALL_VERSION=$(curl -s "$API_RELEASE_URL" | grep -o '"tag_name": "v[^"]*"' | cut -d'"' -f4 | tr -d 'v')
  if [ -z "$INSTALL_VERSION" ]; then
    echo "Failed to get latest version from GitHub"
    exit 1
  fi
else
  INSTALL_VERSION="$VERSION"
  # Remove 'v' prefix if present
  INSTALL_VERSION=$(echo "$INSTALL_VERSION" | sed 's/^v//')
fi

echo "Downloading and installing agent version ${INSTALL_VERSION} from ${GITHUB_URL} ..."

# Download checksums file
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR" || exit 1
CHECKSUM=$(curl -sL "$GITHUB_URL/LuBoyanzy/Aether/releases/download/v${INSTALL_VERSION}/aether_${INSTALL_VERSION}_checksums.txt" | grep "$FILE_NAME" | cut -d' ' -f1)
if [ -z "$CHECKSUM" ] || ! echo "$CHECKSUM" | grep -qE "^[a-fA-F0-9]{64}$"; then
  echo "Failed to get checksum or invalid checksum format"
  exit 1
fi

if ! curl -#L "$GITHUB_URL/LuBoyanzy/Aether/releases/download/v${INSTALL_VERSION}/$FILE_NAME" -o "$FILE_NAME"; then
  echo "Failed to download the agent from ""$GITHUB_URL/LuBoyanzy/Aether/releases/download/v${INSTALL_VERSION}/$FILE_NAME"
  rm -rf "$TEMP_DIR"
  exit 1
fi

if [ "$($CHECK_CMD "$FILE_NAME" | cut -d' ' -f1)" != "$CHECKSUM" ]; then
  echo "Checksum verification failed: $($CHECK_CMD "$FILE_NAME" | cut -d' ' -f1) & $CHECKSUM"
  rm -rf "$TEMP_DIR"
  exit 1
fi

if ! tar -xzf "$FILE_NAME" aether-agent; then
  echo "Failed to extract the agent"
  rm -rf "$TEMP_DIR"
  exit 1
fi

mv aether-agent "$BIN_PATH"
chown aether:aether "$BIN_PATH"
chmod 755 "$BIN_PATH"

# Set SELinux context if needed
set_selinux_context

# Cleanup
rm -rf "$TEMP_DIR"

# Make sure /etc/machine-id exists for persistent fingerprint
if [ ! -f /etc/machine-id ]; then
  if [ -r /proc/sys/kernel/random/uuid ]; then
    cat /proc/sys/kernel/random/uuid | tr -d '-' > /etc/machine-id
  elif command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr -d '-' > /etc/machine-id
  else
    echo "Warning: Could not create /etc/machine-id (no /proc uuid and no uuidgen found)."
  fi
fi

# Check for NVIDIA GPUs and grant device permissions for systemd service
detect_nvidia_devices() {
  local devices=""
  for i in /dev/nvidia*; do
    if [ -e "$i" ]; then
      devices="${devices}DeviceAllow=$i rw\n"
    fi
  done
  echo "$devices"
}

# Modify service installation part, add Alpine check before systemd service creation
if is_alpine; then
  echo "Creating OpenRC service for Alpine Linux..."
  cat >/etc/init.d/aether-agent <<EOF
#!/sbin/openrc-run

name="aether-agent"
description="Aether Agent Service"
command="$BIN_PATH"
command_user="aether"
command_background="yes"
pidfile="/run/\${RC_SVCNAME}.pid"
output_log="/var/log/aether-agent.log"
error_log="/var/log/aether-agent.err"

start_pre() {
    checkpath -f -m 0644 -o aether:aether "\$output_log" "\$error_log"
}

export PORT="$PORT"
export KEY="$KEY"
export TOKEN="$TOKEN"
export HUB_URL="$HUB_URL"

depend() {
    need net
    after firewall
}
EOF

  chmod +x /etc/init.d/aether-agent
  rc-update add aether-agent default

  # Create log files with proper permissions
  touch /var/log/aether-agent.log /var/log/aether-agent.err
  chown aether:aether /var/log/aether-agent.log /var/log/aether-agent.err

  # Start the service
  rc-service aether-agent restart

  # Check if service started successfully
  sleep 2
  if ! rc-service aether-agent status | grep -q "started"; then
    echo "Error: The Aether Agent service failed to start. Checking logs..."
    tail -n 20 /var/log/aether-agent.err
    exit 1
  fi

  # Auto-update service for Alpine
  if [ "$AUTO_UPDATE_FLAG" = "true" ]; then
    AUTO_UPDATE="y"
  elif [ "$AUTO_UPDATE_FLAG" = "false" ]; then
    AUTO_UPDATE="n"
  else
    printf "\nEnable automatic daily updates for aether-agent? (y/n): "
    read AUTO_UPDATE
  fi
  case "$AUTO_UPDATE" in
  [Yy]*)
    echo "Setting up daily automatic updates for aether-agent..."

    # Create cron job to run aether-agent update command daily at midnight
    if ! crontab -u root -l 2>/dev/null | grep -q "aether-agent.*update"; then
      (crontab -u root -l 2>/dev/null; echo "12 0 * * * $BIN_PATH update >/dev/null 2>&1") | crontab -u root -
    fi

    printf "\nDaily updates have been enabled via cron job.\n"
    ;;
  esac

  # Check service status
  if ! rc-service aether-agent status >/dev/null 2>&1; then
    echo "Error: The Aether Agent service is not running."
    rc-service aether-agent status
    exit 1
  fi

elif is_openwrt; then
  echo "Creating procd init script service for OpenWRT..."
  cat >/etc/init.d/aether-agent <<EOF
#!/bin/sh /etc/rc.common

USE_PROCD=1
START=99

start_service() {
    procd_open_instance
    procd_set_param command $BIN_PATH
    procd_set_param user aether
    procd_set_param pidfile /var/run/aether-agent.pid
    procd_set_param env PORT="$PORT" KEY="$KEY" TOKEN="$TOKEN" HUB_URL="$HUB_URL"
    procd_set_param respawn
    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}

# Extra command to trigger agent update
EXTRA_COMMANDS="update restart"
EXTRA_HELP="        update          Update the Aether agent
        restart         Restart the Aether agent"

update() {
    $BIN_PATH update
}

EOF

  # Enable the service
  chmod +x /etc/init.d/aether-agent
  /etc/init.d/aether-agent enable

  # Start the service
  /etc/init.d/aether-agent restart

  # Auto-update service for OpenWRT using a crontab job
  if [ "$AUTO_UPDATE_FLAG" = "true" ]; then
    AUTO_UPDATE="y"
    sleep 1 # give time for the service to start
  elif [ "$AUTO_UPDATE_FLAG" = "false" ]; then
    AUTO_UPDATE="n"
    sleep 1 # give time for the service to start
  else
    printf "\nEnable automatic daily updates for aether-agent? (y/n): "
    read AUTO_UPDATE
  fi
  case "$AUTO_UPDATE" in
  [Yy]*)
    echo "Setting up daily automatic updates for aether-agent..."

    if ! crontab -u root -l 2>/dev/null | grep -q "aether-agent.*update"; then
      (crontab -u root -l 2>/dev/null; echo "12 0 * * * /etc/init.d/aether-agent update") | crontab -u root -
    fi

    /etc/init.d/cron restart

    printf "\nDaily updates have been enabled.\n"
    ;;
  esac

  # Check service status
  if ! /etc/init.d/aether-agent running >/dev/null 2>&1; then
    echo "Error: The Aether Agent service is not running."
    /etc/init.d/aether-agent status
    exit 1
  fi

elif is_freebsd; then
  echo "Creating FreeBSD rc service..."
  
  # Create environment configuration file with proper permissions
  echo "Creating environment configuration file..."
  cat >"$AGENT_DIR/env" <<EOF
LISTEN="$PORT"
KEY="$KEY"
TOKEN="$TOKEN"
HUB_URL="$HUB_URL"
EOF
  chmod 640 "$AGENT_DIR/env"
  chown root:aether "$AGENT_DIR/env"
  
  # Create the rc service file
  generate_freebsd_rc_service > /usr/local/etc/rc.d/aether-agent

  # Set proper permissions for the rc script
  chmod 755 /usr/local/etc/rc.d/aether-agent
  
  # Enable and start the service
  echo "Enabling and starting the agent service..."
  sysrc aether_agent_enable="YES"
  service aether-agent restart
  
  # Check if service started successfully
  sleep 2
  if ! service aether-agent status | grep -q "is running"; then
    echo "Error: The Aether Agent service failed to start. Checking logs..."
    tail -n 20 /var/log/aether_agent.log
    exit 1
  fi

  # Auto-update service for FreeBSD
  if [ "$AUTO_UPDATE_FLAG" = "true" ]; then
    AUTO_UPDATE="y"
  elif [ "$AUTO_UPDATE_FLAG" = "false" ]; then
    AUTO_UPDATE="n"
  else
    printf "\nEnable automatic daily updates for aether-agent? (y/n): "
    read AUTO_UPDATE
  fi
  case "$AUTO_UPDATE" in
  [Yy]*)
    echo "Setting up daily automatic updates for aether-agent..."

    # Create cron job in /etc/cron.d 
    cat >/etc/cron.d/aether-agent <<EOF
# Aether Agent daily update job
12 0 * * * root $BIN_PATH update >/dev/null 2>&1
EOF
    chmod 644 /etc/cron.d/aether-agent
    printf "\nDaily updates have been enabled via /etc/cron.d.\n"
    ;;
  esac

  # Check service status
  if ! service aether-agent status >/dev/null 2>&1; then
    echo "Error: The Aether Agent service is not running."
    service aether-agent status
    exit 1
  fi

else
  # Original systemd service installation code
  echo "Creating the systemd service for the agent..."

  # Detect NVIDIA devices and grant device permissions
  NVIDIA_DEVICES=$(detect_nvidia_devices)

  cat >/etc/systemd/system/aether-agent.service <<EOF
[Unit]
Description=Aether Agent Service
Wants=network-online.target
After=network-online.target

[Service]
Environment="PORT=$PORT"
Environment="KEY=$KEY"
Environment="TOKEN=$TOKEN"
Environment="HUB_URL=$HUB_URL"
# Environment="EXTRA_FILESYSTEMS=sdb"
ExecStart=$BIN_PATH
User=aether
Restart=on-failure
RestartSec=5
StateDirectory=aether-agent

# Security/sandboxing settings
KeyringMode=private
LockPersonality=yes
ProtectClock=yes
ProtectHome=read-only
ProtectHostname=yes
ProtectKernelLogs=yes
ProtectSystem=strict
RemoveIPC=yes
RestrictSUIDSGID=true

$(if [ -n "$NVIDIA_DEVICES" ]; then printf "%b" "# NVIDIA device permissions\n${NVIDIA_DEVICES}"; fi)

[Install]
WantedBy=multi-user.target
EOF

  # Load and start the service
  printf "\nLoading and starting the agent service...\n"
  systemctl daemon-reload
  systemctl enable aether-agent.service
  systemctl start aether-agent.service



  # Prompt for auto-update setup
  if [ "$AUTO_UPDATE_FLAG" = "true" ]; then
    AUTO_UPDATE="y"
    sleep 1 # give time for the service to start
  elif [ "$AUTO_UPDATE_FLAG" = "false" ]; then
    AUTO_UPDATE="n"
    sleep 1 # give time for the service to start
  else
    printf "\nEnable automatic daily updates for aether-agent? (y/n): "
    read AUTO_UPDATE
  fi
  case "$AUTO_UPDATE" in
  [Yy]*)
    echo "Setting up daily automatic updates for aether-agent..."

    # Create systemd service for the daily update
    cat >/etc/systemd/system/aether-agent-update.service <<EOF
[Unit]
Description=Update aether-agent if needed
Wants=aether-agent.service

[Service]
Type=oneshot
ExecStart=$BIN_PATH update
EOF

    # Create systemd timer for the daily update
    cat >/etc/systemd/system/aether-agent-update.timer <<EOF
[Unit]
Description=Run aether-agent update daily

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=4h

[Install]
WantedBy=timers.target
EOF

    systemctl daemon-reload
    systemctl enable --now aether-agent-update.timer

    printf "\nDaily updates have been enabled.\n"
    ;;
  esac

  # Wait for the service to start or fail
  if [ "$(systemctl is-active aether-agent.service)" != "active" ]; then
    echo "Error: The Aether Agent service is not running."
    echo "$(systemctl status aether-agent.service)"
    exit 1
  fi
fi

printf "\n\033[32mAether Agent has been installed successfully! It is now running on $PORT.\033[0m\n"
