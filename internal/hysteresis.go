// Package internal: KryOs core logic.
package internal

// Production-validated thresholds for Z53/Z63/Z73/2023/2024.
const (
	// Pump reacts to CPU temperature with 6°C hysteresis.
	PumpCPUHysteresis = 6.0

	// Fan reacts to liquid temperature with 3°C hysteresis.
	FanLiquidHysteresis = 3.0
)

// thresholds[i] triggers the transition to level i+1.
var (
	PumpCPUThresholds  = [3]float64{55, 70, 85}
	FanLiquidThresholds = [3]float64{34, 38, 42}
	PumpDutyByLevel    = [4]int{35, 45, 65, 90}
	FanDutyByLevel     = [4]int{25, 35, 50, 70}
)

// Levels represents the current regulation state.
type Levels struct {
	Pump int
	Fan  int
}

// Compute applies hysteresis for pump (by CPU) and fan (by liquid).
func Compute(cpuTemp, liquidTemp float64, prev Levels) Levels {
	return Levels{
		Pump: computeLevel(cpuTemp, prev.Pump, PumpCPUThresholds[:], PumpCPUHysteresis),
		Fan:  computeLevel(liquidTemp, prev.Fan, FanLiquidThresholds[:], FanLiquidHysteresis),
	}
}

// computeLevel decides the level with hysteresis.
// Semantics (from the original bash): changes ONE level per tick, no jumps.
// - If prev=0: rise if temp >= thresholds[0] (level 1).
// - If prev>0:
//   - Rise to prev+1 if temp >= thresholds[prev].
//   - Otherwise fall to prev-1 if temp <= thresholds[prev-1] - hysteresis.
//   - Otherwise stay.
// Rise has priority over fall (matches the bash `if/elif`).
func computeLevel(temp float64, prev int, thresholds []float64, hysteresis float64) int {
	if prev < len(thresholds) && temp >= thresholds[prev] {
		return prev + 1
	}
	if prev > 0 && temp <= thresholds[prev-1]-hysteresis {
		return prev - 1
	}
	return prev
}
