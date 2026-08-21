#!/bin/sh
set -eu

puid="${PUID:-1000}"
pgid="${PGID:-1000}"

case "$puid" in *[!0-9]*|'') echo "PUID must be a numeric ID" >&2; exit 1 ;; esac
case "$pgid" in *[!0-9]*|'') echo "PGID must be a numeric ID" >&2; exit 1 ;; esac

mkdir -p /data
chown "$puid:$pgid" /data

exec su-exec "$puid:$pgid" /app/novelreader "$@"
