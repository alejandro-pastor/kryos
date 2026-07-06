// Package internal contiene la lógica pura de KryOs:
// acceso a sysfs, histéresis, persistencia de estado y unidades systemd.
// No expone nada fuera del binario.
package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Nombres de hwmon aceptados para cualquier Kraken Z3/2023/2024.
// El driver nzxt-kraken3 expone el modelo (z53/z63/z73) en kernel 6.9-6.10
// y "kraken3" en 6.10+. Ambos se aceptan; no necesitamos distinguir el modelo.
var krakenHwmonNames = []string{"z53", "z63", "z73", "kraken3"}

// Nombres de hwmon aceptados para el sensor de CPU.
var cpuHwmonNames = []string{"k10temp", "coretemp"}

// Errores exportados para que los handlers CLI produzcan mensajes claros.
var (
	ErrKrakenNotFound = errors.New("ningún Kraken Z3/2023/2024 detectado en hwmon (¿cargaste el driver nzxt-kraken3?)")
	ErrCPUNotFound    = errors.New("ningún sensor de CPU detectado (k10temp/coretemp)")
)

// findHwmonByName escanea /sys/class/hwmon/hwmon*/name y devuelve el path
// base del primer hwmon cuyo name coincida. Devuelve "" si no hay match.
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

// FindKrakenSensor devuelve el path base del hwmon del Kraken y el nombre
// real detectado (z53, z63, z73, kraken3) para reportar en --status.
func FindKrakenSensor() (basePath, detectedName string, err error) {
	for _, name := range krakenHwmonNames {
		if path := findHwmonByName(name); path != "" {
			return path, name, nil
		}
	}
	return "", "", ErrKrakenNotFound
}

// FindCPUSensor devuelve el path base del hwmon del sensor de CPU.
func FindCPUSensor() (string, error) {
	for _, name := range cpuHwmonNames {
		if path := findHwmonByName(name); path != "" {
			return path, nil
		}
	}
	return "", ErrCPUNotFound
}

// ReadTemp lee un atributo de temperatura y devuelve el valor en grados Celsius.
// attr suele ser "temp1_input". Algunos k10temp exponen temp2_input (Tccd1)
// y temp3_input (Tccd2); pasar el attr correspondiente en cada caso.
func ReadTemp(path, attr string) (float64, error) {
	data, err := os.ReadFile(filepath.Join(path, attr))
	if err != nil {
		return 0, err
	}
	raw, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parseando %s: %w", attr, err)
	}
	return float64(raw) / 1000.0, nil
}

// ReadRPM lee fan1_input o fan2_input y devuelve las revoluciones por minuto.
func ReadRPM(path, fan string) (int, error) {
	data, err := os.ReadFile(filepath.Join(path, fan))
	if err != nil {
		return 0, err
	}
	rpm, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parseando %s: %w", fan, err)
	}
	return rpm, nil
}

// ReadPWM devuelve el valor raw 0-255 de pwm1 o pwm2.
// Usar cuando necesitas el valor exacto (ej: restauración bit-exacta en --calibrate).
func ReadPWM(path, pwm string) (int, error) {
	data, err := os.ReadFile(filepath.Join(path, pwm))
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parseando %s: %w", pwm, err)
	}
	return val, nil
}

// ReadPWMPercent devuelve el duty cycle como porcentaje 0-100.
// Cálculo: (raw * 100) / 255. Usar cuando quieres mostrar al usuario.
func ReadPWMPercent(path, pwm string) (int, error) {
	raw, err := ReadPWM(path, pwm)
	if err != nil {
		return 0, err
	}
	return (raw * 100) / 255, nil
}

// ReadPWMEnable lee pwm1_enable o pwm2_enable: 0=off, 1=manual, 2=curve.
func ReadPWMEnable(path, pwm string) (int, error) {
	attr := pwm + "_enable"
	data, err := os.ReadFile(filepath.Join(path, attr))
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parseando %s: %w", attr, err)
	}
	return val, nil
}

// WritePWM escribe un duty cycle como porcentaje 0-100 usando truncamiento
// (no redondeo). Para percent=90 da 229 (90*255/100), NO 230.
// Esto es deliberado: replica la operación `duty * 255 // 100` de liquidctl
// (línea 282 de kraken3.py), garantizando paridad bit-exacta con el bash
// actual. Para los valores del plan (25/35/45/50/65/70/90) los resultados
// coinciden: 63, 89, 114, 127, 165, 178, 229.
func WritePWM(path, pwm string, percent int) error {
	pwmDuty := (percent * 255) / 100
	return os.WriteFile(filepath.Join(path, pwm), []byte(strconv.Itoa(pwmDuty)), 0644)
}

// WritePWMRaw escribe un valor raw 0-255 directo.
// Usar cuando necesitas restaurar un valor exacto leído con ReadPWM.
func WritePWMRaw(path, pwm string, raw int) error {
	return os.WriteFile(filepath.Join(path, pwm), []byte(strconv.Itoa(raw)), 0644)
}

// SetMode escribe pwm1_enable o pwm2_enable: 0=off, 1=manual, 2=curve.
func SetMode(path, pwm string, mode int) error {
	return os.WriteFile(filepath.Join(path, pwm+"_enable"), []byte(strconv.Itoa(mode)), 0644)
}
