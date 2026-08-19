#!/bin/sh
set -e
if [ -d "/app/dist-fresh" ]; then
  rm -rf /app/dist/* /app/dist/.* 2>/dev/null || true
  cp -r /app/dist-fresh/. /app/dist/
fi
exec "$@"
