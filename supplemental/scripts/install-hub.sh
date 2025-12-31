#!/bin/sh

is_freebsd() {
  [ "$(uname -s)" = "FreeBSD" ]
}

# Function to ensure the proxy URL ends with a /
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

# PROVIDE: aether_hub
# REQUIRE: DAEMON NETWORKING
# BEFORE: LOGIN
# KEYWORD: shutdown

# Add the following lines to /etc/rc.conf to configure Aether Hub:
#
# aether_hub_enable (bool):   Set to YES to enable Aether Hub
#                             Default: YES
# aether_hub_port (str):      Port to listen on
#                             Default: 19090
# aether_hub_user (str):      Aether Hub daemon user
#                             Default: aether
# aether_hub_bin (str):       Path to the aether binary
#                             Default: /usr/local/sbin/aether
# aether_hub_data (str):      Path to the aether data directory
#                             Default: /usr/local/etc/aether/aether_data
# aether_hub_flags (str):     Extra flags passed to aether command invocation
#                             Default:

. /etc/rc.subr

name="aether_hub"
rcvar=aether_hub_enable

load_rc_config $name
: ${aether_hub_enable:="YES"}
: ${aether_hub_port:="19090"}
: ${aether_hub_user:="aether"}
: ${aether_hub_flags:=""}
: ${aether_hub_bin:="/usr/local/sbin/aether"}
: ${aether_hub_data:="/usr/local/etc/aether/aether_data"}

logfile="/var/log/${name}.log"
pidfile="/var/run/${name}.pid"

procname="/usr/sbin/daemon"
start_precmd="${name}_prestart"
start_cmd="${name}_start"
stop_cmd="${name}_stop"

extra_commands="upgrade"
upgrade_cmd="aether_hub_upgrade"

aether_hub_prestart()
{
    if [ ! -d "${aether_hub_data}" ]; then
        echo "Creating data directory ${aether_hub_data}"
        mkdir -p "${aether_hub_data}"
        chown "${aether_hub_user}:${aether_hub_user}" "${aether_hub_data}"
    fi
}

aether_hub_start()
{
    echo "Starting ${name}"
    cd "$(dirname "${aether_hub_data}")" || exit 1
    /usr/sbin/daemon -f \
            -P "${pidfile}" \
            -o "${logfile}" \
            -u "${aether_hub_user}" \
            "${aether_hub_bin}" serve --http "0.0.0.0:${aether_hub_port}" ${aether_hub_flags}
}

aether_hub_stop()
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

aether_hub_upgrade()
{
    echo "Upgrading ${name}"
    if command -v sudo >/dev/null; then
        sudo -u "${aether_hub_user}" -- "${aether_hub_bin}" update
    else
        su -m "${aether_hub_user}" -c "${aether_hub_bin} update"
    fi
}

run_rc_command "$1"
EOF
}

# Detect system architecture
detect_architecture() {
  arch=$(uname -m)
  case "$arch" in
    x86_64)
      arch="amd64"
      ;;
    armv7l)
      arch="arm"
      ;;
    aarch64)
      arch="arm64"
      ;;
  esac
  echo "$arch"
}

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

# Define default values
PORT=19090
GITHUB_PROXY_URL=""
AUTO_UPDATE_FLAG="false"
UNINSTALL=false

# Parse command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    -u)
      UNINSTALL=true
      shift
      ;;
    -h|--help)
      printf "Aether Hub installation script\n\n"
      printf "Usage: ./install-hub.sh [options]\n\n"
      printf "Options: \n"
      printf "  -u           : Uninstall the Aether Hub\n"
      printf "  -p <port>    : Specify a port number (default: 19090)\n"
      printf "  -c <url>     : (Optional) Use a custom GitHub mirror URL\n"
      printf "  --auto-update : Enable automatic daily updates (disabled by default)\n"
      printf "  -h, --help   : Display this help message\n"
      exit 0
      ;;
    -p)
      shift
      PORT="$1"
      shift
      ;;
    -c)
      shift
      GITHUB_PROXY_URL=$(ensure_trailing_slash "$1")
      shift
      ;;
    --auto-update)
      AUTO_UPDATE_FLAG="true"
      shift
      ;;
    *)
      echo "Invalid option: $1" >&2
      exit 1
      ;;
  esac
