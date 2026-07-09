#!/bin/bash
# kryos-stress-test.sh — CPU stress test with KryOs monitoring.
# Runs stress-ng for 5 minutes and shows KryOs status every 10s.
# Press Ctrl+C to stop early (cleans up stress-ng automatically).

set -e

# --- Configuration ---
DURATION=300  # 5 minutes
INTERVAL=10   # match KryOs timer interval
KRYOS="/usr/local/bin/kryos"

# --- Helpers ---
cleanup() {
	echo
	echo "=== Stopping stress test ==="
	kill "$STRESS_PID" 2>/dev/null || true
	wait "$STRESS_PID" 2>/dev/null || true
	echo "stress-ng stopped."
	print_summary
	exit 0
}

print_summary() {
	echo
	echo "=== Test Summary ==="
	echo "Duration: $((DURATION - REMAINING))s / ${DURATION}s"
	echo "Peak CPU: ${PEAK_CPU:-N/A}°C"
	echo "Peak liquid: ${PEAK_LIQ:-N/A}°C"
	echo "Max pump level: ${MAX_PUMP:-0}"
	echo "Max fan level: ${MAX_FAN:-0}"
}

# --- Prerequisites ---
if ! command -v stress-ng &>/dev/null; then
	echo "stress-ng is not installed. Install it with:"
	echo "  sudo dnf install stress-ng   # Fedora"
	echo "  sudo apt install stress-ng   # Debian/Ubuntu"
	exit 1
fi

if ! command -v "$KRYOS" &>/dev/null; then
	echo "KryOs binary not found at $KRYOS"
	echo "Install it first: sudo kryos --install"
	exit 1
fi

if [[ $EUID -ne 0 ]]; then
	echo "This script needs root to read KryOs status. Run with sudo."
	exit 1
fi

# --- Trap Ctrl+C ---
trap cleanup SIGINT SIGTERM

# --- Initial state ---
echo "=== KryOs Stress Test ==="
echo "Duration: ${DURATION}s (${DURATION}s / 60 = $((DURATION / 60))min)"
echo "Press Ctrl+C to stop early"
echo
"$KRYOS" --status
echo

# --- Run stress-ng in background ---
stress-ng --cpu "$(nproc)" --timeout "${DURATION}s" --metrics-brief &
STRESS_PID=$!

# --- Monitor loop ---
PEAK_CPU=0
PEAK_LIQ=0
MAX_PUMP=0
MAX_FAN=0
REMAINING=$DURATION

while kill -0 "$STRESS_PID" 2>/dev/null; do
	sleep "$INTERVAL"
	REMAINING=$((REMAINING - INTERVAL))
	[[ $REMAINING -lt 0 ]] && REMAINING=0

	echo "--- ${REMAINING}s remaining ---"
	STATUS=$("$KRYOS" --status 2>&1)

	# Extract values for summary (silently)
	CPU=$(echo "$STATUS" | grep "^CPU:" | grep -oP '[\d.]+' | head -1)
	LIQ=$(echo "$STATUS" | grep "^Liquid:" | grep -oP '[\d.]+' | head -1)
	# Show current status
	echo "$STATUS" | tail -n +2
	echo

	# Track peaks
	PEAK_CPU=$(echo "$CPU $PEAK_CPU" | awk '{if ($1+0 > $2+0) print $1; else print $2}')
	PEAK_LIQ=$(echo "$LIQ $PEAK_LIQ" | awk '{if ($1+0 > $2+0) print $1; else print $2}')

	# Track levels from state file
	if STATE=$(sudo "$KRYOS" --get-state 2>/dev/null); then
		PUMP=$(echo "$STATE" | awk '{print $1}')
		FAN=$(echo "$STATE" | awk '{print $2}')
		[[ $PUMP -gt $MAX_PUMP ]] && MAX_PUMP=$PUMP
		[[ $FAN -gt $MAX_FAN ]] && MAX_FAN=$FAN
	fi
done

# --- Completion ---
echo "=== Stress test complete ==="
print_summary
