//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

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

func HideConsole() {
}
