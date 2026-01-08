//go:build cli

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"nstudio/app/common/daemon"
	"nstudio/app/common/util"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

type commandLineArguments struct {
	Mode       string
	Port       int
	Host       string
	ConfigFile string
	Status     bool
	Stop       bool
	Logs       bool
	Help       bool
}

func processCommandLine() commandLineArguments {
	mode := flag.String("mode", "help", "Server mode: gui, http, websocket, grpc, tcp, namedpipe, filesystem")
	port := flag.Int("port", 8989, "Server port (for applicable modes)")
	host := flag.String("host", "localhost", "Server host (for applicable modes)")
	configFile := flag.String("config", "", "Path to custom config JSON file")
	status := flag.Bool("status", false, "Check server status")
	stop := flag.Bool("stop", false, "Stop running server")
	logs := flag.Bool("logs", false, "Show server log file location")
	help := flag.Bool("help", false, "Show help")

	flag.Parse()

	arguments := commandLineArguments{
		Mode:       *mode,
		Port:       *port,
		Host:       *host,
		ConfigFile: *configFile,
		Status:     *status,
		Stop:       *stop,
		Logs:       *logs,
		Help:       *help,
	}

	if arguments.Status {
		handleStatus()
		os.Exit(0)
	}

	if arguments.Stop {
		handleStop()
		os.Exit(0)
	}

	if arguments.Logs {
		handleLogs()
		os.Exit(0)
	}

	if arguments.Help {
		log.Info(helpText)
		os.Exit(0)
	}

	return arguments
}

func handleStatus() {
	running, pid, err := daemon.GetDaemonStatus()
	if err != nil {
		fmt.Printf("Error checking server status: %v\n", err)
		os.Exit(1)
	}

	if !running {
		fmt.Println("🔴 Server is not running")
		os.Exit(1)
	}

	statusInfo, err := daemon.ReadDaemonStatus()
	if err != nil {
		fmt.Printf("🟡 Server is running (PID: %d) but status info unavailable: %v\n", pid, err)
		fmt.Printf("Status file path: %s\n", daemon.GetStatusFilePath())
		return
	}

	uptime := time.Since(statusInfo.StartTime)

	fmt.Printf("🟢 Server is running (PID: %d)\n", statusInfo.PID)
	fmt.Printf("Version:            %s\n", statusInfo.Version)
	fmt.Printf("Uptime:             %s\n", util.FormatDuration(uptime))
	fmt.Printf("Processed Messages: %d\n", statusInfo.ProcessedMessages)
}

func handleStop() {
	running, pid, err := daemon.GetDaemonStatus()
	if err != nil {
		fmt.Printf("Error checking server status: %v\n", err)
		os.Exit(1)
	}

	if !running {
		fmt.Println("Server is not running")
		os.Exit(1)
	}

	fmt.Printf("Stopping server (PID: %d)...\n", pid)
	err = daemon.StopDaemon()
	if err != nil {
		fmt.Printf("Error stopping server: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Server stopped successfully")
}

func handleLogs() {
	logFilePath := daemon.GetLogFilePath()

	// Check if log file exists
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		fmt.Printf("Log file does not exist: %s\n", logFilePath)
		fmt.Println("Server may not be running or hasn't created logs yet.")
		return
	}

	fmt.Printf("Attaching to server log: %s\n", logFilePath)
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println("----------------------------------------")

	file, err := os.Open(logFilePath)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Seek to end - 2048 bytes (approx) to show some context if file is large
	stat, _ := file.Stat()
	if stat.Size() > 2048 {
		file.Seek(-2048, io.SeekEnd)
	}

	reader := bufio.NewReader(file)
	if stat.Size() > 2048 {
		// Read until newline to avoid partial line
		reader.ReadBytes('\n')
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			fmt.Printf("Error reading log: %v\n", err)
			break
		}
		fmt.Print(line)
		// Small sleep to yield CPU
		time.Sleep(10 * time.Millisecond)
	}
}
