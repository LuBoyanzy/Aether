#!/bin/bash

PORT=45876
KEY=""
TOKEN=""
HUB_URL=""
REPO_OWNER="LuBoyanzy"
REPO_NAME="Aether"
GITHUB_URL="https://github.com"

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

usage() {
  printf "Aether Agent macOS installation script (direct GitHub download)\n\n"
  printf "Usage: ./install-agent-brew.sh [options]\n\n"
  printf "Options: \n"
  printf "  -k            SSH key (required, or interactive if not provided)\n"
  printf "  -p            Port (default: $PORT)\n"
  printf "  -t            Token (optional)\n"
  printf "  -url          Hub URL (optional)\n"
  printf "  --mirror [URL]        : (Optional) Use a custom GitHub proxy URL\n"
  printf "  --china-mirrors       : Use built-in China mirror (gh.aether.dev)\n"
  printf "  --china-mirrors [URL] : Use a custom GitHub proxy URL (same as --mirror)\n"
  printf "  -h, --help    Display this help message\n"
  exit 0
}

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
  --mirror* | --china-mirrors*)
    case "$1" in
    --china-mirrors | --china-mirrors=*)
      if echo "$1" | grep -q "="; then
        CUSTOM_PROXY=$(echo "$1" | cut -d'=' -f2)
        if [ -n "$CUSTOM_PROXY" ]; then
          GITHUB_URL="$(ensure_trailing_slash "$CUSTOM_PROXY")https://github.com"
        else
          GITHUB_URL="https://gh.aether.dev"
        fi
      elif [ "$2" != "" ] && ! echo "$2" | grep -q '^-'; then
        GITHUB_URL="$(ensure_trailing_slash "$2")https://github.com"
        shift
      else
        GITHUB_URL="https://gh.aether.dev"
      fi
      ;;
    *)
      if echo "$1" | grep -q "="; then
        CUSTOM_PROXY=$(echo "$1" | cut -d'=' -f2)
        if [ -n "$CUSTOM_PROXY" ]; then
          GITHUB_URL="$(ensure_trailing_slash "$CUSTOM_PROXY")https://github.com"
        else
          echo "No proxy URL provided; using default GitHub"
        fi
      elif [ "$2" != "" ] && ! echo "$2" | grep -q '^-'; then
        GITHUB_URL="$(ensure_trailing_slash "$2")https://github.com"
        shift
      else
        echo "No proxy URL provided; using default GitHub"
      fi
      ;;
    esac
    ;;
  -h | --help)
    usage
    ;;
  *)
    echo "Invalid option: $1" >&2
    usage
    ;;
  esac
  shift
done

if [ "$(id -u)" -eq 0 ]; then
  echo "Please run this script as a regular user (no sudo)."
  echo "It will install to ~/.local/bin if /usr/local/bin isn't writable, and set up a user LaunchAgent."
  exit 1
fi

if [ -z "$KEY" ]; then
  read -p "Enter SSH key: " KEY
fi

# TOKEN and HUB_URL are optional for backwards compatibility - no interactive prompts

mkdir -p ~/.config/aether ~/.cache/aether

echo "KEY=\"$KEY\"" >~/.config/aether/aether-agent.env
echo "LISTEN=$PORT" >>~/.config/aether/aether-agent.env

if [ -n "$TOKEN" ]; then
  echo "TOKEN=\"$TOKEN\"" >>~/.config/aether/aether-agent.env
fi
if [ -n "$HUB_URL" ]; then
  echo "HUB_URL=\"$HUB_URL\"" >>~/.config/aether/aether-agent.env
fi

# Determine architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
else
  ARCH="amd64"
fi
OS="darwin"
FILE_NAME="aether-agent_${OS}_${ARCH}.tar.gz"

API_RELEASE_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
INSTALL_VERSION=$(curl -s "$API_RELEASE_URL" | grep -o '"tag_name": "v[^"]*"' | cut -d'"' -f4 | tr -d 'v')
if [ -z "$INSTALL_VERSION" ]; then
  echo "Failed to get latest version from GitHub"
  exit 1
fi

CHECKSUM=$(curl -sL "$GITHUB_URL/${REPO_OWNER}/${REPO_NAME}/releases/download/v${INSTALL_VERSION}/aether_${INSTALL_VERSION}_checksums.txt" | grep "$FILE_NAME" | cut -d' ' -f1)
if [ -z "$CHECKSUM" ]; then
  echo "Failed to fetch checksum"
  exit 1
fi

TMP_DIR=$(mktemp -d)
cd "$TMP_DIR" || exit 1
curl -#L "$GITHUB_URL/${REPO_OWNER}/${REPO_NAME}/releases/download/v${INSTALL_VERSION}/${FILE_NAME}" -o "$FILE_NAME"
if [ $? -ne 0 ]; then
  echo "Failed to download archive from $GITHUB_URL"
  exit 1
fi

DOWNLOAD_SUM=$(shasum -a 256 "$FILE_NAME" | cut -d' ' -f1)
if [ "$DOWNLOAD_SUM" != "$CHECKSUM" ]; then
  echo "Checksum mismatch: expected $CHECKSUM got $DOWNLOAD_SUM"
  exit 1
fi

tar -xzf "$FILE_NAME" aether-agent

TARGET="/usr/local/bin/aether-agent"
if [ ! -w "$(dirname "$TARGET")" ]; then
  TARGET="$HOME/.local/bin/aether-agent"
  mkdir -p "$(dirname "$TARGET")"
  echo "Installing to $TARGET (add $(dirname "$TARGET") to your PATH if needed)."
fi

mv aether-agent "$TARGET"
chmod +x "$TARGET"

cd - >/dev/null
rm -rf "$TMP_DIR"

LAUNCHER="$HOME/.config/aether/aether-agent-launcher.sh"
cat >"$LAUNCHER" <<EOF
#!/bin/bash
set -a
if [ -f "\$HOME/.config/aether/aether-agent.env" ]; then
  # shellcheck disable=SC1090
  source "\$HOME/.config/aether/aether-agent.env"
fi
set +a
exec "$TARGET"
EOF
chmod +x "$LAUNCHER"

PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST_PATH="$PLIST_DIR/com.aether.agent.plist"
mkdir -p "$PLIST_DIR"

cat >"$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>com.aether.agent</string>
    <key>ProgramArguments</key>
    <array>
      <string>$LAUNCHER</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.cache/aether/aether-agent.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.cache/aether/aether-agent.log</string>
  </dict>
</plist>
EOF

# Reload LaunchAgent (ignore unload errors if it wasn't loaded yet).
launchctl unload "$PLIST_PATH" >/dev/null 2>&1 || true
launchctl load "$PLIST_PATH"

echo "Aether agent installed at $TARGET"
echo "LaunchAgent installed at $PLIST_PATH"
echo "View logs in ~/.cache/aether/aether-agent.log"
printf "Change environment variables in ~/.config/aether/aether-agent.env\n"