done

# Ensure the proxy URL ends with a /
GITHUB_PROXY_URL=$(ensure_trailing_slash "$GITHUB_PROXY_URL")

# Set paths based on operating system
if is_freebsd; then
  HUB_DIR="/usr/local/etc/aether"
  BIN_PATH="/usr/local/sbin/aether"
else
  HUB_DIR="/opt/aether"
  BIN_PATH="/opt/aether/aether"
fi

# Uninstall process
if [ "$UNINSTALL" = true ]; then
  if is_freebsd; then
    echo "Stopping and disabling the Aether Hub service..."
    service aether-hub stop 2>/dev/null
    sysrc aether_hub_enable="NO" 2>/dev/null

    echo "Removing the FreeBSD service files..."
    rm -f /usr/local/etc/rc.d/aether-hub

    echo "Removing the daily update cron job..."
    rm -f /etc/cron.d/aether-hub

    echo "Removing log files..."
    rm -f /var/log/aether_hub.log

    echo "Removing the Aether Hub binary and data..."
    rm -f "$BIN_PATH"
    rm -rf "$HUB_DIR"

    echo "Removing the dedicated user..."
    pw user del aether 2>/dev/null

    echo "The Aether Hub has been uninstalled successfully!"
    exit 0
  else
    # Stop and disable the Aether Hub service
    echo "Stopping and disabling the Aether Hub service..."
    systemctl stop aether-hub.service
    systemctl disable aether-hub.service

    # Remove the systemd service file
    echo "Removing the systemd service file..."
    rm -f /etc/systemd/system/aether-hub.service

    # Remove the update timer and service if they exist
    echo "Removing the daily update service and timer..."
    systemctl stop aether-hub-update.timer 2>/dev/null
    systemctl disable aether-hub-update.timer 2>/dev/null
    rm -f /etc/systemd/system/aether-hub-update.service
    rm -f /etc/systemd/system/aether-hub-update.timer

    # Reload the systemd daemon
    echo "Reloading the systemd daemon..."
    systemctl daemon-reload

    # Remove the Aether Hub binary and data
    echo "Removing the Aether Hub binary and data..."
    rm -rf "$HUB_DIR"

    # Remove the dedicated user
    echo "Removing the dedicated user..."
    userdel aether 2>/dev/null

    echo "The Aether Hub has been uninstalled successfully!"
    exit 0
  fi
fi

# Function to check if a package is installed
package_installed() {
  command -v "$1" >/dev/null 2>&1
}

# Check for package manager and install necessary packages if not installed
if package_installed pkg && is_freebsd; then
  if ! package_installed tar || ! package_installed curl; then
    pkg update
    pkg install -y gtar curl
  fi
elif package_installed apt-get; then
  if ! package_installed tar || ! package_installed curl; then
    apt-get update
    apt-get install -y tar curl
  fi
elif package_installed yum; then
  if ! package_installed tar || ! package_installed curl; then
    yum install -y tar curl
  fi
elif package_installed pacman; then
  if ! package_installed tar || ! package_installed curl; then
    pacman -Sy --noconfirm tar curl
  fi
else
  echo "Warning: Please ensure 'tar' and 'curl' are installed."
fi

# Create a dedicated user for the service if it doesn't exist
echo "Creating a dedicated user for the Aether Hub service..."
if is_freebsd; then
  if ! id -u aether >/dev/null 2>&1; then
    pw user add aether -d /nonexistent -s /usr/sbin/nologin -c "aether user"
  fi
else
  if ! id -u aether >/dev/null 2>&1; then
    useradd -M -s /bin/false aether
  fi
fi

