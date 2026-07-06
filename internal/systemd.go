// Package internal: lógica pura de KryOs.
package internal

import "embed"

// SystemdFS expone las unidades systemd embebidas (kryos.service, kryos.timer)
// para que el handler --install pueda extraerlas a /etc/systemd/system/.
//
//go:embed systemd/*
var SystemdFS embed.FS
