#!/bin/sh
set -e

if [ "$1" = "purge" ]; then
	rm -f /etc/aether.conf
fi

