#!/bin/bash
# kryos-ab-monitor: captures bash and kryos dry-run states every N seconds.
# Run as root. Output is append-only to LOG (default: /var/log/kryos-ab.log)
set -e
LOG=${LOG:-/var/log/kryos-ab.log}
INTERVAL="${INTERVAL:-10}"
DURATION="${DURATION:-600}"

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: run with sudo"
    exit 1
fi

echo "timestamp,bash_pump,bash_fan,kryos_pump,kryos_fan,cpu_x1000,liquid_x1000" > "$LOG"
echo "Monitor started: ${DURATION}s, interval ${INTERVAL}s, log=$LOG" >&2

END=$(($(date +%s) + DURATION))
while [ "$(date +%s)" -lt "$END" ]; do
    TS=$(date +%H:%M:%S)
    BASH_STATE_PATH=${BASH_STATE_PATH:-/run/kraken-curve.state}
    BASH=$(cat "$BASH_STATE_PATH" 2>/dev/null | tr ' ' ',')
    KRYOS=$(cat /run/kryos/dryrun.state 2>/dev/null | tr ' ' ',')
    CPU=$(cat /sys/class/hwmon/hwmon4/temp1_input 2>/dev/null)
    LIQUID=$(cat /sys/class/hwmon/hwmon5/temp1_input 2>/dev/null)
    echo "$TS,$BASH,$KRYOS,$CPU,$LIQUID" >> "$LOG"
    sleep "$INTERVAL"
done
echo "Monitor finished: ${DURATION}s captured" >&2
