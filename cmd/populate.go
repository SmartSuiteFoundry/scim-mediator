package cmd

import (
	"log/slog"
	"os"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/smartsuite"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var populateCmd = &cobra.Command{
	Use:   "populate",
	Short: "Populates the local store by fetching all users and groups from SmartSuite.",
	Long:  `Performs a full read from the SmartSuite SCIM API and overwrites the local users.json and groups.json files. This is intended for initial setup.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		slog.Info("Starting population process")

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

		// Populate Users
		slog.Info("Fetching users from SmartSuite")
		scimUsers, err := client.GetUsers(ctx)
		if err != nil {
			slog.Error("Failed to get users from API", "error", err)
			os.Exit(1)
		}

		userStore := make(map[string]models.UserRecord)
		for _, u := range scimUsers {
			if ctx.Err() != nil {
				slog.Warn("Shutdown signal received during user population. Halting.", "reason", ctx.Err())
				return
			}
			if u.UserName == "" {
				continue
			}
			status := "inactive"
			if u.Active {
				status = "active"
			}
			userStore[u.UserName] = models.UserRecord{
				SCIMID:       u.ID,
				Email:        u.Emails[0].Value,
				Status:       status,
				Name:         u.Name,
				Title:        u.Title,
				Organization: u.EnterpriseData.Organization,
			}
		}

		if err := s.SaveUsers(userStore); err != nil {
			slog.Error("Failed to save users to store", "error", err)
			os.Exit(1)
		}
		slog.Info("Successfully populated users.", "count", len(userStore))

		// Populate Groups
		slog.Info("Fetching groups from SmartSuite")
		scimGroups, err := client.GetGroups(ctx)
		if err != nil {
			slog.Error("Failed to get groups from API", "error", err)
			os.Exit(1)
		}

		groupStore := make(map[string]models.GroupRecord)
		for _, g := range scimGroups {
			if ctx.Err() != nil {
				slog.Warn("Shutdown signal received during group population. Halting.", "reason", ctx.Err())
				return
			}
			if g.DisplayName == "" {
				continue
			}
			groupStore[g.DisplayName] = models.GroupRecord{SCIMID: g.ID}
		}

		if err := s.SaveGroups(groupStore); err != nil {
			slog.Error("Failed to save groups to store", "error", err)
			os.Exit(1)
		}
		slog.Info("Successfully populated groups.", "count", len(groupStore))

		slog.Info("Population process completed successfully.")
	},
}
