package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/xrpscan/heimdall-ingestor/internal/config"
	"github.com/xrpscan/heimdall-ingestor/internal/logger"
	"github.com/xrpscan/heimdall-ingestor/internal/rest"
	"github.com/xrpscan/heimdall-ingestor/pkg/registry"
)

func main() {
	// This is the root context of the app.
	// It should be passed to all services of the app (example: http server, database client).
	// It is canceled automatically if an interruption is detected.
	// It should be canceled *manually* by the programmer in case any fatal error occurs.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Allow the user to specify the config path.
	// This makes switching between test and live configs convenient.
	configPath := flag.String("config", "config/config.json", "config file path")
	flag.Parse()

	// Very first dependency of the app.
	conf, err := config.Load(*configPath)
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Setup logger.
	closer := logger.Init(conf.Logger.FilePath, conf.Logger.Level, conf.Logger.Pretty)

	// Registry to ensure graceful shutdown.
	reg := registry.New(slog.Default())
	// Close all registered services before application exit.
	defer reg.MustCloseAll()

	// Register the log file for closing.
	reg.RegisterWithFunc("log-file", func(context.Context) error { return closer() })

	// Log config file path along with the working directory to avoid confusions.
	wd, _ := os.Getwd()
	slog.InfoContext(ctx, "config file path", "path", *configPath, "wd", wd)

	// Create http server and start listening.
	setupHttpServer(ctx, cancel, conf, reg)

	// Block until the app is interrupted or a process calls the CancelFunc.
	<-ctx.Done()
}

// setupHttpServer creates a new http server and asynchronously starts listening.
//
// It registers the http server with the registry and also calls cancel if the server errors.
func setupHttpServer(
	ctx context.Context, cancel context.CancelFunc, conf config.Config, reg *registry.Registry,
) {
	// Create and register the REST API server of the app.
	server := rest.NewServer(ctx, conf.HttpServer.Addr, rest.NewHandler(conf))
	reg.Register("http-server", server)

	go func() {
		// Signal the registry for closure.
		defer cancel()
		slog.InfoContext(ctx, "starting the http-server", "addr", conf.HttpServer.Addr)

		// Start listening.
		if err := server.ListenAndServe(); err != nil {
			slog.ErrorContext(ctx, "error in ListenAndServe call", "error", err)
		}
	}()
}
