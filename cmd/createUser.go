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

var createUserCmd = &cobra.Command{
	Use:   "create-user",
	Short: "Provisions a single new user from a file.",
	Long: `Reads a JSON file containing the new user's attributes, validates that the
user does not already exist in SmartSuite, then creates the user and updates the local store.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		fromFile, _ := cmd.Flags().GetString("from-file")
		slog.Info("Starting create-user process", "from_file", fromFile)

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

		var newUser models.SCIMUser
		if err := json.Unmarshal(inputData, &newUser); err != nil {
			slog.Error("Failed to unmarshal user data from file", "error", err)
			os.Exit(1)
		}

		if newUser.UserName == "" {
			slog.Error("Input user data must contain a 'userName' (ePPN).")
			os.Exit(1)
		}

		targetEPPN := newUser.UserName

		// --- Validation ---
		slog.Info("Validating user existence before creation...", "eppn", targetEPPN)

		// 1. Check the API first for the most up-to-date information.
		existingUser, err := client.GetUserByUsername(ctx, targetEPPN)
		if err != nil {
			slog.Error("Failed to search for user via API", "eppn", targetEPPN, "error", err)
			os.Exit(1)
		}
		if existingUser != nil {
			slog.Error("User already exists in SmartSuite. Cannot create a duplicate.", "eppn", targetEPPN, "scim_id", existingUser.ID)
			os.Exit(1)
		}

		// 2. As a secondary check, ensure they aren't in our local store either.
		userStore, err := s.LoadUsers()
		if err != nil {
			slog.Error("Failed to load local user store", "error", err)
			os.Exit(1)
		}

		if _, exists := userStore[targetEPPN]; exists {
			slog.Error("User already exists in the local store. Run 'refresh' to sync state.", "eppn", targetEPPN)
			os.Exit(1)
		}

		// --- Execution ---
		logAndAudit(s, "CreateUser", targetEPPN, "info", "Attempting to create user...")

		createdUser, err := client.CreateUser(ctx, newUser)
		if err != nil {
			logAndAudit(s, "CreateUser", targetEPPN, "fatal", "Failed to create user via API", "error", err)
		}

		// --- Success Path ---
		userStore[createdUser.UserName] = models.UserRecord{
			SCIMID:       createdUser.ID,
			Email:        createdUser.Emails[0].Value,
			Status:       "active",
			Name:         createdUser.Name,
			Title:        createdUser.Title,
			Organization: createdUser.EnterpriseData.Organization,
		}

		if err := s.SaveUsers(userStore); err != nil {
			logAndAudit(s, "CreateUser", targetEPPN, "fatal", "API user creation succeeded, but failed to save to local store. MANUAL INTERVENTION REQUIRED.", "error", err)
		}

		logAndAudit(s, "CreateUser", targetEPPN, "info", "Successfully created user.", "scim_id", createdUser.ID)
		slog.Info("Create user process completed successfully.")
	},
}
