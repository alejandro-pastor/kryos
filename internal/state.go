package internal

import (
	"os"
	"strconv"
	"strings"
)

// DefaultStatePath es la ubicación del state file.
// Persiste entre reboots; lo crea systemd mediante StateDirectory=kryos
// en el .service. Decisión: se descartó /run/kryos/ porque con Type=oneshot
// el RuntimeDirectory se borra entre ticks, rompiendo la histéresis.
const DefaultStatePath = "/var/lib/kryos/curve.state"

// Load lee el state file. Si no existe o está corrupto, devuelve {0, 0}.
// Política del handoff: degradar a estado seguro, autorecupera en 10s.
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
	return Levels{Pump: pump, Fan: fan}, nil
}

// Save escribe el state file en formato "<pump> <fan>".
func Save(path string, levels Levels) error {
	content := strconv.Itoa(levels.Pump) + " " + strconv.Itoa(levels.Fan) + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}
