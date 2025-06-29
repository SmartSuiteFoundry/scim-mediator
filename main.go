package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SmartSuiteFoundry/scim-mediator/cmd"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/logger"
)

func main() {
	// We need to parse flags before initializing the logger to check for --debug.
	// We do a pre-parse here. Cobra will parse them again, which is fine.
	var debug bool
	for _, arg := range os.Args[1:] {
		if arg == "--debug" {
			debug = true
			break
		}
	}

	// Initialize the structured logger for the entire application.
	logger.Init(debug)
	slog.Info("Application starting", "debug_mode", debug)

	// Set up a context that is cancelled on an interrupt signal (Ctrl+C).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Execute the root command with the cancellable context.
	cmd.ExecuteContext(ctx)

	slog.Info("Application shutting down")
}
