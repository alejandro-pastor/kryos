// Package internal: lógica pura de KryOs.
package internal

// Umbrales validados en producción, válidos para Z53/Z63/Z73/2023/2024.
// Decisión del usuario: opción A del handoff v2, NO ajustar.
const (
	// Bomba: reacciona a CPU, histéresis 6°C
	PumpCPUHysteresis = 6.0

	// Fan: reacciona a líquido, histéresis 3°C
	FanLiquidHysteresis = 3.0
)

// Arrays no pueden ser const en Go; se declaran como var.
// thresholds[i] → nivel i+1
var (
	PumpCPUThresholds  = [3]float64{55, 70, 85}
	FanLiquidThresholds = [3]float64{34, 38, 42}
	PumpDutyByLevel    = [4]int{35, 45, 65, 90}
	FanDutyByLevel     = [4]int{25, 35, 50, 70}
)

// Levels representa el estado de regulación actual.
type Levels struct {
	Pump int
	Fan  int
}

// Compute aplica histéresis para bomba (por CPU) y fan (por líquido).
func Compute(cpuTemp, liquidTemp float64, prev Levels) Levels {
	return Levels{
		Pump: computeLevel(cpuTemp, prev.Pump, PumpCPUThresholds[:], PumpCPUHysteresis),
		Fan:  computeLevel(liquidTemp, prev.Fan, FanLiquidThresholds[:], FanLiquidHysteresis),
	}
}

// computeLevel decide el nivel con histéresis.
// Semántica (del bash original): el estado CAMBIA UN NIVEL por tick, no salta.
// - Si prev=0: subir si temp >= thresholds[0] (nivel 1).
// - Si prev>0:
//   - Subir a prev+1 si temp >= thresholds[prev] (umbral del nivel siguiente).
//   - Si no, bajar a prev-1 si temp <= thresholds[prev-1] - hysteresis.
//   - Si no, mantener.
// Subir tiene prioridad sobre bajar (replica el `if/elif` del bash).
func computeLevel(temp float64, prev int, thresholds []float64, hysteresis float64) int {
	if prev < len(thresholds) && temp >= thresholds[prev] {
		return prev + 1
	}
	if prev > 0 && temp <= thresholds[prev-1]-hysteresis {
		return prev - 1
	}
	return prev
}
