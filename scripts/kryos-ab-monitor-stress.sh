#!/bin/bash
# kryos-ab-monitor-stress: captura SOLO durante stress test
set -e
LOG=${LOG:-/var/log/kryos-ab-stress.log}
DURATION="${DURATION:-280}"

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: ejecuta con sudo"
    exit 1
fi

echo "timestamp,bash_pump,bash_fan,kryos_pump,kryos_fan,cpu_x1000,liquid_x1000" > "$LOG"
echo "Monitor stress arrancado: ${DURATION}s" >&2

END=$(($(date +%s) + DURATION))
while [ "$(date +%s)" -lt "$END" ]; do
    TS=$(date +%H:%M:%S)
    BASH_STATE_PATH=${BASH_STATE_PATH:-/run/kraken-curve.state}
    BASH=$(cat "$BASH_STATE_PATH" 2>/dev/null | tr ' ' ',')
    KRYOS=$(cat /run/kryos/dryrun.state 2>/dev/null | tr ' ' ',')
    CPU=$(cat /sys/class/hwmon/hwmon4/temp1_input 2>/dev/null)
    LIQUID=$(cat /sys/class/hwmon/hwmon5/temp1_input 2>/dev/null)
    echo "$TS,$BASH,$KRYOS,$CPU,$LIQUID" >> "$LOG"
    sleep 5
done
echo "Monitor stress terminado" >&2
