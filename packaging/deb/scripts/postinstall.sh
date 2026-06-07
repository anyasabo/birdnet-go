#!/bin/sh
# postinstall: runs after files are unpacked, on install AND upgrade.
set -e

SERVICE_USER=birdnet-go
DATA_DIR=/var/lib/birdnet-go

# 1. Create a dedicated system user/group (idempotent).
if ! getent group "$SERVICE_USER" >/dev/null 2>&1; then
    addgroup --system "$SERVICE_USER"
fi
if ! getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
    adduser --system --no-create-home --ingroup "$SERVICE_USER" \
        --home "$DATA_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"
fi

# 2. Give the service user access to sound devices for mic capture.
if getent group audio >/dev/null 2>&1; then
    adduser "$SERVICE_USER" audio >/dev/null 2>&1 || true
fi

# 3. Ensure the data dir exists and is owned by the service user.
mkdir -p "$DATA_DIR"
chown "$SERVICE_USER":"$SERVICE_USER" "$DATA_DIR"
chmod 0750 "$DATA_DIR"

# 4. Register the vendored shared libraries with the dynamic linker.
ldconfig

# 5. Reload systemd and (re)start the service.
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
    # enable on first install; restart picks up new binary on upgrade.
    systemctl enable birdnet-go.service || true
    systemctl restart birdnet-go.service || true
fi

exit 0
