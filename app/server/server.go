package server

import (
	"nstudio/app/common/response"
	serverHTTP "nstudio/app/server/http"
	"nstudio/app/server/stats"
)

type ServerMode string

const (
	ModeGUI        ServerMode = "gui"
	ModeHTTP       ServerMode = "http"
	ModeWebSocket  ServerMode = "websocket"
	ModeGRPC       ServerMode = "grpc"
	ModeTCP        ServerMode = "tcp"
	ModeNamedPipe  ServerMode = "namedpipe"
	ModeFileSystem ServerMode = "filesystem"
	ModeLibrary    ServerMode = "library"
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
	stats.Initialize(string(serverConfig.Mode), serverConfig.Host, serverConfig.Port)

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
		return response.Err("Unsupported server mode: %s", serverConfig.Mode)
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
	return response.Err("WebSocket server not yet implemented")
}

func startGRPCServer(config ServerConfig) error {
	return response.Err("gRPC server not yet implemented")
}

func startTCPServer(config ServerConfig) error {
	return response.Err("TCP server not yet implemented")
}

func startNamedPipeServer(config ServerConfig) error {
	return response.Err("Named pipe server not yet implemented")
}

func startFileSystemServer(config ServerConfig) error {
	return response.Err("File system server not yet implemented")
}
