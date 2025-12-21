//go:build windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// SetupSignalHandler sets up signal handling for graceful shutdown
func SetupSignalHandler(exitFunc func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		if exitFunc != nil {
			exitFunc()
		}
		os.Exit(0)
	}()
}
