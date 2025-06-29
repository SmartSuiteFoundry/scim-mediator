package cmd

import (
	"context"
	"log/slog"
	"os"
	"reflect"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/smartsuite"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refreshes the local store by comparing with live data from SmartSuite.",
	Long:  `Fetches all users and groups from the SmartSuite API, compares them to the local store, logs any deltas found, and updates the local store.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		slog.Info("Starting refresh & reconcile process")

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

		// --- Reconcile Users ---
		slog.Info("--- Reconciling Users ---")
		if err := reconcileUsers(ctx, s, client); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				slog.Warn("Refresh process halted by shutdown signal.", "reason", err)
				return
			}
			slog.Error("Failed to reconcile users", "error", err)
			os.Exit(1)
		}

		// --- Reconcile Groups ---
		slog.Info("--- Reconciling Groups ---")
		if err := reconcileGroups(ctx, s, client); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				slog.Warn("Refresh process halted by shutdown signal.", "reason", err)
				return
			}
			slog.Error("Failed to reconcile groups", "error", err)
			os.Exit(1)
		}

		slog.Info("Refresh process completed successfully.")
	},
}

func reconcileUsers(ctx context.Context, s *store.Store, client *smartsuite.Client) error {
	oldState, err := s.LoadUsers()
	if err != nil {
		return err
	}

	scimUsers, err := client.GetUsers(ctx)
	if err != nil {
		return err
	}
	newState := make(map[string]models.UserRecord)
	for _, u := range scimUsers {
		if u.UserName == "" {
			continue
		}
		status := "inactive"
		if u.Active {
			status = "active"
		}
		newState[u.UserName] = models.UserRecord{
			SCIMID:       u.ID,
			Email:        u.Emails[0].Value,
			Status:       status,
			Name:         u.Name,
			Title:        u.Title,
			Organization: u.EnterpriseData.Organization,
		}
	}

	for eppn, newUser := range newState {
		if oldUser, ok := oldState[eppn]; !ok {
			logAndAudit(s, "Refresh: Delta Found", eppn, "info", "User created in SmartSuite directly.", "scim_id", newUser.SCIMID)
		} else {
			// Check for changes in key fields. Using reflect.DeepEqual for structs like Name.
			if oldUser.Status != newUser.Status {
				logAndAudit(s, "Refresh: Delta Found", eppn, "info", "User status changed outside of mediator.", "from_status", oldUser.Status, "to_status", newUser.Status)
			}
			if oldUser.Title != newUser.Title {
				logAndAudit(s, "Refresh: Delta Found", eppn, "info", "User title changed outside of mediator.", "from_title", oldUser.Title, "to_title", newUser.Title)
			}
			if !reflect.DeepEqual(oldUser.Name, newUser.Name) {
				logAndAudit(s, "Refresh: Delta Found", eppn, "info", "User name changed outside of mediator.")
			}
		}
	}

	for eppn, oldUser := range oldState {
		if _, ok := newState[eppn]; !ok {
			logAndAudit(s, "Refresh: Delta Found", eppn, "info", "User deleted in SmartSuite directly.", "scim_id", oldUser.SCIMID)
		}
	}

	if err := s.SaveUsers(newState); err != nil {
		return err
	}
	slog.Info("User reconciliation complete.", "total_users", len(newState))
	return nil
}

func reconcileGroups(ctx context.Context, s *store.Store, client *smartsuite.Client) error {
	oldState, err := s.LoadGroups()
	if err != nil {
		return err
	}

	scimGroups, err := client.GetGroups(ctx)
	if err != nil {
		return err
	}
	newState := make(map[string]models.GroupRecord)
	for _, g := range scimGroups {
		if g.DisplayName == "" {
			continue
		}
		newState[g.DisplayName] = models.GroupRecord{SCIMID: g.ID}
	}

	for name, newGroup := range newState {
		if _, ok := oldState[name]; !ok {
			logAndAudit(s, "Refresh: Delta Found", name, "info", "Group created in SmartSuite directly.", "scim_id", newGroup.SCIMID)
		}
	}

	for name, oldGroup := range oldState {
		if _, ok := newState[name]; !ok {
			logAndAudit(s, "Refresh: Delta Found", name, "info", "Group deleted in SmartSuite directly.", "scim_id", oldGroup.SCIMID)
		}
	}

	if err := s.SaveGroups(newState); err != nil {
		return err
	}
	slog.Info("Group reconciliation complete.", "total_groups", len(newState))
	return nil
}
