//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler configures OS-specific signal handling.
// On Unix systems (Linux, macOS, BSD), supports SIGHUP for hot-reload.
func setupSignalHandler() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return sigChan
}

// isReloadSignal checks if the signal is a configuration reload signal.
// On Unix systems, this is SIGHUP.
func isReloadSignal(sig os.Signal) bool {
	return sig == syscall.SIGHUP
}

// isShutdownSignal checks if the signal is a shutdown signal.
// On Unix systems, this includes SIGINT (Ctrl+C) and SIGTERM.
func isShutdownSignal(sig os.Signal) bool {
	return sig == os.Interrupt || sig == syscall.SIGTERM
}
