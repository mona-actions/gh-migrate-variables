package cmd

import (
	"fmt"

	"github.com/mona-actions/gh-migrate-variables/pkg/sync"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var SyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync organization and repository variables from CSV",
	Long:  "Sync organization and repository variables from CSV",
	Run: func(cmd *cobra.Command, args []string) {
		GetFlagOrViperValue(cmd, map[string]bool{
			"file":                true,
			"target-hostname":     false,
			"target-organization": true,
			"target-token":        true,
		})

		ShowConnectionStatus("sync")
		
		if err := sync.SyncVariables(); err != nil {
			fmt.Printf("failed to export variables: %v\n", err)
		}
		return
	},
}

func init() {
	// Add flags to the SyncCmd
	SyncCmd.Flags().StringP("file", "f", "", "CSV file containing variables to synchronize")
	SyncCmd.Flags().StringP("target-hostname", "n", "", "GitHub Enterprise Server hostname URL (optional) Ex. https://github.example.com")
	SyncCmd.Flags().StringP("target-organization", "o", "", "Organization to sync (required)")
	SyncCmd.Flags().StringP("target-token", "t", "", "GitHub token (required)")

	// Bind flags to viper
	viper.BindPFlag("GHMV_FILE", SyncCmd.Flags().Lookup("file"))
	viper.BindPFlag("GHMV_TARGET_HOSTNAME", SyncCmd.Flags().Lookup("target-hostname"))
	viper.BindPFlag("GHMV_TARGET_ORGANIZATION", SyncCmd.Flags().Lookup("target-organization"))
	viper.BindPFlag("GHMV_TARGET_TOKEN", SyncCmd.Flags().Lookup("target-token"))
}
