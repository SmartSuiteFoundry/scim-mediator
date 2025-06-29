package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/smartsuite"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var manageGroupMembersCmd = &cobra.Command{
	Use:   "manage-group-members",
	Short: "Adds or removes members from a group.",
	Long:  `Modifies an existing group's membership by adding or removing users based on their ePPN.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		groupName, _ := cmd.Flags().GetString("group")
		addMembers, _ := cmd.Flags().GetStringSlice("add")
		removeMembers, _ := cmd.Flags().GetStringSlice("remove")

		slog.Info("Managing members", "group", groupName, "add_count", len(addMembers), "remove_count", len(removeMembers))

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
			slog.Error("Failed to load user store", "error", err)
			os.Exit(1)
		}
		groupStore, err := s.LoadGroups()
		if err != nil {
			slog.Error("Failed to load group store", "error", err)
			os.Exit(1)
		}

		group, ok := groupStore[groupName]
		if !ok {
			slog.Error("Group not found in local store.", "group_name", groupName)
			os.Exit(1)
		}

		var operations []models.SCIMPatchOp
		for _, eppn := range addMembers {
			user, ok := userStore[eppn]
			if !ok {
				slog.Warn("User not found, cannot add to group. Skipping.", "eppn", eppn)
				continue
			}
			operations = append(operations, models.SCIMPatchOp{
				Op:    "add",
				Path:  "members",
				Value: []map[string]string{{"value": user.SCIMID}},
			})
		}

		for _, eppn := range removeMembers {
			user, ok := userStore[eppn]
			if !ok {
				slog.Warn("User not found, cannot remove from group. Skipping.", "eppn", eppn)
				continue
			}
			operations = append(operations, models.SCIMPatchOp{
				Op:   "remove",
				Path: fmt.Sprintf(`members[value eq "%s"]`, user.SCIMID),
			})
		}

		if len(operations) == 0 {
			slog.Info("No valid members to add or remove. Exiting.")
			return
		}

		logAndAudit(s, "ManageGroupMembers", groupName, "info", "Attempting to modify group...")

		err = client.PatchGroup(ctx, group.SCIMID, operations)
		if err != nil {
			logAndAudit(s, "ManageGroupMembers", groupName, "fatal", "Failed to modify group via API", "error", err)
		}

		logAndAudit(s, "ManageGroupMembers", groupName, "info", "Successfully modified members for group.")
		slog.Info("Group membership management completed successfully.")
	},
}
