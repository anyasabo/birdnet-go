#!/bin/sh
# preremove: runs before files are removed. On upgrade ($1 = "upgrade") systemd
# leaves the unit alone; the new postinstall restarts it. Only stop/disable on a
# real removal.
set -e

if [ -d /run/systemd/system ]; then
    case "$1" in
        remove|purge|"")
            systemctl stop birdnet-go.service || true
            systemctl disable birdnet-go.service || true
            ;;
    esac
fi

exit 0
