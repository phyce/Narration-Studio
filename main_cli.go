//go:build cli

package main

import (
	"fmt"
	"nstudio/app/common/daemon"
	"nstudio/app/common/issue"
	"nstudio/app/config"
	"nstudio/app/server"
	"nstudio/app/tts/modelManager"
	"os"
	"time"
)

func main() {
	SetupSignalHandler(func() {
		fmt.Println("\nShutting down...")
	})

	arguments := processCommandLine()

	if err := initializeApp(arguments.ConfigFile); err != nil {
		issue.Panic("Failed to initialize app", err)
	}

	modelManager.Initialize(false)
	registerEngines()

	app := NewApp()

	if arguments.Mode == "gui" {
		fmt.Println("Error: GUI mode not supported in CLI build.")
		os.Exit(1)
	}

	if arguments.Mode == "help" {
		fmt.Println(helpText)
		return
	}
	startCmdServer(app, arguments)
}

func startCmdServer(app *App, options commandLineArguments) {
	if !daemon.IsDaemonChild() {
		pid, err := daemon.Daemonize()
		if err != nil {
			fmt.Printf("Error starting server: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Server started with PID: %d\n", pid)
		fmt.Printf("Use --stop to stop the server\n")
		fmt.Printf("Use --status to check server status\n")
		return
	}

	serverConfig := server.ServerConfig{
		Mode:       server.ServerMode(options.Mode),
		Port:       options.Port,
		Host:       options.Host,
		ConfigFile: options.ConfigFile,
	}

	pidFile := daemon.GetPidFilePath()
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		fmt.Printf("Warning: Failed to write PID file: %v\n", err)
	}

	status := daemon.DaemonStatusInfo{
		PID:               os.Getpid(),
		Version:           config.GetInfo().Version,
		StartTime:         time.Now(),
		ProcessedMessages: 0,
	}

	if err := daemon.WriteDaemonStatus(status); err != nil {
		fmt.Printf("Warning: Failed to write status file: %v\n", err)
	}

	defer func() {
		os.Remove(pidFile)
		os.Remove(daemon.GetStatusFilePath())
	}()

	fmt.Printf("Starting %s server on %s:%d (PID: %d)\n", serverConfig.Mode, options.Host, options.Port, os.Getpid())

	err := server.StartServer(serverConfig)
	if err != nil {
		issue.Panic("Failed to start server", err)
	}
}
