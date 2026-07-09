package internal

import "testing"

// TestCompute_Pump validates pump threshold logic from the original bash.
// CPU thresholds: 55/70/85, hysteresis: 6°C.
func TestCompute_Pump(t *testing.T) {
	tests := []struct {
		name     string
		cpuTemp  float64
		prev     int
		expected int
	}{
		// Rise from 0
		{"level 0→1 CPU=55", 55.0, 0, 1},
		{"level 0→1 CPU=60", 60.0, 0, 1},
		{"level 0→1 CPU=70 (no jump)", 70.0, 0, 1},
		{"level 0→1 CPU=90 (no jump)", 90.0, 0, 1},
		// Hold
		{"level 1→1 CPU=60", 60.0, 1, 1},
		{"level 1→1 CPU=69", 69.0, 1, 1},
		{"level 2→2 CPU=75", 75.0, 2, 2},
		{"level 3→3 CPU=86", 86.0, 3, 3},
		// Fall one level (threshold - hysteresis)
		{"level 1→0 CPU=48", 48.0, 1, 0},
		{"level 2→1 CPU=63", 63.0, 2, 1},
		{"level 3→2 CPU=78", 78.0, 3, 2},
		// No fall (between threshold and threshold-hysteresis)
		{"level 1→1 CPU=50", 50.0, 1, 1},
		{"level 2→2 CPU=65", 65.0, 2, 2},
		{"level 3→3 CPU=80", 80.0, 3, 3},
		// Rise one level
		{"level 1→2 CPU=70", 70.0, 1, 2},
		{"level 2→3 CPU=85", 85.0, 2, 3},
		// Rise has priority over fall
		{"level 1→2 CPU=70 (rise wins)", 70.0, 1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeLevel(tt.cpuTemp, tt.prev, PumpCPUThresholds[:], PumpCPUHysteresis)
			if result != tt.expected {
				t.Errorf("computeLevel(%.1f, %d) = %d, want %d",
					tt.cpuTemp, tt.prev, result, tt.expected)
			}
		})
	}
}

// TestCompute_Fan validates fan threshold logic.
// Liquid thresholds: 34/38/42, hysteresis: 3°C.
func TestCompute_Fan(t *testing.T) {
	tests := []struct {
		name       string
		liquidTemp float64
		prev       int
		expected   int
	}{
		// Rise from 0
		{"level 0→1 liquid=34", 34.0, 0, 1},
		{"level 0→1 liquid=36", 36.0, 0, 1},
		{"level 0→1 liquid=38 (no jump)", 38.0, 0, 1},
		{"level 0→1 liquid=42 (no jump)", 42.0, 0, 1},
		// Hold
		{"level 1→1 liquid=35", 35.0, 1, 1},
		{"level 2→2 liquid=40", 40.0, 2, 2},
		{"level 3→3 liquid=43", 43.0, 3, 3},
		// Fall one level
		{"level 1→0 liquid=30", 30.0, 1, 0},
		{"level 2→1 liquid=34", 34.0, 2, 1},
		{"level 3→2 liquid=38", 38.0, 3, 2},
		// No fall
		{"level 1→1 liquid=32", 32.0, 1, 1},
		{"level 2→2 liquid=36", 36.0, 2, 2},
		// Rise one level
		{"level 1→2 liquid=38", 38.0, 1, 2},
		{"level 2→3 liquid=42", 42.0, 2, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeLevel(tt.liquidTemp, tt.prev, FanLiquidThresholds[:], FanLiquidHysteresis)
			if result != tt.expected {
				t.Errorf("computeLevel(%.1f, %d) = %d, want %d",
					tt.liquidTemp, tt.prev, result, tt.expected)
			}
		})
	}
}

// TestCompute_Integration validates pump and fan simultaneously.
func TestCompute_Integration(t *testing.T) {
	prev := Levels{Pump: 0, Fan: 0}

	// CPU at 60, liquid at 36: pump level 1, fan level 1
	result := Compute(60.0, 36.0, prev)
	expected := Levels{Pump: 1, Fan: 1}
	if result != expected {
		t.Errorf("Compute(60, 36) = %+v, want %+v", result, expected)
	}

	// Now CPU at 75, liquid at 40: pump 1→2, fan 1→2
	result = Compute(75.0, 40.0, result)
	expected = Levels{Pump: 2, Fan: 2}
	if result != expected {
		t.Errorf("Compute(75, 40) = %+v, want %+v", result, expected)
	}
}

// TestCompute_CPURamp simulates a CPU stress cycle:
// tick 1: light load (CPU 60, liquid 35) → 1 1
// tick 2: medium load (CPU 72, liquid 39) → 2 2
// tick 3: heavy load (CPU 86, liquid 42) → 3 3
// tick 4: CPU drops but liquid stays high → 3 3
// tick 5: CPU=75 (in hysteresis, fall 1 level) → 2 3
// tick 6: CPU=60, liquid=38 → 1 2
// tick 7: CPU=45, liquid=34 → 0 1
// tick 8: CPU=42, liquid=33 → 0 1
func TestCompute_CPURamp(t *testing.T) {
	sec := []struct {
		cpu, liquid float64
		expected    Levels
	}{
		{45, 33, Levels{0, 0}},
		{60, 35, Levels{1, 1}},
		{72, 39, Levels{2, 2}},
		{86, 42, Levels{3, 3}},
		{75, 41, Levels{2, 3}},
		{60, 38, Levels{1, 2}},
		{45, 34, Levels{0, 1}},
		{42, 33, Levels{0, 1}},
	}

	prev := Levels{Pump: 0, Fan: 0}
	for i, s := range sec {
		result := Compute(s.cpu, s.liquid, prev)
		if result != s.expected {
			t.Errorf("tick %d: Compute(%.1f, %.1f) = %+v, want %+v",
				(i+1)*10, s.cpu, s.liquid, result, s.expected)
		}
		prev = result
	}
}

// TestCompute_LiquidSpike simulates a liquid temperature spike:
// validates pump and fan are independent (pump does not react to liquid,
// fan does not react to CPU).
func TestCompute_LiquidSpike(t *testing.T) {
	sec := []struct {
		cpu, liquid float64
		expected    Levels
	}{
		{50, 33, Levels{0, 0}},
		{55, 36, Levels{1, 1}},
		{60, 39, Levels{1, 2}},
		{60, 43, Levels{1, 3}},
		{60, 40, Levels{1, 3}},
		{55, 35, Levels{1, 2}},
		{55, 32, Levels{1, 1}},
		{50, 30, Levels{1, 0}},
	}

	prev := Levels{Pump: 0, Fan: 0}
	for i, s := range sec {
		result := Compute(s.cpu, s.liquid, prev)
		if result != s.expected {
			t.Errorf("tick %d: Compute(%.1f, %.1f) = %+v, want %+v",
				(i+1)*10, s.cpu, s.liquid, result, s.expected)
		}
		prev = result
	}
}
