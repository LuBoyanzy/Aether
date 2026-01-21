#!/bin/sh
set -e

[ "$1" = "configure" ] || exit 0

CONFIG_FILE=/etc/aether.conf
SERVICE=aether
SERVICE_USER=aether

# Create group and user (idempotent)
if ! getent group "$SERVICE_USER" >/dev/null; then
	echo "Creating $SERVICE_USER group"
	addgroup --quiet --system "$SERVICE_USER"
fi

if ! getent passwd "$SERVICE_USER" >/dev/null; then
	echo "Creating $SERVICE_USER user"
	adduser --quiet --system "$SERVICE_USER" \
		--ingroup "$SERVICE_USER" \
		--no-create-home \
		--home /nonexistent \
		--gecos "System user for $SERVICE"
fi

# Create config file if it doesn't already exist.
# Unit 文件中提供了 HTTP 默认值，因此该文件允许为空（用户可自行写入 APP_URL 等变量）。
if [ ! -f "$CONFIG_FILE" ]; then
	touch "$CONFIG_FILE"
	chmod 0600 "$CONFIG_FILE"
	chown "$SERVICE_USER":"$SERVICE_USER" "$CONFIG_FILE"
fi

deb-systemd-helper enable "$SERVICE".service
systemctl daemon-reload
deb-systemd-invoke start "$SERVICE".service || echo "could not start $SERVICE.service!"

