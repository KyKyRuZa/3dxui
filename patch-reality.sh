#!/bin/bash
set -e

CONTAINER="3xui"
CONFIG_PATH="/app/bin/config.json"
TMP_PATH="/tmp/config.json"

# Копируем конфиг из контейнера
docker cp "${CONTAINER}:${CONFIG_PATH}" "${TMP_PATH}"

# Патчим: добавляем settings в realitySettings если отсутствует или неполный
python3 << 'PYEOF'
import json, sys

with open("/tmp/config.json") as f:
    cfg = json.load(f)

patched = False
for inbound in cfg.get("inbounds", []):
    if inbound.get("protocol") != "vless":
        continue
    ss = inbound.get("streamSettings", {})
    rs = ss.get("realitySettings", {})
    if not rs:
        continue
    
    settings = rs.get("settings", {})
    
    # Добавляем недостающие поля
    defaults = {
        "publicKey": "_HtHehCc9jxZYtB5JVrjnkT_6VmtV2HH-wL-Y6BAHGg",
        "fingerprint": "chrome",
        "spiderX": "/mGn5imjkPwwKbgh"
    }
    
    for k, v in defaults.items():
        if not settings.get(k):
            settings[k] = v
            patched = True
    
    if not rs.get("settings") or patched:
        rs["settings"] = settings
        patched = True

if patched:
    with open("/tmp/config.json", "w") as f:
        json.dump(cfg, f, indent=2)
    print("CONFIG_PATCHED")
else:
    print("CONFIG_OK")
PYEOF

# Если конфиг был изменён — копируем обратно и перезапускаем
if grep -q "CONFIG_PATCHED" /tmp/config.json; then
    # В данном случае python3 вывод в stdout, не в файл. Исправляем:
    :
fi

# Проще: делаем patch через python и проверяем exit code
PATCH_RESULT=$(python3 << 'PYEOF'
import json, sys

with open("/tmp/config.json") as f:
    cfg = json.load(f)

patched = False
for inbound in cfg.get("inbounds", []):
    if inbound.get("protocol") != "vless":
        continue
    rs = inbound.get("streamSettings", {}).get("realitySettings", {})
    if not rs:
        continue
    settings = rs.get("settings", {})
    defaults = {
        "publicKey": "_HtHehCc9jxZYtB5JVrjnkT_6VmtV2HH-wL-Y6BAHGg",
        "fingerprint": "chrome",
        "spiderX": "/mGn5imjkPwwKbgh"
    }
    for k, v in defaults.items():
        if not settings.get(k):
            settings[k] = v
            patched = True
    if patched:
        rs["settings"] = settings

if patched:
    with open("/tmp/config.json", "w") as f:
        json.dump(cfg, f, indent=2)
    print("PATCHED")
else:
    print("OK")
PYEOF
)

echo "$PATCH_RESULT"

if [ "$PATCH_RESULT" = "PATCHED" ]; then
    docker cp /tmp/config.json "${CONTAINER}:${CONFIG_PATH}"
    echo "Config patched and copied. Restarting container..."
    docker restart "${CONTAINER}"
    sleep 5
    docker exec "${CONTAINER}" netstat -tlnp | grep 443 || true
else
    echo "Config already has required settings. Just restarting xray..."
    docker exec "${CONTAINER}" sh -c 'kill -HUP $(pgrep -f xray-linux-amd64)' 2>/dev/null || docker restart "${CONTAINER}"
    sleep 3
    docker exec "${CONTAINER}" netstat -tlnp | grep 443 || true
fi

rm -f /tmp/config.json
