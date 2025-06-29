package cmd

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	// Variable to hold the value of the debug flag
	debug bool
)

var rootCmd = &cobra.Command{
	Use:   "scim-mediator",
	Short: "A trusted mediator for SCIM interactions with SmartSuite.",
	Long: `scim-mediator is a CLI application that provides a reliable and auditable
way to manage the identity lifecycle for a SmartSuite tenant.`,
}

// ExecuteContext executes the root command with a given context.
func ExecuteContext(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	// Define the global --debug flag
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug level logging.")

	// Add sub-commands here
	rootCmd.AddCommand(populateCmd)
	rootCmd.AddCommand(refreshCmd)
	rootCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(createGroupCmd)
	rootCmd.AddCommand(manageGroupMembersCmd)
	rootCmd.AddCommand(processBatchCmd)
	rootCmd.AddCommand(cleanupUsersCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".cobra")
	}

	viper.SetEnvPrefix("SMARTSUITE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		slog.Info("Using config file", "file", viper.ConfigFileUsed())
	}
}
