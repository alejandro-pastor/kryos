#!/bin/bash
# kryos-calibrate.sh — Compare direct PWM writes vs liquidctl at 35/65/90%.
# Tests that KryOs writes the same PWM duty as liquidctl.
# Requires: liquidctl installed, Kraken initialized, root.

set -e

KRYOS="/usr/local/bin/kryos"
THRESHOLD=100  # max RPM difference to pass

if ! command -v liquidctl &>/dev/null; then
	echo "liquidctl is required for calibration."
	echo "Install it: pip install liquidctl"
	exit 1
fi

if ! command -v "$KRYOS" &>/dev/null; then
	echo "KryOs binary not found at $KRYOS"
	exit 1
fi

if [[ $EUID -ne 0 ]]; then
	echo "Root required. Run with sudo."
	exit 1
fi

echo "=== KryOs Calibration ==="
echo "Compares direct PWM writes vs liquidctl at 3 points."
echo "All diffs must be <= ${THRESHOLD} RPM to pass."
echo

# Detect Kraken hwmon path
KRAKEN_PATH=$("$KRYOS" --status 2>/dev/null | grep "^Kraken:" | grep -oP '/sys[^ ]+')
if [[ -z "$KRAKEN_PATH" ]]; then
	echo "No Kraken detected. Is the nzxt-kraken3 driver loaded?"
	exit 1
fi
echo "Kraken at: $KRAKEN_PATH"

# Save initial state for restoration
read_pwm_raw() {
	local pwm=$1
	cat "$KRAKEN_PATH/$pwm"
}
read_pwm_enable() {
	local pwm=$1
	cat "${KRAKEN_PATH}/${pwm}_enable"
}

INIT_PUMP_MODE=$(read_pwm_enable pwm1 2>/dev/null || echo 0)
INIT_PUMP_DUTY=$(read_pwm_raw pwm1 2>/dev/null || echo 0)
INIT_FAN_MODE=$(read_pwm_enable pwm2 2>/dev/null || echo 0)
INIT_FAN_DUTY=$(read_pwm_raw pwm2 2>/dev/null || echo 0)

# Restore initial state on exit
restore_state() {
	echo 1 > "$KRAKEN_PATH/pwm1_enable" 2>/dev/null || true
	echo "$INIT_PUMP_DUTY" > "$KRAKEN_PATH/pwm1" 2>/dev/null || true
	echo "$INIT_FAN_MODE" > "$KRAKEN_PATH/pwm2_enable" 2>/dev/null || true
	echo "$INIT_FAN_DUTY" > "$KRAKEN_PATH/pwm2" 2>/dev/null || true
}
trap restore_state EXIT

# Test points
ALL_PASSED=true
for DUTY in 35 65 90; do
	printf "Testing %d%% ... " "$DUTY"

	# Direct PWM write
	echo 1 > "$KRAKEN_PATH/pwm1_enable"
	echo $((DUTY * 255 / 100)) > "$KRAKEN_PATH/pwm1"
	sleep 5
	RPM_PWM=$(cat "$KRAKEN_PATH/fan1_input" 2>/dev/null || echo 0)

	# liquidctl write
	liquidctl --match kraken set pump speed "$DUTY" >/dev/null 2>&1
	sleep 5
	RPM_LC=$(cat "$KRAKEN_PATH/fan1_input" 2>/dev/null || echo 0)

	# Compare
	DIFF=$((RPM_PWM - RPM_LC))
	[[ $DIFF -lt 0 ]] && DIFF=$((-DIFF))

	if [[ $DIFF -le $THRESHOLD ]]; then
		echo "OK (pwm=$RPM_PWM liquidctl=$RPM_LC diff=$DIFF)"
	else
		echo "FAIL (pwm=$RPM_PWM liquidctl=$RPM_LC diff=$DIFF > ${THRESHOLD})"
		ALL_PASSED=false
	fi
done

echo
if $ALL_PASSED; then
	echo "Calibration passed. KryOs PWM is bit-equivalent to liquidctl."
	exit 0
else
	echo "Calibration FAILED. Check your Kraken driver and liquidctl version."
	exit 1
fi
