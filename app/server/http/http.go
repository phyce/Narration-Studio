package http

import (
	"context"
	"fmt"
	customMiddleware "nstudio/app/server/http/middleware"
	configRoute "nstudio/app/server/http/routes/config"
	"nstudio/app/server/http/routes/engines"
	"nstudio/app/server/http/routes/profiles"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type ServerConfig struct {
	Host string
	Port int
}

func StartHTTPServer(config ServerConfig) error {
	echoServer := echo.New()

	echoServer.HideBanner = true
	echoServer.HidePort = false

	echoServer.Use(middleware.Logger())
	echoServer.Use(middleware.Recover())
	echoServer.Use(middleware.CORS())
	echoServer.Use(middleware.RequestID())

	echoServer.Use(middleware.BodyLimit("10M"))

	echoServer.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: 60 * time.Second,
	}))

	setupRoutes(echoServer)

	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	fmt.Printf("HTTP server listening on %s\n", address)

	go func() {
		if err := echoServer.Start(address); err != nil {
			echoServer.Logger.Info("Shutting down the server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := echoServer.Shutdown(ctx); err != nil {
		return err
	}

	fmt.Println("Server stopped")
	return nil
}

func setupRoutes(server *echo.Echo) {
	server.GET("/health", handleHealth)
	server.GET("/info", handleInfo)

	api := server.Group("")
	api.Use(customMiddleware.AuthMiddleware)

	// TTS endpoints
	api.POST("/tts", handleUnifiedTTS)
	api.POST("/tts/:engineId/:modelId/:voiceId", handleSimpleTTS)

	// Engine endpoints
	api.GET("/engines", engines.GetEngines)
	api.GET("/engines/:engineId/models", engines.GetModels)
	api.GET("/engines/:engineId/models/:modelId/voices", engines.GetVoices)

	// Voice tree endpoint
	api.GET("/voices", engines.GetAllVoices)

	// Profile endpoints
	api.GET("/profiles", handleListProfiles)
	api.POST("/profiles", handleCreateProfile)
	api.GET("/profiles/:profileId", handleGetProfile)
	api.DELETE("/profiles/:profileId", handleDeleteProfile)

	api.GET("/profiles/:profileId/voices", profiles.GetVoices)
	api.GET("/profiles/:profileId/voices/:character", profiles.GetCharacterVoice)
	api.POST("/profiles/:profileId/voices/:character", profiles.SetCharacterVoice)
	api.DELETE("/profiles/:profileId/voices/:character", profiles.DeleteCharacterVoice)

	// Admin-only endpoints
	admin := server.Group("")
	admin.Use(customMiddleware.AdminAuthMiddleware)

	// Config endpoints
	admin.GET("/config", configRoute.Get)
	admin.POST("/config", configRoute.Update)
	admin.PATCH("/config", configRoute.Patch)
	admin.GET("/config/value", configRoute.GetValue)
	admin.GET("/config/schema", configRoute.GetSchema)
}
