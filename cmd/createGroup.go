package cmd

import (
	"encoding/json"
	"log/slog"
	"os"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/smartsuite"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createGroupCmd = &cobra.Command{
	Use:   "create-group",
	Short: "Provisions a new group (team) from a file.",
	Long:  `Reads a JSON file containing the new group's name, then creates the group in SmartSuite.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		fromFile, _ := cmd.Flags().GetString("from-file")
		slog.Info("Starting create-group process", "from_file", fromFile)

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

		inputData, err := os.ReadFile(fromFile)
		if err != nil {
			slog.Error("Failed to read input file", "file", fromFile, "error", err)
			os.Exit(1)
		}

		var newGroup models.SCIMGroup
		if err := json.Unmarshal(inputData, &newGroup); err != nil {
			slog.Error("Failed to unmarshal group data from file", "error", err)
			os.Exit(1)
		}

		if newGroup.DisplayName == "" {
			slog.Error("Input group data must contain a 'displayName'.")
			os.Exit(1)
		}

		targetGroupName := newGroup.DisplayName

		groupStore, err := s.LoadGroups()
		if err != nil {
			slog.Error("Failed to load local group store", "error", err)
			os.Exit(1)
		}

		if _, exists := groupStore[targetGroupName]; exists {
			slog.Error("Group with this name already exists in the local store.", "group_name", targetGroupName)
			os.Exit(1)
		}

		logAndAudit(s, "CreateGroup", targetGroupName, "info", "Attempting to create group...")

		createdGroup, err := client.CreateGroup(ctx, newGroup)
		if err != nil {
			logAndAudit(s, "CreateGroup", targetGroupName, "fatal", "Failed to create group via API", "error", err)
		}

		groupStore[createdGroup.DisplayName] = models.GroupRecord{
			SCIMID: createdGroup.ID,
		}

		if err := s.SaveGroups(groupStore); err != nil {
			logAndAudit(s, "CreateGroup", targetGroupName, "fatal", "API group creation succeeded, but failed to save to local store. MANUAL INTERVENTION REQUIRED.", "error", err)
		}

		logAndAudit(s, "CreateGroup", targetGroupName, "info", "Successfully created group.", "scim_id", createdGroup.ID)
		slog.Info("Create group process completed successfully.")
	},
}
