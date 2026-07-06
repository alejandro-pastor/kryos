// kryos: lightweight fan/pump control for NZXT Kraken Z3/2023/2024 on Linux.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alejandro-pastor/kryos/internal"
)

var (
	version = "dev"
	commit  = "none"
)

// CLI flags
var (
	flagStatus     = flag.Bool("status", false, "imprime estado actual (CPU, líquido, RPM, duty, hwmon detectado)")
	flagOnce       = flag.Bool("once", false, "ejecuta el ciclo de control y sale (modo systemd)")
	flagDryRun     = flag.Bool("dry-run", false, "imprime acción planeada, NO escribe sysfs")
	flagSetPump    = flag.Int("set-pump", -1, "one-shot: fuerza bomba a N% vía pwm1 directo, sale")
	flagSetFan     = flag.Int("set-fan", -1, "one-shot: fuerza fan a N% vía pwm2 directo, sale")
	flagGetState   = flag.Bool("get-state", false, "imprime <pump_lvl> <fan_lvl> <cpu_temp> <liquid_temp> en una línea (machine-parseable)")
	flagCalibrate  = flag.Bool("calibrate", false, "compara pwm directo vs liquidctl en 3 puntos (35/65/90)")
	flagInstall    = flag.Bool("install", false, "instala kryos.service y kryos.timer, los habilita")
	flagUninstall  = flag.Bool("uninstall", false, "desinstala kryos.service y kryos.timer")
	flagStatePath  = flag.String("state", internal.DefaultStatePath, "ruta del state file")
	flagVerbose    = flag.Bool("verbose", false, "más info a stderr")
	flagShowVer    = flag.Bool("version", false, "imprime versión + commit hash")
	flagShowHelp   = flag.Bool("help", false, "muestra esta ayuda")
)

