# Test results

Anonymized summary of validation tests performed during KryOs development.
Does not include raw logs or local system data.

## Tests performed

| # | Test | Result | Key metrics |
|---|------|--------|-------------|
| 1 | PWM calibration vs liquidctl (3 points) | ✅ 3/3 OK | diff: 3, 22, 0 RPM (margin 100) |
| 2 | A/B at idle (bash + KryOs dry-run) | ✅ 0 divergences | ~50 ticks, 1 pump transition 1→0 |
| 3 | A/B under load (8 cores, 120s) | ✅ 0 divergences | 2 pump transitions 1→2, 2→3 |
| 4 | A/B cooldown (post-load) | ✅ 0 divergences | pump transition 1→0 |
| 5 | Post-fix persistent state validation | ✅ Pump reached level 3 | pwm scaled 115 → 165 → 230 |

## Test 1: Calibration

Test run with the bash timer **paused** to avoid interference.
Compares RPM from direct PWM writes against `liquidctl set pump speed N`.

| Point | Direct PWM | liquidctl | diff |
|-------|------------|-----------|------|
| 35%   | 1354 RPM   | 1351 RPM  | 3    |
| 65%   | 2090 RPM   | 2068 RPM  | 22   |
| 90%   | 2597 RPM   | 2597 RPM  | 0    |

All points within the 100 RPM margin. Direct PWM method validated as
bit-equivalent to liquidctl.

> Note: a first run with the bash timer active gave diffs > 700 RPM.
> Cause: the bash timer overwrote pwm1 between KryOs writes and reads.
> Solution: pause the bash timer during the test.

## Tests 2-4: A/B comparison

KryOs in `--dry-run` mode (reads hwmon, computes, **does not write**)
running in parallel with the bash script that controlled the system.
Both controllers saw the same temperatures and persisted their decisions
in separate state files. Tick-by-tick comparison.

**Global metrics**:
- Ticks analyzed: 100+
- Divergences: 0
- States visited: `{0,2}`, `{1,2}`, `{2,2}`, `{3,2}`

**Validated transitions**:

| Transition | Approx. tick | Bash | KryOs |
|------------|-------------|------|-------|
| Pump 1→0 (idle) | minute 6 | ✓ | ✓ |
| Pump 0→1 (ramp up) | minute 1 | ✓ | ✓ |
| Pump 1→2 (stress) | minute 11 | ✓ | ✓ |
| Pump 2→3 (stress) | minute 13 | ✓ | ✓ |
| Pump 3→1 (cooldown) | minute 17 | ✓ | ✓ |

The fan reached level 2 (liquid 38°C) but did not reach level 3
(liquid 42°C) during the capture window. 75% transition coverage
of the 4×4 matrix.

## Test 5: Post-fix persistent state validation

**Bug found**: with `RuntimeDirectory=kryos` + `Type=oneshot`, systemd
deletes the directory between executions. Result: KryOs always read
state `{0,0}` and could never advance past level 1, even when CPU
exceeded 85°C (level 3 threshold).

**Pre-fix evidence** (CPU 91.6°C, liquid 42.3°C):
- pwm1 stayed at 115 (level 1 = 45%) during the entire 120s stress
- The pump never reached level 2 or 3
- Liquid hit the level 3 fan threshold, but the fan did not respond

**Fix applied**:
- `RuntimeDirectory=` → `StateDirectory=` (persists between runs)
- State file: `/run/kryos/curve.state` → `/var/lib/kryos/curve.state`

**Post-fix evidence** (same 120s stress test):
- pwm1 scaled correctly: 115 → 165 → 230
- Corresponding RPM: 1600 → 2100 → 2600
- Level 3 reached at ~90s of stress (CPU ~85.5°C)

## Reproduce the tests

Test scripts are in `../scripts/`:

- `../scripts/kryos-ab-monitor.sh` — A/B capture at idle
- `../scripts/kryos-ab-monitor-stress.sh` — A/B capture under load
- `../scripts/kryos-dryrun.service` — systemd unit for dry-run KryOs
- `../scripts/kryos-dryrun.timer` — 10s timer for dry-run

To reproduce:
1. Build and install KryOs: `sudo install -m 0755 kryos /usr/local/bin/`
2. Enable KryOs: `sudo kryos --install`
3. Enable dry-run KryOs in parallel: `sudo cp scripts/kryos-dryrun.{service,timer} /etc/systemd/system/ && sudo systemctl enable --now kryos-dryrun.timer`
4. Run monitor: `sudo BASH_STATE_PATH=/var/lib/kryos/curve.state kryos-ab-monitor.sh 600`
5. Compare: `sudo kryos --get-state`
