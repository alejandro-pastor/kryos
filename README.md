# KryOs

Lightweight fan/pump control for NZXT Kraken Z3/2023/2024 on Linux.

## What it does

The Kraken firmware on Linux doesn't autorregulate below 60°C. This binary
fixes that by reading sysfs and writing PWM duty cycles with hysteresis.
Peak RAM: ~2 MB. No daemon. No telemetry. No dependencies.

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
- The driver loaded: `sudo modprobe nzxt-kraken3`
- root access (to write PWM)

## Installation

1. Download the latest release:
   ```bash
   wget https://github.com/alejandro-pastor/kryos/releases/latest/download/kryos
   chmod +x kryos
   sudo mv kryos /usr/local/bin/kryos
   ```
2. Verify it detects your Kraken:
   ```bash
   sudo kryos --status
   ```
   Expected output: profile name (Z53/Z63/Z73/2023/2024), CPU temp, liquid temp, pump/fan RPM.

3. Run the calibration test to verify that direct PWM writes are equivalent to liquidctl:
   ```bash
   sudo kryos --calibrate
   ```
   This tests 3 points (35%, 65%, 90%) and compares pump RPM via direct PWM vs liquidctl.
   The test takes ~30 seconds and restores the original pump state on exit.
   All 3 points must pass with |diff| ≤ 100 RPM.

4. Install as a systemd service (runs every 10s):
   ```bash
   sudo kryos --install
   ```

## Memory comparison

| Solution | Peak RAM | Resident (between ticks) |
|----------|----------|--------------------------|
| NZXT CAM (Windows) | 300-600 MB | 300-600 MB |
| CoolerControl | 50-80 MB | 50-80 MB |
| fan2go | 15-20 MB | 15-20 MB |
| kryos (this) | ~2 MB | 0 MB |

## Uninstallation

```bash
sudo kryos --uninstall
sudo rm /usr/local/bin/kryos
```

## Troubleshooting

- **"no Kraken detected"** → load the driver: `sudo modprobe nzxt-kraken3`
- **"permission denied"** → run as root or check that the service has `User=root`
- **"pump duty not changing"** → check that the firmware is initialized: `sudo liquidctl initialize all`

## How it works

The Kraken firmware only does safety regulation (100% pump at 60°C+).
This binary:

- Reads CPU temperature from k10temp/coretemp sysfs
- Reads liquid temperature from the Kraken hwmon driver
- Applies hysteresis (pump reacts to CPU, fan reacts to liquid)
- Writes PWM duty cycles every 10s

The hardware delta CPU↔liquid is ~37°C (limit of the 240mm radiator).
This is not a software issue; it's physics.

## Credits

- Built on top of the in-kernel `nzxt-kraken3` driver (Aleksa Savic, Jonas Malaco, GPL-2.0)
- Protocol reverse-engineering from [liquidctl](https://github.com/liquidctl/liquidctl) (Jonas Malaco and contributors, GPL-3.0)
- Inspired by the philosophy of [suckless](https://suckless.org/) and the simplicity of [fan2go](https://github.com/ibolba17/fan2go)

## License

GPL-3.0
