package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Kraken hwmon names for nzxt-kraken3 driver (kernel 6.9+).
var krakenHwmonNames = []string{"z53", "z63", "z73", "kraken3"}

// CPU hwmon names for AMD/Intel temperature sensors.
var cpuHwmonNames = []string{"k10temp", "coretemp"}

// Sentinel errors for CLI handlers.
var (
	ErrKrakenNotFound = errors.New("no Kraken Z3/2023/2024 found in hwmon (did you load the nzxt-kraken3 driver?)")
	ErrCPUNotFound    = errors.New("no CPU temperature sensor found (k10temp/coretemp)")
)

// findHwmonByName scans /sys/class/hwmon/hwmon*/name and returns the base path
// of the first match. Returns "" if none found.
func findHwmonByName(name string) string {
	matches, err := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	if err != nil {
		return ""
	}
	for _, nameFile := range matches {
		data, err := os.ReadFile(nameFile)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == name {
			return filepath.Dir(nameFile)
		}
	}
	return ""
}

// FindKrakenSensor returns the hwmon base path and the detected name
// (z53, z63, z73, kraken3) for --status output.
func FindKrakenSensor() (basePath, detectedName string, err error) {
	for _, name := range krakenHwmonNames {
		if path := findHwmonByName(name); path != "" {
			return path, name, nil
		}
	}
	return "", "", ErrKrakenNotFound
}

// FindCPUSensor returns the hwmon base path for the CPU temperature sensor.
func FindCPUSensor() (string, error) {
	for _, name := range cpuHwmonNames {
		if path := findHwmonByName(name); path != "" {
			return path, nil
		}
	}
	return "", ErrCPUNotFound
}

// ReadTemp reads a temperature attribute and returns the value in Celsius.
// attr is typically "temp1_input". Some k10temp expose temp2_input (Tccd1)
// and temp3_input (Tccd2); pass the appropriate attr for each case.
func ReadTemp(path, attr string) (float64, error) {
	data, err := os.ReadFile(filepath.Join(path, attr))
	if err != nil {
		return 0, err
	}
	raw, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", attr, err)
	}
	return float64(raw) / 1000.0, nil
}

// ReadRPM reads fan1_input or fan2_input and returns RPM.
func ReadRPM(path, fan string) (int, error) {
	data, err := os.ReadFile(filepath.Join(path, fan))
	if err != nil {
		return 0, err
	}
	rpm, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", fan, err)
	}
	return rpm, nil
}

// ReadPWM returns the raw 0-255 value from pwm1 or pwm2.
func ReadPWM(path, pwm string) (int, error) {
	data, err := os.ReadFile(filepath.Join(path, pwm))
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", pwm, err)
	}
	return val, nil
}

// ReadPWMPercent returns the duty cycle as 0-100 percentage.
func ReadPWMPercent(path, pwm string) (int, error) {
	raw, err := ReadPWM(path, pwm)
	if err != nil {
		return 0, err
	}
	return (raw * 100) / 255, nil
}

// ReadPWMEnable reads pwm1_enable or pwm2_enable: 0=off, 1=manual, 2=curve.
func ReadPWMEnable(path, pwm string) (int, error) {
	attr := pwm + "_enable"
	data, err := os.ReadFile(filepath.Join(path, attr))
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", attr, err)
	}
	return val, nil
}

// WritePWM writes a 0-100 percentage duty cycle using truncation (not rounding).
// This matches liquidctl's `duty * 255 // 100` for bit-exact parity.
func WritePWM(path, pwm string, percent int) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	pwmDuty := (percent * 255) / 100
	return os.WriteFile(filepath.Join(path, pwm), []byte(strconv.Itoa(pwmDuty)), 0644)
}

// WritePWMRaw writes a raw 0-255 value directly.
func WritePWMRaw(path, pwm string, raw int) error {
	return os.WriteFile(filepath.Join(path, pwm), []byte(strconv.Itoa(raw)), 0644)
}

// SetMode writes pwm1_enable or pwm2_enable: 0=off, 1=manual, 2=curve.
func SetMode(path, pwm string, mode int) error {
	return os.WriteFile(filepath.Join(path, pwm+"_enable"), []byte(strconv.Itoa(mode)), 0644)
}
