# KryOs

Lightweight fan/pump control for NZXT Kraken Z3/2023/2024 on Linux.

## What it does

The Kraken firmware on Linux doesn't auto-regulate below 60°C. This binary
fills the gap by reading CPU and liquid temperatures from sysfs hwmon and
writing PWM duty cycles with hysteresis. Peak RAM: ~2 MB. No daemon. No
telemetry. No external dependencies.

## Supported devices

| Model | PID | Radiator | Status |
|-------|-----|----------|--------|
| Kraken Z53 | 0x3008 | 240mm | Tested |
| Kraken Z63 | 0x3008 | 280mm | Should work |
| Kraken Z73 | 0x3008 | 360mm | Should work |
| Kraken 2023 Standard | 0x300E | 240/280/360mm | Should work |
| Kraken 2023 Elite | 0x300C | 240/280/360mm | Should work |
| Kraken 2024 Elite RGB | 0x3012 | 240/280/360mm | Should work |
| Kraken 2024 Plus | 0x3014 | 240/280/360mm | Should work |
| Kraken X53/X63/X73 | 0x2007/0x2014 | various | Partial (pump only) |
| Kraken X42/X52/X62/X72 | various | various | Not supported (different driver) |

## Requirements

- Linux kernel ≥ 6.9 (for `nzxt-kraken3` upstream driver)
- Load the driver: `sudo modprobe nzxt-kraken3`
- Root access (to write PWM sysfs)

## Quick start

```bash
# 1. Download the binary
wget https://github.com/alejandro-pastor/kryos/releases/latest/download/kryos
chmod +x kryos
sudo mv kryos /usr/local/bin/kryos

# 2. Verify it detects your Kraken
sudo kryos --status

# 3. Install as a systemd service (runs every 10s)
sudo kryos --install
```

Expected output from `--status`:

```
KryOs 0.1.0 (581a80f)
Kraken: z53 at /sys/class/hwmon/hwmon5
CPU:    56.2°C (sensor at /sys/class/hwmon/hwmon4)
Liquid: 35.9°C
Pump:   1342 RPM @ 34%
Fan:    1129 RPM @ 50%

Pump [▶35%|45%|65%|90%]
Fan  [25%|35%|▶50%|70%]
```

The arrow (▶) marks the current duty level.

## Usage

| Command | Description |
|---------|-------------|
| `kryos --status` | Show current CPU/liquid temp, RPM, duty, and curve levels |
| `kryos --once` | Run one control cycle (used internally by systemd) |
| `kryos --set-pump 60` | Force pump to 60% (one-shot) |
| `kryos --set-fan 50` | Force fan to 50% (one-shot) |
| `kryos --get-state` | Print machine-parseable state: `pump_lvl fan_lvl cpu_temp liquid_temp` |
| `kryos --install` | Install and enable systemd service + timer |
| `kryos --uninstall` | Stop and remove systemd units |
| `kryos --version` | Print version and commit hash |
| `kryos --help` | Show available flags |

## Aliases (optional)

Add this to your `~/.bashrc` for a friendlier CLI:

```bash
kryos() {
  case "$1" in
    status) sudo /usr/local/bin/kryos --status ;;
    logs)   sudo journalctl -u kryos.service -n 15 --no-pager ;;
    watch)  watch -n 10 sudo /usr/local/bin/kryos --status ;;
    test)   sudo /usr/local/bin/kryos-stress-test.sh ;;
    state)  sudo /usr/local/bin/kryos --get-state ;;
    help)
      echo "Usage: kryos <command>"
      echo
      echo "Commands:"
      echo "  status   Show current CPU/liquid temp, RPM and duty levels"
      echo "  logs     Show last 15 log lines from the systemd service"
      echo "  watch    Live monitor, updates every 10s (Ctrl+C to stop)"
      echo "  test     Run 5-minute CPU stress test with monitoring"
      echo "  state    Print machine-parseable output (pump_lvl fan_lvl cpu liquid)"
      echo "  help     Show this help"
      echo
      echo "Flags: use kryos --help for binary flags (--set-pump, --set-fan, etc.)"
      ;;
    *) sudo /usr/local/bin/kryos "$@" ;;
  esac
}
```

