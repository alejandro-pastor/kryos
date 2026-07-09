#!/bin/bash
# kryos-dryrun.sh — Simulate one KryOs control cycle without writing PWM.
# Prints what KryOs would do: computed levels and duty percentages.
# State is saved so the next real tick starts from the correct level.
# Useful for A/B testing or validating hysteresis behavior.

set -e

KRYOS="/usr/local/bin/kryos"

if ! command -v "$KRYOS" &>/dev/null; then
	echo "KryOs binary not found at $KRYOS"
	exit 1
fi

if [[ $EUID -ne 0 ]]; then
	echo "Root required. Run with sudo."
	exit 1
fi

exec "$KRYOS" --once --dry-run
