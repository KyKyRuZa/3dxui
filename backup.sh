#!/bin/sh
set -e

BACKUP_DIR="/opt/3dxui/backups"
DB_NAME="${DB_NAME:-vpn_db}"
DB_USER="${DB_USER:-vpn_user}"
DAYS="${BACKUP_RETENTION_DAYS:-7}"
DATE="$(date +%Y%m%d_%H%M%S)"
FILE="$BACKUP_DIR/${DB_NAME}_${DATE}.sql.gz"

mkdir -p "$BACKUP_DIR"

docker compose exec -T postgres pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$FILE"

find "$BACKUP_DIR" -name "${DB_NAME}_*.sql.gz" -type f -mtime +$DAYS -delete
