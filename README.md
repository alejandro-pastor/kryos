# KryOs

> Lightweight fan/pump control for NZXT Kraken Z3/2023/2024 on Linux.

![CI](https://github.com/alejandro-pastor/kryos/actions/workflows/ci.yml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.26-blue)
![License](https://img.shields.io/badge/license-GPL--3.0-green)
![Release](https://img.shields.io/github/v/release/alejandro-pastor/kryos)

## What it does

The Kraken firmware on Linux doesn't auto-regulate below 60°C. This binary
fills the gap by reading CPU and liquid temperatures from sysfs hwmon and
writing PWM duty cycles with hysteresis. Peak RAM: ~2 MB. No daemon. No
telemetry. No external dependencies.

## Story

KryOs was born from the idea of being able to control my NZXT Kraken liquid
cooling system, since Linux doesn't have its own official software like Windows
does. I looked into existing options, but they did more than I wanted and ran
as background daemons, constantly hogging resources. So, I decided to create
a Bash script to automate and command the cooling system to adjust both the
pump and the fan.

At first, I got it to consume 20 megabytes every 5 seconds. But then I
realized that, since it's a liquid loop, there's no need to constantly poll
it because, well, it uses resources. It was minimal — 100 milliseconds every
5 seconds. Even so, I increased the interval to 10 seconds, which cut the
total number of requests in half (pure math). Yet, it still hit a peak RAM
usage of 23 MB, which felt like too much for me. So, I decided to rewrite it
in Go to make it much lighter.

After a series of improvements, a lot of trial and error, and plenty of
testing, I managed to bring the peak RAM usage down to just 2 megabytes every
10 seconds. On top of that, it is only active for 10 milliseconds, down from
the previous 100 milliseconds. Honestly, this wasn't strictly necessary; it
was just a really fun experiment. But if you're like me and obsessed with
keeping resource consumption as low as possible, this project might be a great
fit for you — as long as your cooling system is supported and you're running
Linux, of course, since Linux is the whole reason I built this.

I should also mention that the official app averages between 200 and 600 MB of
continuous RAM usage. For me, this is a massive improvement, and I feel it
reduces the overall overhead of the cooling system even further. In the end,
between those 10-second intervals, resource consumption is zero. Only during
that 10-millisecond window will it use a mere 2 megabytes on one of your CPU
cores.

If you're like me and get a bit paranoid about sketchy commands or weird stuff
running on your system, don't worry: **zero dependencies**, 100% local. It
doesn't send any data anywhere outside your machine, nobody is going to steal
your information, and it is completely safe and secure. You can verify this
yourself:

```bash
# 1. No external dependencies (zero network libraries)
go list -m all

# 2. No network imports in the codebase
go list -f '{{.Imports}}' ./cmd/kryos

# 3. All I/O is local (sysfs sensors + state files)
grep -rn "ReadFile\|WriteFile" --include="*.go" . | grep -v "_test"
```

Honestly, I'm thrilled with how it turned out, and I hope you like it. If you
run into any issues, please let me know, and I'll try to fix them. Thank you
very much!

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

Pump [▶35%|45%|65%|90%|100%]
Fan  [25%|35%|▶50%|70%|100%]
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
| `kryos --print-aliases` | Print the kryos() shell function for ~/.bashrc |
| `kryos --help` | Show available flags |

## Aliases (optional)

Add this to your `~/.bashrc` for a friendlier CLI:

```bash
kryos() {
  case "$1" in
    status) sudo /usr/local/bin/kryos --status ;;
    logs)   sudo journalctl -u kryos.service -n 15 --no-pager ;;
    watch)  watch -n 10 sudo /usr/local/bin/kryos --status ;;
    test)   sudo /usr/local/bin/kryos-test ;;
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

To add the function to your shell without copying from this README:

```bash
kryos --print-aliases >> ~/.bashrc
source ~/.bashrc
```

This does not modify any files — it only prints to stdout. You decide
whether to redirect it to your shell configuration.

Note: `--install` does not modify your bash configuration. Aliases are
entirely opt-in.

## Run a stress test

If you installed KryOs with `--install`, the stress test script is already
at `/usr/local/bin/kryos-test`. With the alias function above:

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

If liquid temperature reaches **51°C**, both pump and fan are forced to
**100%** regardless of the normal curve (level 4, emergency). Normal
regulation resumes when liquid drops below **46°C** (5°C hysteresis).
This is an additional safety layer below the kernel driver's internal
critical threshold (59°C).

| Parameter | Value | Unit |
|-----------|-------|------|
| Pump CPU thresholds | 55 / 70 / 85 | °C |
| Pump levels | 35 / 45 / 65 / 90 / 100 | % duty |
| Pump hysteresis | 6 | °C |
| Fan liquid thresholds | 34 / 38 / 42 | °C |
| Fan levels | 25 / 35 / 50 / 70 / 100 | % duty |
| Fan hysteresis | 3 | °C |
| Emergency threshold | 51 | °C liquid |
| Emergency hysteresis | 5 | °C |

## Technical decisions

| Need | Chosen | Alternative | Why |
|------|--------|-------------|-----|
| Execution every 10s | **systemd timer** | Continuous daemon | Zero RAM between ticks, systemd handles failures and logging |
| State persistence | **StateDirectory** | RuntimeDirectory | RuntimeDirectory is deleted between oneshot ticks (confirmed and fixed bug) |
| Curve configuration | **Hardcoded** (v1) | TOML config file | Simplicity first; configurable thresholds planned for v0.2.0 |
| PWM write method | **Direct sysfs** | liquidctl CLI | Zero external dependencies, bit-exact parity with liquidctl |
| Safety net | **Driver curve (59°C)** | Software monitoring | Kernel driver already forces 100% at critical temperature |
| Emergency layer | **51°C liquid threshold** | — | Additional safety 8°C below kernel's critical point |

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

## Project status

| Feature | Status |
|---------|--------|
| Pump curve by CPU temperature | ✅ Done |
| Fan curve by liquid temperature | ✅ Done |
| Hysteresis (one level per tick) | ✅ Done |
| systemd timer installation (`--install`) | ✅ Done |
| Visual bar in `--status` | ✅ Done |
| Stress test script (`kryos test`) | ✅ Done |
| `--print-aliases` for bashrc setup | ✅ Done |
| Emergency level (51°C liquid, both to 100%) | ✅ Done |
| Configurable thresholds | 🔜 Planned (v0.2.0) |
| GPU temperature support | 💡 Idea |

## Credits

- Built on top of the in-kernel `nzxt-kraken3` driver (Aleksa Savic, Jonas Malaco, GPL-2.0)
- Protocol reverse-engineering from [liquidctl](https://github.com/liquidctl/liquidctl) (Jonas Malaco and contributors, GPL-3.0)
- Inspired by the philosophy of [suckless](https://suckless.org/) and the simplicity of [fan2go](https://github.com/markasoftware/fan2go)

## Author

**Alejandro Pastor** — [github.com/alejandro-pastor](https://github.com/alejandro-pastor)

## License

KryOs is licensed under the **GNU General Public License v3.0** (GPL-3.0).
See the [LICENSE](LICENSE) file for the full text.

As the sole author, I retain the right to offer alternative licensing terms
if needed. If GPL-3.0 is incompatible with your use case, open an issue and
we can discuss it.
