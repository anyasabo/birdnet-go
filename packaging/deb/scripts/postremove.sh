#!/bin/sh
# postremove: runs after files are removed.
set -e

# Refresh the linker cache now that the vendored libraries are gone.
ldconfig 2>/dev/null || true

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
fi

# On purge, drop the service user. Data in /var/lib/birdnet-go is intentionally
# left in place so a reinstall keeps the database and clips; remove it by hand
# if you really want a clean slate.
if [ "$1" = "purge" ]; then
    if getent passwd birdnet-go >/dev/null 2>&1; then
        deluser --system birdnet-go >/dev/null 2>&1 || true
    fi
    if getent group birdnet-go >/dev/null 2>&1; then
        delgroup --system birdnet-go >/dev/null 2>&1 || true
    fi
fi

exit 0
