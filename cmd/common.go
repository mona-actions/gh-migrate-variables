package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetFlagOrViperValue(cmd *cobra.Command, flags map[string]bool) map[string]string {
	values := make(map[string]string)
	var missing []string

	for name, required := range flags {
		envName := "GHMV_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))

		flagVal, _ := cmd.Flags().GetString(name)
		kebabVal := viper.GetString(name)
		prefixedVal := viper.GetString(envName)

		value := ""
		if flagVal != "" {
			value = flagVal
		} else if kebabVal != "" {
			value = kebabVal
		} else if prefixedVal != "" {
			value = prefixedVal
		}

		if value != "" {
			viper.Set(name, value)
			viper.Set(envName, value)
			values[name] = value
		} else if required {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "Error: missing required values: %s\n", strings.Join(missing, ", "))
		os.Exit(1)
	}

	return values
}

func ShowConnectionStatus(actionType string) {
	var endpoint string // Declare endpoint once

	// Determine the endpoint based on action type
	switch actionType {
	case "export":
		endpoint = "source-hostname"
	case "sync":
		endpoint = "target-hostname"
	}

	hostname := getNormalizedEndpoint(endpoint)
	httpProxy := viper.GetString("HTTP_PROXY")
	httpsProxy := viper.GetString("HTTPS_PROXY")

	fmt.Println(getHostnameMessage(hostname))
	fmt.Println(getProxyStatus(httpProxy, httpsProxy))
}

func getNormalizedEndpoint(key string) string {
	hostname := viper.GetString(key)
	if hostname != "" {
		hostname = strings.TrimPrefix(hostname, "http://")
		hostname = strings.TrimPrefix(hostname, "https://")
		hostname = strings.TrimSuffix(hostname, "/api/v3")
		hostname = strings.TrimSuffix(hostname, "/")
		hostname = fmt.Sprintf("https://%s/api/v3", hostname)
		viper.Set(key, hostname)
	}
	return hostname
}

func getHostnameMessage(hostname string) string {
	if hostname != "" {
		return fmt.Sprintf("\n🔗 Using: GitHub Enterprise Server: %s", hostname)
	}
	return "\n📡 Using: GitHub.com"
}

func getProxyStatus(httpProxy, httpsProxy string) string {
	if httpProxy != "" || httpsProxy != "" {
		return "🔄 Proxy: ✅ Configured\n"
	}
	return "🔄 Proxy: ❌ Not configured\n"
}
