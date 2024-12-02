package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/mona-actions/gh-migrate-variables/internal/api"
	"github.com/pterm/pterm"
	"github.com/spf13/viper"
)

func ExportVariables() error {
	start := time.Now()
	spinner, _ := pterm.DefaultSpinner.Start("Exporting variables...")
	// Validate environment variables
	organization := viper.GetString("source-organization")
	token := viper.GetString("source-token")
	hostname := viper.GetString("source-hostname")

	if organization == "" || token == "" {
		return fmt.Errorf("missing required environment variables: GHMV_SOURCE_ORGANIZATION, GHMV_SOURCE_TOKEN, or VARIABLES_CSV_FILE")
	}

	var allVariables []map[string]string

	// Fetch organization variables
	pterm.Info.Printf("Fetching organization variables for %s...", organization)
	orgVariables, err := api.FetchOrgVariables(organization, token, hostname)
	if err != nil {
		pterm.Error.Printf("Warning: Failed to fetch organization variables: %v\n", err)
	} else {
		pterm.Success.Printf("Found %d organization variables\n", len(orgVariables))
		allVariables = append(allVariables, orgVariables...)
	}

	// Fetch repositories
	pterm.Info.Printf("Fetching repository list for %s...\n", organization)
	repos, err := api.FetchAllRepositories(organization, token, hostname)
	if err != nil {
		return fmt.Errorf("failed to fetch repositories: %w", err)
	}
	pterm.Info.Printf("Found %d repositories\n", len(repos))

	// Process each repository
	var successful, failed int
	for _, repo := range repos {
		pterm.Info.Printf("Querying Actions API for variables in %s...\n", repo)
		repoVariables, err := api.FetchRepoVariables(organization, repo, token, hostname)
		if err != nil {
			pterm.Error.Printf("Warning: Failed to fetch variables for repo %s: %v\n", repo, err)
			failed++
			continue
		}

		if len(repoVariables) > 0 {
			allVariables = append(allVariables, repoVariables...)
			pterm.Success.Printf("Found %d variables in repository %s\n", len(repoVariables), repo)
			successful++
		} else {
			successful++
		}
	}

	// Exit if no variables found
	if len(allVariables) == 0 {
		pterm.Info.Println("No variables found to export.")
		return nil
	}

	// Create and write to CSV file
	outputFile := organization + "_variables.csv"
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("cannot create file %s: %w", outputFile, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Name", "Value", "Scope", "Visibility"}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write variables
	variablesWritten := 0
	for _, variable := range allVariables {
		if name, ok := variable["Name"]; ok && name != "" {
			value := variable["Value"]
			scope := variable["Scope"]
			visibility := variable["Visibility"]
			if err := writer.Write([]string{name, value, scope, visibility}); err != nil {
				return fmt.Errorf("failed to write variable to CSV: %w", err)
			}
			variablesWritten++
		}
	}
	spinner.Success()
	// Print summary
	fmt.Printf("\n📊 Export Summary:\n")
	fmt.Printf("Total repositories found: %d\n", len(repos))
	fmt.Printf("✅ Successfully processed: %d repositories\n", successful)
	fmt.Printf("❌ Failed to process: %d repositories\n", failed)
	fmt.Printf("📝 Total variables exported: %d\n", variablesWritten)
	fmt.Printf("📁 Output file: %s\n", outputFile)
	fmt.Printf("🕐 Total time: %v\n", time.Since(start).Round(time.Second))

	if failed > 0 {
		fmt.Printf("\n🛑 Export completed with some failures. Some variables may not have been exported.\n")
		fmt.Printf("export completed with %d failed repositories", failed)
		os.Exit(1)
	}

	fmt.Println("\n✅ Export completed successfully!")
	return nil
}
