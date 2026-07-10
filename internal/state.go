package internal

import (
	"os"
	"strconv"
	"strings"
)

// DefaultStatePath is the state file location.
// Persists across reboots via systemd StateDirectory=kryos in the .service.
// /run/kryos/ was discarded because with Type=oneshot systemd deletes
// RuntimeDirectory between ticks, breaking hysteresis.
const DefaultStatePath = "/var/lib/kryos/curve.state"

// Load reads the state file. If missing or corrupt, returns {0, 0}.
// Policy: degrade to safe state, auto-recover in 10s.
func Load(path string) (Levels, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Levels{Pump: 0, Fan: 0}, err
	}
	parts := strings.Fields(string(data))
	if len(parts) != 2 {
		return Levels{Pump: 0, Fan: 0}, nil
	}
	pump, err1 := strconv.Atoi(parts[0])
	fan, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return Levels{Pump: 0, Fan: 0}, nil
	}
	// Validate range: levels must be 0-4 (4 = emergency).
	// Corrupt values would cause a panic when indexing duty arrays.
	if pump < 0 || pump > 4 || fan < 0 || fan > 4 {
		return Levels{Pump: 0, Fan: 0}, nil
	}
	return Levels{Pump: pump, Fan: fan}, nil
}

// Save writes the state file in "<pump> <fan>" format.
func Save(path string, levels Levels) error {
	content := strconv.Itoa(levels.Pump) + " " + strconv.Itoa(levels.Fan) + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}
