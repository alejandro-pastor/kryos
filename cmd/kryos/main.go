// kryos: lightweight fan/pump control for NZXT Kraken Z3/2023/2024 on Linux.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alejandro-pastor/kryos/internal"
)

var (
	version = "dev"
	commit  = "none"
)

// CLI flags
var (
	flagStatus     = flag.Bool("status", false, "show current status (CPU, liquid, RPM, duty, curve levels)")
	flagOnce       = flag.Bool("once", false, "run one control cycle and exit (systemd mode)")
	flagSetPump    = flag.Int("set-pump", -1, "one-shot: force pump to N% via pwm1, then exit")
	flagSetFan     = flag.Int("set-fan", -1, "one-shot: force fan to N% via pwm2, then exit")
	flagDryRun     = flag.Bool("dry-run", false, "")
	flagGetState   = flag.Bool("get-state", false, "print <pump_lvl> <fan_lvl> <cpu_temp> <liquid_temp> (machine-parseable)")
	flagInstall    = flag.Bool("install", false, "install kryos.service and kryos.timer, enable and start")
	flagUninstall  = flag.Bool("uninstall", false, "remove kryos.service and kryos.timer")
	flagStatePath  = flag.String("state", internal.DefaultStatePath, "")
	flagVerbose    = flag.Bool("verbose", false, "")
	flagShowVer    = flag.Bool("version", false, "print version and commit hash")
	flagShowHelp   = flag.Bool("help", false, "show this help")
	flagPrintAliases = flag.Bool("print-aliases", false, "print the kryos() shell function for ~/.bashrc")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: kryos [flags]\n\nFlags:\n")
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name == "state" || f.Name == "verbose" || f.Name == "dry-run" {
				return
			}
			fmt.Fprintf(os.Stderr, "  --%-12s %s\n", f.Name, f.Usage)
		})
	}
}

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
	case *flagInstall:
		exitOnErr(runInstall())
	case *flagUninstall:
		exitOnErr(runUninstall())
	case *flagPrintAliases:
		runPrintAliases()
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

// verboseLog prints to stderr only when --verbose is active.
func verboseLog(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// runPrintAliases prints the kryos() shell function for ~/.bashrc.
func runPrintAliases() {
	fmt.Print(aliasesScript)
}

const aliasesScript = `# === KryOs shell function ===
# Add this to your ~/.bashrc by running:
#   kryos --print-aliases >> ~/.bashrc
# Then reload with:
#   source ~/.bashrc
#
# Or manually copy this block into your shell configuration.
# ============================

# KryOs - NZXT Kraken controller
kryos() {
  case "$1" in
    status) sudo /usr/local/bin/kryos --status ;;
    logs)   sudo journalctl -u kryos.service -n 15 --no-pager ;;
    watch)  watch -n 10 sudo /usr/local/bin/kryos --status ;;
    test)   sudo /usr/local/bin/kryos-test ;;
    state)  sudo /usr/local/bin/kryos --get-state ;;
    freq)   watch -n 1 "grep MHz /proc/cpuinfo" ;;
    help)
      echo "Usage: kryos <command>"
      echo
      echo "Commands:"
      echo "  status   Show current CPU/liquid temp, RPM and duty levels"
      echo "  logs     Show last 15 log lines from the systemd service"
      echo "  watch    Live monitor, updates every 10s (Ctrl+C to stop)"
      echo "  test     Run 5-minute CPU stress test with monitoring"
      echo "  state    Print machine-parseable output (pump_lvl fan_lvl cpu liquid)"
      echo "  freq     Show CPU core frequencies in real time (updates every 1s)"
      echo "  help     Show this help"
      echo
      echo "Flags: use kryos --help for binary flags (--set-pump, --set-fan, etc.)"
      ;;
    *) sudo /usr/local/bin/kryos "$@" ;;
  esac
}
`

// buildBar generates a visual bar for the current level, e.g. [35%|45%|▶65%|90%].
func buildBar(duties []int, current int) string {
	parts := make([]string, len(duties))
	for i, d := range duties {
		if i == current {
			parts[i] = fmt.Sprintf("▶%d%%", d)
		} else {
			parts[i] = fmt.Sprintf("%d%%", d)
		}
	}
	return "[" + strings.Join(parts, "|") + "]"
}

// runStatus detects hwmon, reads temperatures and RPM, prints human-readable status.
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
		return fmt.Errorf("reading liquid temp: %w", err)
	}
	cpu, err := internal.ReadTemp(cpuPath, "temp1_input")
	if err != nil {
		return fmt.Errorf("reading CPU temp: %w", err)
	}
	pumpRPM, _ := internal.ReadRPM(krakenPath, "fan1_input")
	fanRPM, _ := internal.ReadRPM(krakenPath, "fan2_input")
	pumpPct, _ := internal.ReadPWMPercent(krakenPath, "pwm1")
	fanPct, _ := internal.ReadPWMPercent(krakenPath, "pwm2")

	prev, _ := internal.Load(statePath)

	fmt.Printf("KryOs %s (%s)\n", version, commit)
	fmt.Printf("Kraken: %s at %s\n", krakenName, krakenPath)
	fmt.Printf("CPU:    %.1f\u00b0C (sensor at %s)\n", cpu, cpuPath)
	fmt.Printf("Liquid: %.1f\u00b0C\n", liquid)
	fmt.Printf("Pump:   %d RPM @ %d%%\n", pumpRPM, pumpPct)
	fmt.Printf("Fan:    %d RPM @ %d%%\n", fanRPM, fanPct)
	fmt.Println()
	fmt.Printf("Pump %s\n", buildBar(internal.PumpDutyByLevel[:], prev.Pump))
	fmt.Printf("Fan  %s\n", buildBar(internal.FanDutyByLevel[:], prev.Fan))
	verboseLog(verbose, "thresholds: pump CPU %v / fan liquid %v", internal.PumpCPUThresholds, internal.FanLiquidThresholds)
	return nil
}