func main() {
	flag.Parse()
	if *flagShowHelp {
		flag.Usage()
		return
	}
	if *flagShowVer {
		fmt.Printf("kryos %s (commit %s)\n", version, commit)
		return
	}

	switch {
	case *flagStatus:
		exitOnErr(runStatus(*flagStatePath, *flagVerbose))
	case *flagOnce:
		exitOnErr(runOnce(*flagStatePath, *flagDryRun, *flagVerbose))
	case *flagSetPump >= 0:
		exitOnErr(runSetPump(*flagSetPump))
	case *flagSetFan >= 0:
		exitOnErr(runSetFan(*flagSetFan))
	case *flagGetState:
		exitOnErr(runGetState(*flagStatePath))
	case *flagCalibrate:
		exitOnErr(runCalibrate())
	case *flagInstall:
		exitOnErr(runInstall())
	case *flagUninstall:
		exitOnErr(runUninstall())
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// verboseLog solo imprime si --verbose está activo.
func verboseLog(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// runStatus detecta hwmon, lee temperaturas y RPM, imprime estado legible.
func runStatus(statePath string, verbose bool) error {
	krakenPath, krakenName, err := internal.FindKrakenSensor()
	if err != nil {
		return err
	}
	cpuPath, err := internal.FindCPUSensor()
	if err != nil {
		return err
	}

	liquid, err := internal.ReadTemp(krakenPath, "temp1_input")
	if err != nil {
		return fmt.Errorf("leyendo líquido: %w", err)
	}
	cpu, err := internal.ReadTemp(cpuPath, "temp1_input")
	if err != nil {
		return fmt.Errorf("leyendo CPU: %w", err)
	}
	pumpRPM, _ := internal.ReadRPM(krakenPath, "fan1_input")
	fanRPM, _ := internal.ReadRPM(krakenPath, "fan2_input")
	pumpPct, _ := internal.ReadPWMPercent(krakenPath, "pwm1")
	fanPct, _ := internal.ReadPWMPercent(krakenPath, "pwm2")

	prev, _ := internal.Load(statePath)

	fmt.Printf("Kraken: %s en %s\n", krakenName, krakenPath)
	fmt.Printf("CPU:    %.1f°C (sensor en %s)\n", cpu, cpuPath)
	fmt.Printf("Liquid: %.1f°C\n", liquid)
	fmt.Printf("Pump:   %d RPM @ %d%%\n", pumpRPM, pumpPct)
	fmt.Printf("Fan:    %d RPM @ %d%%\n", fanRPM, fanPct)
	fmt.Printf("State:  pump_lvl=%d fan_lvl=%d\n", prev.Pump, prev.Fan)
	verboseLog(verbose, "thresholds: pump CPU %v / fan liquid %v", internal.PumpCPUThresholds, internal.FanLiquidThresholds)
	return nil
}

// runOnce ejecuta el ciclo de control: lee temperaturas, aplica histéresis,
// escribe pwm, persiste estado.
func runOnce(statePath string, dryRun, verbose bool) error {
	krakenPath, krakenName, err := internal.FindKrakenSensor()
	if err != nil {
		return err
	}
	cpuPath, err := internal.FindCPUSensor()
	if err != nil {
		return err
	}

	liquid, err := internal.ReadTemp(krakenPath, "temp1_input")
	if err != nil || liquid <= 0 {
		// Política: lectura inválida → no tocar pwm, dejar último estado, exit 0
		verboseLog(verbose, "liquid temp inválida (%.1f, err=%v), conservando estado", liquid, err)
		return nil
	}
	cpu, err := internal.ReadTemp(cpuPath, "temp1_input")
	if err != nil || cpu <= 0 {
		verboseLog(verbose, "CPU temp inválida (%.1f, err=%v), conservando estado", cpu, err)
		return nil
	}

	prev, _ := internal.Load(statePath)
	levels := internal.Compute(cpu, liquid, prev)

	pumpPct := internal.PumpDutyByLevel[levels.Pump]
	fanPct := internal.FanDutyByLevel[levels.Fan]

	verboseLog(verbose, "cpu=%.1f liquid=%.1f prev=%d/%d -> %d/%d (pump=%d%% fan=%d%%)",
		cpu, liquid, prev.Pump, prev.Fan, levels.Pump, levels.Fan, pumpPct, fanPct)

	if dryRun {
		// Dry-run: imprime lo que habría hecho y guarda state (sin escribir pwm).
		// Guardar state permite comparar trayectorias con el bash real en A/B tests.
		fmt.Printf("dry-run: pump=%d%% fan=%d%% (pump_lvl=%d fan_lvl=%d)\n", pumpPct, fanPct, levels.Pump, levels.Fan)
		if err := internal.Save(statePath, levels); err != nil {
			return fmt.Errorf("guardando state dry-run: %w", err)
		}
		return nil
	}

	if levels.Pump != prev.Pump {
		if err := internal.SetMode(krakenPath, "pwm1", 1); err != nil {
			return fmt.Errorf("set pump mode: %w", err)
		}
		if err := internal.WritePWM(krakenPath, "pwm1", pumpPct); err != nil {
			return fmt.Errorf("write pump pwm: %w", err)
		}
	}
	if levels.Fan != prev.Fan {
		if err := internal.SetMode(krakenPath, "pwm2", 1); err != nil {
			return fmt.Errorf("set fan mode: %w", err)
		}
		if err := internal.WritePWM(krakenPath, "pwm2", fanPct); err != nil {
			return fmt.Errorf("write fan pwm: %w", err)
		}
	}

	if err := internal.Save(statePath, levels); err != nil {
		return fmt.Errorf("guardando state: %w", err)
	}
	_ = krakenName
	return nil
}

// runSetPump fuerza la bomba a N% en pwm1 directo.
func runSetPump(percent int) error {
	krakenPath, _, err := internal.FindKrakenSensor()
	if err != nil {
		return err
	}
	if err := internal.SetMode(krakenPath, "pwm1", 1); err != nil {
		return err
	}
	return internal.WritePWM(krakenPath, "pwm1", percent)
}

// runSetFan fuerza el fan a N% en pwm2 directo.
func runSetFan(percent int) error {
	krakenPath, _, err := internal.FindKrakenSensor()
	if err != nil {
		return err
	}
	if err := internal.SetMode(krakenPath, "pwm2", 1); err != nil {
		return err
	}
	return internal.WritePWM(krakenPath, "pwm2", percent)
}

// runGetState imprime <pump_lvl> <fan_lvl> <cpu_temp> <liquid_temp> en una
// línea, machine-parseable. Exit 0 = éxito, exit 1 = fallo de lectura.
func runGetState(statePath string) error {
	krakenPath, _, err := internal.FindKrakenSensor()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	cpuPath, err := internal.FindCPUSensor()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	liquid, err := internal.ReadTemp(krakenPath, "temp1_input")
	if err != nil || liquid <= 0 {
		fmt.Fprintln(os.Stderr, "failed to read liquid temperature")
		return err
	}
	cpu, err := internal.ReadTemp(cpuPath, "temp1_input")
	if err != nil || cpu <= 0 {
		fmt.Fprintln(os.Stderr, "failed to read CPU temperature")
		return err
	}

	prev, _ := internal.Load(statePath)
	levels := internal.Compute(cpu, liquid, prev)

	fmt.Printf("%d %d %.1f %.1f\n", levels.Pump, levels.Fan, cpu, liquid)
	return nil
}

// runCalibrate compara pwm directo vs liquidctl en 3 puntos (35/65/90).
// Restaura el estado inicial al terminar (incluso en error) con defer.
func runCalibrate() error {
	if os.Geteuid() != 0 {
		return errors.New("--calibrate requiere ejecutarse como root (sudo kryos --calibrate)")
	}

	krakenPath, _, err := internal.FindKrakenSensor()
	if err != nil {
		return err
	}

	initialPumpMode, err := internal.ReadPWMEnable(krakenPath, "pwm1")
	if err != nil {
		return fmt.Errorf("leyendo pwm1_enable inicial: %w", err)
	}
	initialPumpDuty, err := internal.ReadPWM(krakenPath, "pwm1")
	if err != nil {
		return fmt.Errorf("leyendo pwm1 inicial: %w", err)
	}
	initialFanMode, err := internal.ReadPWMEnable(krakenPath, "pwm2")
	if err != nil {
		return fmt.Errorf("leyendo pwm2_enable inicial: %w", err)
	}
	initialFanDuty, err := internal.ReadPWM(krakenPath, "pwm2")
	if err != nil {
		return fmt.Errorf("leyendo pwm2 inicial: %w", err)
	}

	defer func() {
		if err := internal.SetMode(krakenPath, "pwm1", initialPumpMode); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: restaurando pwm1_enable a %d: %v\n", initialPumpMode, err)
		}
		if err := internal.WritePWMRaw(krakenPath, "pwm1", initialPumpDuty); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: restaurando pwm1 a %d: %v\n", initialPumpDuty, err)
		}
		if err := internal.SetMode(krakenPath, "pwm2", initialFanMode); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: restaurando pwm2_enable a %d: %v\n", initialFanMode, err)
		}
		if err := internal.WritePWMRaw(krakenPath, "pwm2", initialFanDuty); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: restaurando pwm2 a %d: %v\n", initialFanDuty, err)
		}
	}()

	const threshold = 100
	testDuties := []int{35, 65, 90}
	allPassed := true

	for _, duty := range testDuties {
		if err := internal.SetMode(krakenPath, "pwm1", 1); err != nil {
			return fmt.Errorf("set pump mode: %w", err)
		}
		if err := internal.WritePWM(krakenPath, "pwm1", duty); err != nil {
			return fmt.Errorf("write pump pwm: %w", err)
		}
		time.Sleep(5 * time.Second)
		rpmPWM, err := internal.ReadRPM(krakenPath, "fan1_input")
		if err != nil {
			return fmt.Errorf("leyendo RPM tras pwm directo: %w", err)
		}

		cmd := exec.Command("liquidctl", "--match", "kraken", "set", "pump", "speed", fmt.Sprintf("%d", duty))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("liquidctl failed: %w (%s)", err, string(out))
		}
		time.Sleep(5 * time.Second)
		rpmLC, err := internal.ReadRPM(krakenPath, "fan1_input")
		if err != nil {
			return fmt.Errorf("leyendo RPM tras liquidctl: %w", err)
		}

		diff := rpmPWM - rpmLC
		if diff < 0 {
			diff = -diff
		}
		status := "OK"
		if diff > threshold {
			status = "FAIL"
			allPassed = false
		}
		fmt.Printf("%s duty=%d%% pwm=%d liquidctl=%d diff=%d\n", status, duty, rpmPWM, rpmLC, diff)
	}

	if !allPassed {
		return errors.New("calibración falló en uno o más puntos")
	}
	return nil
}

