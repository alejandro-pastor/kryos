// Package internal: KryOs core logic.
package internal

// Production-validated thresholds for Z53/Z63/Z73/2023/2024.
const (
	// Pump reacts to CPU temperature with 6°C hysteresis.
	PumpCPUHysteresis = 6.0

	// Fan reacts to liquid temperature with 3°C hysteresis.
	FanLiquidHysteresis = 3.0

	// LiquidEmergencyThreshold forces pump and fan to 100% when liquid
	// reaches this temperature. Deactivates with 5°C hysteresis (46°C).
	LiquidEmergencyThreshold = 51.0
)

// thresholds[i] triggers the transition to level i+1.
// Level 4 (index 4) is the emergency level (always 100%).
var (
	PumpCPUThresholds   = [3]float64{55, 70, 85}
	FanLiquidThresholds  = [3]float64{34, 38, 42}
	PumpDutyByLevel     = [5]int{35, 45, 65, 90, 100}
	FanDutyByLevel      = [5]int{25, 35, 50, 70, 100}
)

// Levels represents the current regulation state.
type Levels struct {
	Pump int
	Fan  int
}

// Compute applies hysteresis for pump (by CPU) and fan (by liquid).
// Level 4 is the emergency level: both pump and fan forced to 100%
// when liquid temperature reaches 51°C.
func Compute(cpuTemp, liquidTemp float64, prev Levels) Levels {
	// Emergency: liquid too hot, force everything to 100%.
	if liquidTemp >= LiquidEmergencyThreshold {
		return Levels{Pump: 4, Fan: 4}
	}

	// Exiting emergency: liquid dropped below 46°C (51-5).
	// Cap previous level to 3 so computeLevel doesn't index out of bounds.
	if prev.Pump == 4 || prev.Fan == 4 {
		if liquidTemp < LiquidEmergencyThreshold-5.0 {
			pumpPrev := prev.Pump
			fanPrev := prev.Fan
			if pumpPrev > 3 {
				pumpPrev = 3
			}
			if fanPrev > 3 {
				fanPrev = 3
			}
			return Levels{
				Pump: computeLevel(cpuTemp, pumpPrev, PumpCPUThresholds[:], PumpCPUHysteresis),
				Fan:  computeLevel(liquidTemp, fanPrev, FanLiquidThresholds[:], FanLiquidHysteresis),
			}
		}
		// Still in the hysteresis band, stay in emergency.
		return Levels{Pump: 4, Fan: 4}
	}

	// Normal regulation.
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