# Create the directory for the Aether Hub
echo "Creating the directory for the Aether Hub..."
mkdir -p "$HUB_DIR/aether_data"
chown -R aether:aether "$HUB_DIR"
chmod 755 "$HUB_DIR"

# Download and install the Aether Hub
echo "Downloading and installing the Aether Hub..."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(detect_architecture)
FILE_NAME="aether_${OS}_${ARCH}.tar.gz"

curl -sL "${GITHUB_PROXY_URL}https://github.com/LuBoyanzy/Aether/releases/latest/download/$FILE_NAME" | tar -xz -O aether | tee ./aether >/dev/null
chmod +x ./aether
mv ./aether "$BIN_PATH"
chown aether:aether "$BIN_PATH"

if is_freebsd; then
  echo "Creating FreeBSD rc service..."

  # Create the rc service file
  generate_freebsd_rc_service > /usr/local/etc/rc.d/aether-hub

  # Set proper permissions for the rc script
  chmod 755 /usr/local/etc/rc.d/aether-hub

  # Configure the port
  sysrc aether_hub_port="$PORT"

  # Enable and start the service
  echo "Enabling and starting the Aether Hub service..."
  sysrc aether_hub_enable="YES"
  service aether-hub restart

  # Check if service started successfully
  sleep 2
  if ! service aether-hub status | grep -q "is running"; then
    echo "Error: The Aether Hub service failed to start. Checking logs..."
    tail -n 20 /var/log/aether_hub.log
    exit 1
  fi

  # Auto-update service for FreeBSD
  if [ "$AUTO_UPDATE_FLAG" = "true" ]; then
    echo "Setting up daily automatic updates for aether-hub..."

    # Create cron job in /etc/cron.d
    cat >/etc/cron.d/aether-hub <<EOF
# Aether Hub daily update job
12 8 * * * root $BIN_PATH update >/dev/null 2>&1
EOF
    chmod 644 /etc/cron.d/aether-hub
    printf "\nDaily updates have been enabled via /etc/cron.d.\n"
  fi

  # Check service status
  if ! service aether-hub status >/dev/null 2>&1; then
    echo "Error: The Aether Hub service is not running."
    service aether-hub status
    exit 1
  fi

else
  # Original systemd service installation code
  printf "Creating the systemd service for the Aether Hub...\n\n"
  tee /etc/systemd/system/aether-hub.service <<EOF
[Unit]
Description=Aether Hub Service
After=network.target

[Service]
ExecStart=$BIN_PATH serve --http "0.0.0.0:$PORT"
WorkingDirectory=$HUB_DIR
User=aether
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  # Load and start the service
  printf "\nLoading and starting the Aether Hub service...\n"
  systemctl daemon-reload
  systemctl enable aether-hub.service
  systemctl start aether-hub.service

  # Wait for the service to start or fail
  sleep 2

  # Check if the service is running
  if [ "$(systemctl is-active aether-hub.service)" != "active" ]; then
    echo "Error: The Aether Hub service is not running."
    echo "$(systemctl status aether-hub.service)"
    exit 1
  fi

  # Enable auto-update if flag is set to true
  if [ "$AUTO_UPDATE_FLAG" = "true" ]; then
    echo "Setting up daily automatic updates for aether-hub..."

    # Create systemd service for the daily update
    cat >/etc/systemd/system/aether-hub-update.service <<EOF
[Unit]
Description=Update aether-hub if needed
Wants=aether-hub.service

[Service]
Type=oneshot
ExecStart=$BIN_PATH update
EOF

    # Create systemd timer for the daily update
    cat >/etc/systemd/system/aether-hub-update.timer <<EOF
[Unit]
Description=Run aether-hub update daily

[Timer]
OnCalendar=daily
Persistent=true
RandomizedDelaySec=4h

[Install]
WantedBy=timers.target
EOF

    systemctl daemon-reload
    systemctl enable --now aether-hub-update.timer

    printf "\nDaily updates have been enabled.\n"
  fi
fi

echo "The Aether Hub has been installed and configured successfully! It is now accessible on port $PORT."
