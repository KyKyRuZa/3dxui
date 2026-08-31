#!/bin/sh
set -e
if [ -d "/app/dist-fresh" ]; then
  rm -rf /app/dist
  mkdir -p /app/dist
  cp -r /app/dist-fresh/. /app/dist/
fi
exec "$@"
