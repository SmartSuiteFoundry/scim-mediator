package cmd

import (
	"log/slog"
	"os"
	"time"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/smartsuite"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cleanupUsersCmd = &cobra.Command{
	Use:   "cleanup-users",
	Short: "Deletes users who are past their deactivation grace period.",
	Long: `Scans the local user store for any user who was deactivated more than 7 days ago.
For each user found, it issues a permanent DELETE request to the SmartSuite API
and removes them from the local store. This is intended to be run as a nightly scheduled task.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		slog.Info("Starting cleanup process for deactivated users")

		apiURL := viper.GetString("api_url")
		apiKey := viper.GetString("api_key")
		dataDir := viper.GetString("data_dir")
		if dataDir == "" {
			dataDir = "./data"
		}

		client, err := smartsuite.NewClient(apiURL, apiKey)
		if err != nil {
			slog.Error("Failed to create API client", "error", err)
			os.Exit(1)
		}

		s, err := store.NewStore(dataDir)
		if err != nil {
			slog.Error("Failed to create store", "error", err)
			os.Exit(1)
		}

		userStore, err := s.LoadUsers()
		if err != nil {
			slog.Error("Failed to load local user store", "error", err)
			os.Exit(1)
		}

		gracePeriod := -7 * 24 * time.Hour
		cutoffTime := time.Now().Add(gracePeriod)
		usersToDelete := make(map[string]string)

		for eppn, record := range userStore {
			if record.DeactivationTimestamp != nil && record.DeactivationTimestamp.Before(cutoffTime) {
				usersToDelete[eppn] = record.SCIMID
			}
		}

		if len(usersToDelete) == 0 {
			slog.Info("No users found past their deactivation grace period. Cleanup complete.")
			return
		}

		slog.Info("Found users to be permanently deleted.", "count", len(usersToDelete))

		var failedDeletions []string
		for eppn, scimID := range usersToDelete {
			if ctx.Err() != nil {
				slog.Warn("Shutdown signal received during cleanup. Halting.", "reason", ctx.Err())
				break
			}
			logAndAudit(s, "CleanupUser", eppn, "info", "Attempting to delete user.", "scim_id", scimID)

			err := client.DeleteUser(ctx, scimID)
			if err != nil {
				logAndAudit(s, "CleanupUser", eppn, "error", "Failed to delete user via API", "error", err)
				failedDeletions = append(failedDeletions, eppn)
				continue
			}

			delete(userStore, eppn)
			logAndAudit(s, "CleanupUser", eppn, "info", "Successfully deleted user.")
		}

		if err := s.SaveUsers(userStore); err != nil {
			slog.Error("CRITICAL: Finished API deletions but failed to save updated user store. The store is now out of sync.", "error", err)
			os.Exit(1)
		}

		slog.Info("Cleanup process finished.")
		if len(failedDeletions) > 0 {
			slog.Warn("Some users failed to be deleted and will be retried on the next run.", "count", len(failedDeletions), "failed_eppns", failedDeletions)
		}
	},
}
