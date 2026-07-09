// Package internal: KryOs core logic.
package internal

import "embed"

// ScriptsFS exposes embedded utility scripts so --install can extract them.
//
//go:embed scripts/*
var ScriptsFS embed.FS
