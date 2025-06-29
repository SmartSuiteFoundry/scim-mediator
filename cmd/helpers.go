package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"
)

// logAndAudit provides a consistent way to log structured messages to the console
// and also append a human-readable event to the audit.log file.
func logAndAudit(s *store.Store, useCase, target, level, details string, args ...interface{}) {
	// Structured logging for console/log collection
	logArgs := append([]interface{}{"use_case", useCase, "target", target}, args...)

	switch level {
	//case "info":
	//	slog.Info(details, logArgs...)
	case "warn":
		slog.Warn(details, logArgs...)
	case "error":
		slog.Error(details, logArgs...)
	case "fatal":
		// Log as error and then exit.
		slog.Error(details, logArgs...)
		os.Exit(1)
	default:
		//	slog.Info(details, logArgs...)
	}

	// Plain text audit log for human-readable history
	event := models.AuditEvent{
		Timestamp: time.Now(),
		UseCase:   useCase,
		Target:    target,
		Status:    level, // The status in the audit log reflects the log level
		Details:   fmt.Sprintf("%s (%v)", details, args),
	}
	if err := s.AppendToAuditLog(event); err != nil {
		slog.Warn("Failed to write to audit log", "error", err)
	}
}
