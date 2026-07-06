#!/bin/bash
# kryos-ab-monitor: captura states de bash y kryos dry-run cada N segundos.
# Ejecutar como root. Output append-only a /var/log/kryos-ab.log
set -e
LOG=/var/log/kryos-ab.log
INTERVAL="${INTERVAL:-10}"
DURATION="${DURATION:-600}"

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: ejecuta con sudo"
    exit 1
fi

echo "timestamp,bash_pump,bash_fan,kryos_pump,kryos_fan,cpu_x1000,liquid_x1000" > "$LOG"
echo "Monitor arrancado: $DURATION segundos, intervalo ${INTERVAL}s, log=$LOG" >&2

END=$(($(date +%s) + DURATION))
while [ "$(date +%s)" -lt "$END" ]; do
    TS=$(date +%H:%M:%S)
    BASH=$(cat /run/kraken-curve.state 2>/dev/null | tr ' ' ',')
    KRYOS=$(cat /run/kryos/dryrun.state 2>/dev/null | tr ' ' ',')
    CPU=$(cat /sys/class/hwmon/hwmon4/temp1_input 2>/dev/null)
    LIQUID=$(cat /sys/class/hwmon/hwmon5/temp1_input 2>/dev/null)
    echo "$TS,$BASH,$KRYOS,$CPU,$LIQUID" >> "$LOG"
    sleep "$INTERVAL"
done
echo "Monitor terminado: $DURATION segundos capturados" >&2