// runOnce runs one control cycle: reads temps, applies hysteresis, writes PWM, persists state.
func runOnce(statePath string, dryRun, verbose bool) error {
	krakenPath, _, err := internal.FindKrakenSensor()
	if err != nil {
		return err
	}
	cpuPath, err := internal.FindCPUSensor()
	if err != nil {
		return err
	}

	liquid, err := internal.ReadTemp(krakenPath, "temp1_input")
	if err != nil || liquid <= 0 {
		verboseLog(verbose, "invalid liquid temp (%.1f, err=%v), keeping current state", liquid, err)
		return nil
	}
	cpu, err := internal.ReadTemp(cpuPath, "temp1_input")
	if err != nil || cpu <= 0 {
		verboseLog(verbose, "invalid CPU temp (%.1f, err=%v), keeping current state", cpu, err)
		return nil
	}

	prev, _ := internal.Load(statePath)
	levels := internal.Compute(cpu, liquid, prev)

	pumpPct := internal.PumpDutyByLevel[levels.Pump]
	fanPct := internal.FanDutyByLevel[levels.Fan]

	verboseLog(verbose, "cpu=%.1f liquid=%.1f prev=%d/%d -> %d/%d (pump=%d%% fan=%d%%)",
		cpu, liquid, prev.Pump, prev.Fan, levels.Pump, levels.Fan, pumpPct, fanPct)

	if dryRun {
		fmt.Printf("dry-run: pump=%d%% fan=%d%% (pump_lvl=%d fan_lvl=%d)\n", pumpPct, fanPct, levels.Pump, levels.Fan)
		if err := internal.Save(statePath, levels); err != nil {
			return fmt.Errorf("saving state dry-run: %w", err)
		}
		return nil
	}

	if levels.Pump != prev.Pump {
		prevPumpMode, _ := internal.ReadPWMEnable(krakenPath, "pwm1")
		if err := internal.SetMode(krakenPath, "pwm1", 1); err != nil {
			return fmt.Errorf("set pump mode: %w", err)
		}
		if err := internal.WritePWM(krakenPath, "pwm1", pumpPct); err != nil {
			// Restore mode if write fails — avoid inconsistent state.
			internal.SetMode(krakenPath, "pwm1", prevPumpMode)
			return fmt.Errorf("write pump pwm: %w", err)
		}
	}
	if levels.Fan != prev.Fan {
		prevFanMode, _ := internal.ReadPWMEnable(krakenPath, "pwm2")
		if err := internal.SetMode(krakenPath, "pwm2", 1); err != nil {
			return fmt.Errorf("set fan mode: %w", err)
		}
		if err := internal.WritePWM(krakenPath, "pwm2", fanPct); err != nil {
			internal.SetMode(krakenPath, "pwm2", prevFanMode)
			return fmt.Errorf("write fan pwm: %w", err)
		}
	}

	if err := internal.Save(statePath, levels); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	return nil
}

// runSetPump forces pump to N% via pwm1.
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

// runSetFan forces fan to N% via pwm2.
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

// runGetState prints <pump_lvl> <fan_lvl> <cpu_temp> <liquid_temp> in one line,
// machine-parseable. Exit 0 = success, exit 1 = read error.
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

// checkSystemd verifies that systemctl is available.
func checkSystemd() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemd not detected. KryOs requires systemd for timer-based operation. " +
			"To install manually: copy kryos to /usr/local/bin and schedule via cron or your init system")
	}
	return nil
}

// runInstall extracts kryos.service and kryos.timer from embed.FS, runs daemon-reload,
// enables and starts the timer.
func runInstall() error {
	if os.Geteuid() != 0 {
		return errors.New("--install requires root (sudo kryos --install)")
	}
	if err := checkSystemd(); err != nil {
		return err
	}

	for _, name := range []string{"kryos.service", "kryos.timer"} {
		data, err := internal.SystemdFS.ReadFile("systemd/" + name)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", name, err)
		}
		dest := "/etc/systemd/system/" + name
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}
	}

	// Install the stress test script (shorter name for terminal use)
	data, err := internal.ScriptsFS.ReadFile("scripts/kryos-stress-test.sh")
	if err != nil {
		return fmt.Errorf("reading embedded test script: %w", err)
	}
	if err := os.WriteFile("/usr/local/bin/kryos-test", data, 0755); err != nil {
		return fmt.Errorf("writing kryos-test: %w", err)
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if err := exec.Command("systemctl", "enable", "kryos.timer").Run(); err != nil {
		return fmt.Errorf("systemctl enable: %w", err)
	}
	if err := exec.Command("systemctl", "restart", "kryos.timer").Run(); err != nil {
		return fmt.Errorf("systemctl restart: %w", err)
	}
	return nil
}

// runUninstall stops and disables the timer, removes the unit files.
func runUninstall() error {
	if os.Geteuid() != 0 {
		return errors.New("--uninstall requires root (sudo kryos --uninstall)")
	}
	if err := checkSystemd(); err != nil {
		return err
	}

	// Errors here are non-fatal: the timer may not be enabled.
	_ = exec.Command("systemctl", "disable", "--now", "kryos.timer").Run()
	_ = exec.Command("systemctl", "stop", "kryos.timer").Run()

	for _, name := range []string{"kryos.service", "kryos.timer"} {
		dest := "/etc/systemd/system/" + name
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", dest, err)
		}
	}
	// Remove the installed stress test script (non-fatal if absent)
	_ = os.Remove("/usr/local/bin/kryos-test")

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	return nil
}
