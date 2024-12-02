package cmd

import (
	"fmt"

	"github.com/mona-actions/gh-migrate-variables/pkg/export"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// exportCmd represents the export command
var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Exports organization and repository variables to CSV",
	Long:  "Exports organization and repository variables to CSV",
	Run: func(cmd *cobra.Command, args []string) {
		GetFlagOrViperValue(cmd, map[string]bool{
			"source-hostname":     false,
			"source-organization": true,
			"source-token":        true,
			"search-depth":        false,
		})
		ShowConnectionStatus("export")
		if err := export.ExportVariables(); err != nil {
			fmt.Printf("failed to export variables: %v\n", err)
		}
		return
	},
}

func init() {
	// Add flags to the ExportCmd
	ExportCmd.Flags().StringP("source-hostname", "n", "", "GitHub Enterprise Server hostname (optional) Ex. github.example.com")
	ExportCmd.Flags().StringP("source-organization", "o", "", "Organization to export (required)")
	ExportCmd.Flags().StringP("source-token", "t", "", "GitHub token (required)")

	// Bind flags to viper
	viper.BindPFlag("GHMV_SOURCE_HOSTNAME", ExportCmd.Flags().Lookup("source-hostname"))
	viper.BindPFlag("GHMV_SOURCE_ORGANIZATION", ExportCmd.Flags().Lookup("source-organization"))
	viper.BindPFlag("GHMV_SOURCE_TOKEN", ExportCmd.Flags().Lookup("source-token"))
}
