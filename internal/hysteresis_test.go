package internal

import "testing"

// TestCompute_Bomba replica la lógica del bash para umbrales de bomba.
// CPU: 55/70/85 sube, histéresis 6°C.
// Semántica del bash: cambio UN nivel por tick (no salto a 0).
// Casos del bash original portados literalmente.
func TestCompute_Bomba(t *testing.T) {
	tests := []struct {
		name     string
		cpuTemp  float64
		prev     int
		expected int
	}{
		// Subir desde 0
		{"nivel 0→1 con CPU=55", 55.0, 0, 1},
		{"nivel 0→1 con CPU=60", 60.0, 0, 1},
		{"nivel 0→1 con CPU=70 (no salta)", 70.0, 0, 1}, // primer tick: solo sube 1 nivel
		{"nivel 0→1 con CPU=90 (no salta)", 90.0, 0, 1}, // primer tick: solo sube 1 nivel
		// Mantener
		{"nivel 1→1 con CPU=60", 60.0, 1, 1},
		{"nivel 1→1 con CPU=69", 69.0, 1, 1}, // < 70, no sube
		{"nivel 2→2 con CPU=75", 75.0, 2, 2},
		{"nivel 3→3 con CPU=86", 86.0, 3, 3},
		// Bajar UN nivel (umbral - histéresis)
		{"nivel 1→0 con CPU=48", 48.0, 1, 0}, // 48 <= 55-6=49
		{"nivel 2→1 con CPU=63", 63.0, 2, 1}, // 63 <= 70-6=64, baja a 1 (no a 0)
		{"nivel 3→2 con CPU=78", 78.0, 3, 2}, // 78 <= 85-6=79, baja a 2 (no a 0)
		// No bajar (entre threshold y threshold-hysteresis)
		{"nivel 1→1 con CPU=50", 50.0, 1, 1}, // 50 > 49, no baja
		{"nivel 2→2 con CPU=65", 65.0, 2, 2}, // 65 > 64, no baja
		{"nivel 3→3 con CPU=80", 80.0, 3, 3}, // 80 > 79, no baja
		// Subir UN nivel
		{"nivel 1→2 con CPU=70", 70.0, 1, 2},
		{"nivel 2→3 con CPU=85", 85.0, 2, 3},
		// Subir tiene prioridad sobre bajar
		{"nivel 1→2 con CPU=70 (subida aunque podría bajar)", 70.0, 1, 2},
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

// TestCompute_Fan replica la lógica del bash para umbrales de fan.
// Líquido: 34/38/42 sube, histéresis 3°C.
// Semántica del bash: cambio UN nivel por tick.
func TestCompute_Fan(t *testing.T) {
	tests := []struct {
		name       string
		liquidTemp float64
		prev       int
		expected   int
	}{
		// Subir desde 0
		{"nivel 0→1 con líquido=34", 34.0, 0, 1},
		{"nivel 0→1 con líquido=36", 36.0, 0, 1},
		{"nivel 0→1 con líquido=38 (no salta)", 38.0, 0, 1},
		{"nivel 0→1 con líquido=42 (no salta)", 42.0, 0, 1},
		// Mantener
		{"nivel 1→1 con líquido=35", 35.0, 1, 1},
		{"nivel 2→2 con líquido=40", 40.0, 2, 2},
		{"nivel 3→3 con líquido=43", 43.0, 3, 3},
		// Bajar UN nivel
		{"nivel 1→0 con líquido=30", 30.0, 1, 0}, // 30 <= 34-3=31
		{"nivel 2→1 con líquido=34", 34.0, 2, 1}, // 34 <= 38-3=35, baja a 1
		{"nivel 3→2 con líquido=38", 38.0, 3, 2}, // 38 <= 42-3=39, baja a 2
		// No bajar
		{"nivel 1→1 con líquido=32", 32.0, 1, 1}, // 32 > 31, no baja
		{"nivel 2→2 con líquido=36", 36.0, 2, 2}, // 36 > 35, no baja
		// Subir UN nivel
		{"nivel 1→2 con líquido=38", 38.0, 1, 2},
		{"nivel 2→3 con líquido=42", 42.0, 2, 3},
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

// TestCompute_Integración verifica bomba y fan simultáneamente.
func TestCompute_Integración(t *testing.T) {
	// Estado inicial: 0 0
	prev := Levels{Pump: 0, Fan: 0}

	// CPU en 60, líquido en 36: bomba nivel 1, fan nivel 1
	result := Compute(60.0, 36.0, prev)
	expected := Levels{Pump: 1, Fan: 1}
	if result != expected {
		t.Errorf("Compute(60, 36) = %+v, want %+v", result, expected)
	}

	// Ahora CPU en 75, líquido en 40: bomba nivel 1→2, fan nivel 1→2
	result = Compute(75.0, 40.0, result)
	expected = Levels{Pump: 2, Fan: 2}
	if result != expected {
		t.Errorf("Compute(75, 40) = %+v, want %+v", result, expected)
	}
}

// TestCompute_RampaSostenida_CPU simula un pico de CPU:
// segundo 10: carga ligera (CPU 60, líquido 35) → 1 1
// segundo 20: carga media (CPU 72, líquido 39) → 2 2
// segundo 30: carga pesada (CPU 86, líquido 42) → 3 3
// segundo 40: CPU baja pero líquido sigue alto → 3 3
// segundo 50: CPU=75 (justo en histéresis, baja 1 nivel) → 2 3
// segundo 60: CPU=60 (≤ 64, baja a 1), líquido=38 (≤ 39, baja a 2) → 1 2
// segundo 70: CPU=45 (≤ 49, baja a 0), líquido=34 (no llega a 31) → 0 1
// segundo 80: CPU=42, líquido=33 → 0 1 (líquido aún no baja)
func TestCompute_RampaSostenida_CPU(t *testing.T) {
	sec := []struct {
		cpu, liquid float64
		expected    Levels
	}{
		{45, 33, Levels{0, 0}}, // idle inicial
		{60, 35, Levels{1, 1}}, // sube: bomba 1, fan 1
		{72, 39, Levels{2, 2}}, // sigue subiendo
		{86, 42, Levels{3, 3}}, // máximo
		{75, 41, Levels{2, 3}}, // CPU=75 baja 1 nivel (75<=79), líquido=41 mantiene
		{60, 38, Levels{1, 2}}, // CPU=60 baja a 1 (60<=64), líquido=38 baja a 2 (38<=39)
		{45, 34, Levels{0, 1}}, // CPU=45 baja a 0 (45<=49), líquido=34 baja a 1 (34<=35)
		{42, 33, Levels{0, 1}}, // CPU=42 mantiene 0, líquido=33 mantiene 1 (33>31)
	}

	prev := Levels{Pump: 0, Fan: 0}
	for i, s := range sec {
		result := Compute(s.cpu, s.liquid, prev)
		if result != s.expected {
			t.Errorf("segundo %d: Compute(%.1f, %.1f) = %+v, want %+v",
				(i+1)*10, s.cpu, s.liquid, result, s.expected)
		}
		prev = result
	}
}

// TestCompute_RampaRapida_Fan simula un pico de líquido:
// valida que bomba y fan son independientes (bomba no sube por líquido, fan no por CPU).
// Con la semántica "un nivel por tick", el fan tarda varios ticks en bajar
// desde 3 a 0, igual que el bash real.
func TestCompute_RampaRapida_Fan(t *testing.T) {
	sec := []struct {
		cpu, liquid float64
		expected    Levels
	}{
		{50, 33, Levels{0, 0}},
		{55, 36, Levels{1, 1}}, // CPU cruza 55 (bomba sube), líquido cruza 34 (fan sube)
		{60, 39, Levels{1, 2}}, // líquido cruza 38 (fan sube a 2)
		{60, 43, Levels{1, 3}}, // líquido cruza 42 (fan sube a 3, bomba se queda)
		{60, 40, Levels{1, 3}}, // líquido=40 (>39, fan mantiene 3)
		{55, 35, Levels{1, 2}}, // líquido=35 (≤39, fan baja a 2)
		{55, 32, Levels{1, 1}}, // líquido=32 (≤35, fan baja a 1)
		{50, 30, Levels{1, 0}}, // CPU=50 (>49, bomba mantiene), líquido=30 (≤31, fan baja a 0)
	}

	prev := Levels{Pump: 0, Fan: 0}
	for i, s := range sec {
		result := Compute(s.cpu, s.liquid, prev)
		if result != s.expected {
			t.Errorf("segundo %d: Compute(%.1f, %.1f) = %+v, want %+v",
				(i+1)*10, s.cpu, s.liquid, result, s.expected)
		}
		prev = result
	}
}
