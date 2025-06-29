package logger

import (
	"log/slog"
	"os"
)

// Init sets up a global structured JSON logger for the application.
// It sets the log level based on the debug flag.
func Init(debug bool) {
	var logLevel slog.Level
	if debug {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}
