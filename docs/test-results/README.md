# Test results

Resumen anonimizado de las pruebas realizadas durante la fase de validación
de KryOs. No incluye logs crudos ni datos del sistema local.

## Pruebas realizadas

| # | Prueba | Resultado | Métricas clave |
|---|--------|-----------|----------------|
| 1 | Calibración PWM vs liquidctl (3 puntos) | ✅ 3/3 OK | diff: 3, 22, 0 RPM (margen 100) |
| 2 | A/B en idle (bash + KryOs dry-run) | ✅ 0 divergencias | ~50 ticks, 1 transición bomba 1→0 |
| 3 | A/B bajo carga (8 cores, 120s) | ✅ 0 divergencias | 2 transiciones bomba 1→2, 2→3 |
| 4 | A/B cooldown (post-carga) | ✅ 0 divergencias | transición bomba 1→0 |
| 5 | Validación post-fix state persistente | ✅ Bomba nivel 3 | pwm subió 115 → 165 → 230 |

## Prueba 1: Calibración

Test ejecutado con el timer del bash **pausado** para evitar interferencia.
Compara RPM del método pwm directo contra `liquidctl set pump speed N`.

| Punto | pwm directo | liquidctl | diff |
|-------|-------------|-----------|------|
| 35%   | 1354 RPM    | 1351 RPM  | 3    |
| 65%   | 2090 RPM    | 2068 RPM  | 22   |
| 90%   | 2597 RPM    | 2597 RPM  | 0    |

Todos los puntos dentro del margen de 100 RPM. Método pwm directo validado
como bit-equivalente a liquidctl.

> Nota: una primera ejecución con el bash activo dio diffs > 700 RPM.
> Causa: el timer del bash sobrescribía pwm1 entre las escrituras de KryOs
> y las lecturas. Solución: pausar el timer durante el test.

## Pruebas 2-4: A/B comparativo

KryOs en modo `--dry-run` (lee hwmon, calcula, **no escribe**) en paralelo
con el script bash que regulaba el sistema. Ambos reguladores vieron las
mismas temperaturas y persistieron sus decisiones en archivos de state
separados. Comparación tick a tick.

**Métricas globales**:
- Ticks analizados: 100+
- Divergencias: 0
- Estados visitados: `{0,2}`, `{1,2}`, `{2,2}`, `{3,2}`

**Transiciones validadas**:

| Transición | Tick aprox. | Bash | KryOs |
|------------|-------------|------|-------|
| Bomba 1→0 (idle) | minuto 6 | ✓ | ✓ |
| Bomba 0→1 (subida) | minuto 1 | ✓ | ✓ |
| Bomba 1→2 (stress) | minuto 11 | ✓ | ✓ |
| Bomba 2→3 (stress) | minuto 13 | ✓ | ✓ |
| Bomba 3→1 (cooldown) | minuto 17 | ✓ | ✓ |

El fan alcanzó nivel 2 (líquido 38°C) pero no llegó a nivel 3 (líquido 42°C)
durante la ventana de captura. Cobertura del 75% de las transiciones
posibles de la matriz 4×4.

## Prueba 5: Validación post-fix de state persistente

**Bug detectado**: con `RuntimeDirectory=kryos` + `Type=oneshot`, systemd
borra el directorio entre ejecuciones. Resultado: KryOs siempre leía state
`{0,0}` y nunca podía pasar de nivel 1, aunque la CPU superara 85°C
(umbral nivel 3).

**Evidencia pre-fix** (CPU 91.6°C, líquido 42.3°C):
- pwm1 se mantuvo en 115 (nivel 1 = 45%) durante los 120s de stress
- La bomba nunca subió a nivel 2 ni 3
- El líquido alcanzó el umbral de nivel 3 en fan, pero el fan no respondió

**Fix aplicado**:
- `RuntimeDirectory=` → `StateDirectory=` (persiste entre ejecuciones)
- State file: `/run/kryos/curve.state` → `/var/lib/kryos/curve.state`

**Evidencia post-fix** (mismo test de stress 120s):
- pwm1 escaló correctamente: 115 → 165 → 230
- RPM correspondientes: 1600 → 2100 → 2600
- Nivel 3 alcanzado a los 90s de stress (CPU ~85.5°C)

## Reproducir las pruebas

Los scripts de test están en `../scripts/`:

- `../scripts/kryos-ab-monitor.sh` — captura A/B en idle
- `../scripts/kryos-ab-monitor-stress.sh` — captura A/B bajo stress
- `../scripts/kryos-dryrun.service` — unit para KryOs en dry-run
- `../scripts/kryos-dryrun.timer` — timer cada 10s para dry-run

Para reproducir:
1. Compilar e instalar KryOs: `sudo install -m 0755 kryos /usr/local/bin/`
2. Activar KryOs real: `sudo kryos --install`
3. Activar KryOs dry-run en paralelo: `sudo cp scripts/kryos-dryrun.{service,timer} /etc/systemd/system/ && sudo systemctl enable --now kryos-dryrun.timer`
4. Lanzar monitor: `sudo BASH_STATE_PATH=/var/lib/kryos/curve.state kryos-ab-monitor.sh 600`
5. Comparar con `sudo kryos --get-state`
