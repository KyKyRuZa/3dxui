#!/bin/sh
set -e
if [ -d "/app/dist-fresh" ]; then
  find /app/dist -mindepth 1 -delete 2>/dev/null || true
  cp -r /app/dist-fresh/. /app/dist/
fi
exec "$@"
