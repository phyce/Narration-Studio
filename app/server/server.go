package server

import (
	"fmt"
	"nstudio/app/common/response"
	serverHTTP "nstudio/app/server/http"
	"nstudio/app/server/stats"
)

type ServerMode string

const (
	ModeGUI        ServerMode = "gui"        // Default Wails GUI mode
	ModeHTTP       ServerMode = "http"       // HTTP REST API server
	ModeWebSocket  ServerMode = "websocket"  // WebSocket server
	ModeGRPC       ServerMode = "grpc"       // gRPC server
	ModeTCP        ServerMode = "tcp"        // TCP socket server
	ModeNamedPipe  ServerMode = "namedpipe"  // Named pipe server
	ModeFileSystem ServerMode = "filesystem" // File-based communication
	ModeLibrary    ServerMode = "library"    // Shared library mode
)

type ServerConfig struct {
	Mode       ServerMode
	Port       int
	Host       string
	ConfigFile string
}

type ServerAppInterface interface {
	Play(script string, saveNewCharacters bool, overrideVoices string, profileID string)
	GetEngines() string
	GetModelVoices(engine string, model string) string
	//GetCharacterVoices() string
	GetAvailableModels() string
}

func StartServer(serverConfig ServerConfig) error {
	stats.Initialize()

	switch serverConfig.Mode {
	case ModeHTTP:
		return startHTTPServer(serverConfig)
	case ModeWebSocket:
		return startWebSocketServer(serverConfig)
	case ModeGRPC:
		return startGRPCServer(serverConfig)
	case ModeTCP:
		return startTCPServer(serverConfig)
	case ModeNamedPipe:
		return startNamedPipeServer(serverConfig)
	case ModeFileSystem:
		return startFileSystemServer(serverConfig)
	default:
		return response.Err(fmt.Errorf("Unsupported server mode: %s", serverConfig.Mode))
	}
}

func startHTTPServer(config ServerConfig) error {
	httpConfig := serverHTTP.ServerConfig{
		Host: config.Host,
		Port: config.Port,
	}
	return serverHTTP.StartHTTPServer(httpConfig)
}

func startWebSocketServer(config ServerConfig) error {
	return response.Err(fmt.Errorf("WebSocket server not yet implemented"))
}

func startGRPCServer(config ServerConfig) error {
	return response.Err(fmt.Errorf("gRPC server not yet implemented"))
}

func startTCPServer(config ServerConfig) error {
	return response.Err(fmt.Errorf("TCP server not yet implemented"))
}

func startNamedPipeServer(config ServerConfig) error {
	return response.Err(fmt.Errorf("Named pipe server not yet implemented"))
}

func startFileSystemServer(config ServerConfig) error {
	return response.Err(fmt.Errorf("File system server not yet implemented"))
}
