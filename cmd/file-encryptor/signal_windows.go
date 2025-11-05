//go:build windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler configures OS-specific signal handling.
// On Windows, SIGHUP is not supported. Only Ctrl+C (Interrupt) and SIGTERM work.
// Hot-reload via signals is not available on Windows.
func setupSignalHandler() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	// Note: SIGHUP is not functional on Windows, so we only listen for shutdown signals
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}

// isReloadSignal checks if the signal is a configuration reload signal.
// On Windows, hot-reload via signals is not supported.
func isReloadSignal(sig os.Signal) bool {
	// Windows does not support SIGHUP for hot-reload
	return false
}

// isShutdownSignal checks if the signal is a shutdown signal.
// On Windows, this includes Ctrl+C (Interrupt) and SIGTERM.
func isShutdownSignal(sig os.Signal) bool {
	return sig == os.Interrupt || sig == syscall.SIGTERM
}
