#!/bin/bash
# Setup del A/B test en dry-run. Requiere sudo.
# 1. Instala kryos en /usr/local/bin/
# 2. Crea /run/kryos/ con permisos correctos
# 3. Copia kryos-dryrun.service y .timer a /etc/systemd/system/
# 4. Hace daemon-reload, enable y start del timer
set -e

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: ejecuta con sudo"
    exit 1
fi

BIN=${BIN:-/usr/local/bin/kryos}
SCRIPTS=${SCRIPTS:-/usr/local/share/kryos/scripts}

install -m 0755 "$BIN" /usr/local/bin/kryos
mkdir -p /run/kryos
chmod 0750 /run/kryos

install -m 0644 "$SCRIPTS/kryos-dryrun.service" /etc/systemd/system/kryos-dryrun.service
install -m 0644 "$SCRIPTS/kryos-dryrun.timer" /etc/systemd/system/kryos-dryrun.timer

systemctl daemon-reload
systemctl enable --now kryos-dryrun.timer

echo "=== Estado ==="
systemctl status kryos-dryrun.timer --no-pager | head -8
echo "---"
systemctl list-timers --all | grep kryos