Then use it like:

```bash
kryos status       # show current state
kryos watch        # live monitor every 10s
kryos logs         # last 15 log lines
kryos test         # run 5min CPU stress test
kryos state        # machine-parseable output
kryos help         # show commands
kryos --set-pump 60  # regular flags still work
```

Note: `--install` does not modify your bash configuration. Aliases are
entirely opt-in.

## Run a stress test

If you installed KryOs with `--install`, the stress test script is already
at `/usr/local/bin/kryos-stress-test.sh`. With the alias function above:

```bash
kryos test
```

Or run it directly:

```bash
sudo kryos-test
```

This runs `stress-ng` for 5 minutes while monitoring KryOs every 10s.
Press Ctrl+C to stop early — stress-ng will be cleaned up automatically.
At the end it shows a summary with peak temperatures and maximum levels
reached.

## Uninstallation

```bash
sudo kryos --uninstall
sudo rm /usr/local/bin/kryos
```

## Troubleshooting

- **"no Kraken detected"** → load the driver: `sudo modprobe nzxt-kraken3`
- **"permission denied"** → run as root or check that the service has `User=root`
- **"pump duty not changing"** → initialize the firmware: `sudo liquidctl initialize all`
- **Timer stopped after suspend** → `sudo systemctl restart kryos.timer`

## How it works

The Kraken firmware only does safety regulation (100% pump at 60°C+).
This binary:

- Reads CPU temperature from k10temp/coretemp sysfs
- Reads liquid temperature from the Kraken hwmon driver
- Applies hysteresis (pump reacts to CPU, fan reacts to liquid)
- Writes PWM duty cycles via systemd timer every 10s

The hysteresis algorithm moves **one level per tick** (no jumps) with
rise priority over fall, matching the original bash implementation.

| Parameter | Value | Unit |
|-----------|-------|------|
| Pump CPU thresholds | 55 / 70 / 85 | °C |
| Pump levels | 35 / 45 / 65 / 90 | % duty |
| Pump hysteresis | 6 | °C |
| Fan liquid thresholds | 34 / 38 / 42 | °C |
| Fan levels | 25 / 35 / 50 / 70 | % duty |
| Fan hysteresis | 3 | °C |

## Memory comparison

| Solution | Peak RAM | Resident (between ticks) |
|----------|----------|--------------------------|
| NZXT CAM (Windows) | 300-600 MB | 300-600 MB |
| CoolerControl | 50-80 MB | 50-80 MB |
| fan2go | 15-20 MB | 15-20 MB |
| kryos (this) | ~2 MB | 0 MB |

## Development tools

Scripts in `scripts/` are intended for development and testing:

| Script | Purpose |
|--------|---------|
| `kryos-stress-test.sh` | CPU stress + KryOs monitor with summary |
| `kryos-dryrun.sh` | Simulate one tick without writing PWM (A/B testing) |
| `kryos-calibrate.sh` | Compare PWM vs liquidctl at 35/65/90% |
| `kryos-ab-monitor.sh` | A/B comparison with the original bash script |
| `kryos-ab-monitor-stress.sh` | A/B comparison under CPU load |
| `install-dryrun.sh` | Install dry-run systemd units for A/B testing |

## Credits

- Built on top of the in-kernel `nzxt-kraken3` driver (Aleksa Savic, Jonas Malaco, GPL-2.0)
- Protocol reverse-engineering from [liquidctl](https://github.com/liquidctl/liquidctl) (Jonas Malaco and contributors, GPL-3.0)
- Inspired by the philosophy of [suckless](https://suckless.org/) and the simplicity of [fan2go](https://github.com/markasoftware/fan2go)

## License

GPL-3.0
