package sync

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/mona-actions/gh-migrate-variables/internal/api"
	"github.com/pterm/pterm"
	"github.com/spf13/viper"
)

// SyncVariables handles the syncing of variables from a CSV file to a target organization
func SyncVariables() error {
	start := time.Now()
	spinner, _ := pterm.DefaultSpinner.Start("Sync finished...")

	inputFile := viper.GetString("file")
	hostname := viper.GetString("target-hostname")
	targetOrg := viper.GetString("target-organization")
	targetToken := viper.GetString("target-token")

	if inputFile == "" || targetOrg == "" || targetToken == "" {
		return fmt.Errorf("missing required parameters: mapping file, target organization, or target token")
	}

	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("cannot open file %s: %v", inputFile, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("cannot read file %s: %v", inputFile, err)
	}

	var stats struct {
		total     int
		succeeded int
		failed    int
		skipped   int
	}

	// Skip header row and process variables
	for _, record := range records[1:] {
		stats.total++

		if len(record) < 4 {
			pterm.Warning.Printf("Warning: record %v does not have enough columns. Skipping...\n", record)
			stats.skipped++
			continue
		}

		variableName := record[0]
		variableValue := record[1]
		scope := record[2]
		visibility := record[3]

		pterm.Info.Printf("Syncing variable - Name: %s, Value: %s, Scope: %s, Visibility: %s\n",
			variableName, variableValue, scope, visibility)

		if scope == "organization" {
			err := api.AddOrgVariable(targetOrg, variableName, variableValue, visibility, targetToken, hostname)
			if err != nil {
				pterm.Error.Printf("Error adding organization variable %s: %v\n", variableName, err)
				stats.failed++
			} else {
				pterm.Success.Printf("Added organization variable: %s\n", variableName)
				stats.succeeded++
			}
		} else {
			err := api.AddRepoVariable(targetOrg, scope, variableName, variableValue, visibility, targetToken, hostname)
			if err != nil {
				// Check if the error is due to missing repository
				if err.Error() == fmt.Sprintf("repository %s does not exist in organization %s", scope, targetOrg) {
					pterm.Warning.Printf("Skipping variable %s: %v\n", variableName, err)
					stats.skipped++
				} else {
					pterm.Error.Printf("Error adding repository variable %s: %v\n", variableName, err)
					stats.failed++
				}
			} else {
				pterm.Success.Printf("Added repository variable: %s in %s\n", variableName, scope)
				stats.succeeded++
			}
		}
	}
	if stats.failed > 0 {
		spinner.Warning("Some variables failed to sync")
	} else {
		spinner.Success()
	}

	fmt.Printf("\n📊 Sync Summary:\n")
	fmt.Printf("Total variables processed: %d\n", stats.total)
	fmt.Printf("✅ Successfully created: %d\n", stats.succeeded)
	fmt.Printf("❌ Failed: %d\n", stats.failed)
	fmt.Printf("🚧 Skipped: %d\n", stats.skipped)
	fmt.Printf("🕐 Total time: %v\n", time.Since(start).Round(time.Second))

	if stats.failed > 0 {
		fmt.Printf("\n🛑 sync completed with %d failed variables\n", stats.failed)
		os.Exit(1)
	}

	fmt.Println("\n✅ Sync completed successfully!")
	return nil
}
