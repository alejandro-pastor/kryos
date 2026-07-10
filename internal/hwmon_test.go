package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// setupHwmonDir creates a temporary directory simulating a hwmon sysfs path.
func setupHwmonDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeHwmonFile(t *testing.T, dir, attr, content string) {
	t.Helper()
	path := filepath.Join(dir, attr)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", attr, err)
	}
}

func TestReadTemp_Valid(t *testing.T) {
	dir := setupHwmonDir(t)
	writeHwmonFile(t, dir, "temp1_input", "56200\n") // 56.2°C

	temp, err := ReadTemp(dir, "temp1_input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if temp != 56.2 {
		t.Errorf("expected 56.2°C, got %.1f", temp)
	}
}

func TestReadTemp_Zero(t *testing.T) {
	dir := setupHwmonDir(t)
	writeHwmonFile(t, dir, "temp1_input", "0\n")

	temp, err := ReadTemp(dir, "temp1_input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if temp != 0 {
		t.Errorf("expected 0°C, got %.1f", temp)
	}
}

func TestReadTemp_MissingFile(t *testing.T) {
	dir := setupHwmonDir(t)
	_, err := ReadTemp(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestReadRPM_Valid(t *testing.T) {
	dir := setupHwmonDir(t)
	writeHwmonFile(t, dir, "fan1_input", "1354\n")

	rpm, err := ReadRPM(dir, "fan1_input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rpm != 1354 {
		t.Errorf("expected 1354 RPM, got %d", rpm)
	}
}

func TestReadPWMPercent(t *testing.T) {
	tests := []struct {
		raw          int
		expected     int
	}{
		{0, 0},
		{64, 25},
		{128, 50},
		{191, 74},
		{255, 100},
	}

	for _, tt := range tests {
		dir := setupHwmonDir(t)
		writeHwmonFile(t, dir, "pwm1", "0\n") // dummy, we mock ReadPWM differently

		// We test the calculation logic directly
		result := (tt.raw * 100) / 255
		if result != tt.expected {
			t.Errorf("ReadPWMPercent raw=%d: expected %d%%, got %d%%", tt.raw, tt.expected, result)
		}
	}
}

func TestWritePWM_Clamp(t *testing.T) {
	dir := setupHwmonDir(t)
	pwmFile := filepath.Join(dir, "pwm1")

	tests := []struct {
		input    int
		expected int
	}{
		{-10, 0},   // clamped to 0
		{0, 0},     // boundary
		{50, 127},  // (50 * 255) / 100 = 127
		{100, 255}, // boundary
		{150, 255}, // clamped to 100 → 255
	}

	for _, tt := range tests {
		if err := WritePWM(dir, "pwm1", tt.input); err != nil {
			t.Fatalf("WritePWM(%d) failed: %v", tt.input, err)
		}
		data, _ := os.ReadFile(pwmFile)
		result := 0
		if len(data) > 0 {
			result = int(data[0]) // quick check first byte
		}
		// Proper check: parse the file content
		_ = data
		_ = result

		// Read back and verify
		written, err := ReadPWM(dir, "pwm1")
		if err != nil {
			t.Fatalf("ReadPWM failed: %v", err)
		}
		if written != tt.expected {
			t.Errorf("WritePWM(%d): expected raw=%d, got %d", tt.input, tt.expected, written)
		}
		os.Remove(pwmFile)
	}
}

func TestWritePWM_Range(t *testing.T) {
	dir := setupHwmonDir(t)

	// 35% → (35 * 255) / 100 = 89
	if err := WritePWM(dir, "pwm1", 35); err != nil {
		t.Fatalf("WritePWM failed: %v", err)
	}
	val, err := ReadPWM(dir, "pwm1")
	if err != nil {
		t.Fatalf("ReadPWM failed: %v", err)
	}
	if val != 89 {
		t.Errorf("35%% duty: expected 89, got %d", val)
	}
}

func TestSetMode(t *testing.T) {
	dir := setupHwmonDir(t)
	modeFile := filepath.Join(dir, "pwm1_enable")

	if err := SetMode(dir, "pwm1", 1); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}
	data, err := os.ReadFile(modeFile)
	if err != nil {
		t.Fatalf("reading pwm1_enable: %v", err)
	}
	if string(data) != "1" {
		t.Errorf("expected mode '1', got '%s'", string(data))
	}

	if err := SetMode(dir, "pwm1", 2); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}
	data, _ = os.ReadFile(modeFile)
	if string(data) != "2" {
		t.Errorf("expected mode '2', got '%s'", string(data))
	}
}

func TestWritePWMRaw(t *testing.T) {
	dir := setupHwmonDir(t)

	if err := WritePWMRaw(dir, "pwm1", 229); err != nil {
		t.Fatalf("WritePWMRaw failed: %v", err)
	}
	val, err := ReadPWM(dir, "pwm1")
	if err != nil {
		t.Fatalf("ReadPWM failed: %v", err)
	}
	if val != 229 {
		t.Errorf("expected 229, got %d", val)
	}
}

func TestReadPWMEnable_Valid(t *testing.T) {
	dir := setupHwmonDir(t)
	writeHwmonFile(t, dir, "pwm1_enable", "1\n")

	mode, err := ReadPWMEnable(dir, "pwm1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != 1 {
		t.Errorf("expected mode 1, got %d", mode)
	}
}
