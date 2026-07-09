// Package internal: KryOs core logic.
package internal

import "embed"

// SystemdFS exposes the embedded systemd unit files (kryos.service, kryos.timer)
// so the --install handler can extract them to /etc/systemd/system/.
//
//go:embed systemd/*
var SystemdFS embed.FS
