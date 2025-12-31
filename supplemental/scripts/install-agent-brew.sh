#!/bin/bash

PORT=45876
KEY=""
TOKEN=""
HUB_URL=""
REPO_OWNER="LuBoyanzy"
REPO_NAME="Aether"

usage() {
  printf "Aether Agent macOS installation script (direct GitHub download)\n\n"
  printf "Usage: ./install-agent-brew.sh [options]\n\n"
  printf "Options: \n"
  printf "  -k            SSH key (required, or interactive if not provided)\n"
  printf "  -p            Port (default: $PORT)\n"
  printf "  -t            Token (optional)\n"
  printf "  -url          Hub URL (optional)\n"
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

CHECKSUM=$(curl -sL "https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/v${INSTALL_VERSION}/aether_${INSTALL_VERSION}_checksums.txt" | grep "$FILE_NAME" | cut -d' ' -f1)
if [ -z "$CHECKSUM" ]; then
  echo "Failed to fetch checksum"
  exit 1
fi

TMP_DIR=$(mktemp -d)
cd "$TMP_DIR" || exit 1
curl -#L "https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/v${INSTALL_VERSION}/${FILE_NAME}" -o "$FILE_NAME"

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

echo "Aether agent installed at $TARGET"
echo "View logs in ~/.cache/aether/aether-agent.log"
printf "Change environment variables in ~/.config/aether/aether-agent.env\n"