// runInstall extrae kryos.service y kryos.timer de embed.FS, hace daemon-reload,
// habilita y arranca el timer.
func runInstall() error {
	if os.Geteuid() != 0 {
		return errors.New("--install requiere ejecutarse como root (sudo kryos --install)")
	}

	for _, name := range []string{"kryos.service", "kryos.timer"} {
		data, err := internal.SystemdFS.ReadFile("systemd/" + name)
		if err != nil {
			return fmt.Errorf("leyendo %s embebido: %w", name, err)
		}
		dest := "/etc/systemd/system/" + name
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("escribiendo %s: %w", dest, err)
		}
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if err := exec.Command("systemctl", "enable", "kryos.timer").Run(); err != nil {
		return fmt.Errorf("systemctl enable: %w", err)
	}
	if err := exec.Command("systemctl", "start", "kryos.timer").Run(); err != nil {
		return fmt.Errorf("systemctl start: %w", err)
	}
	return nil
}

// runUninstall detiene y deshabilita el timer, elimina los ficheros unit.
func runUninstall() error {
	if os.Geteuid() != 0 {
		return errors.New("--uninstall requiere ejecutarse como root (sudo kryos --uninstall)")
	}

	// Errores aquí son no-fatales: el timer puede no estar habilitado.
	_ = exec.Command("systemctl", "disable", "--now", "kryos.timer").Run()
	_ = exec.Command("systemctl", "stop", "kryos.timer").Run()

	for _, name := range []string{"kryos.service", "kryos.timer"} {
		dest := "/etc/systemd/system/" + name
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("eliminando %s: %w", dest, err)
		}
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	return nil
}
