package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
	"github.com/pterm/pterm"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

type ProxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
}

type GitHubClientConfig struct {
	Token    string
	Hostname string
}

const (
	defaultVariableVisibility = "private"
	EntityTypeOrg             = "organization"
	EntityTypeRepository      = "repository"
)

// Helper function to create a consistent API context with a timeout
func createAPITimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// Helper function to create a longer-lived context for retry operations
func createLongLivedContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

// Helper function to handle optional hostname parameter
func extractHostname(hostname ...string) string {
	if len(hostname) > 0 {
		return hostname[0]
	}
	return ""
}

// Creates a proxy function based on the provided ProxyConfig
func buildProxyFunction(proxyConfig *ProxyConfig) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		// Check if the request host should bypass the proxy based on NoProxy settings
		if proxyConfig != nil && proxyConfig.NoProxy != "" {
			noProxyURLs := strings.Split(proxyConfig.NoProxy, ",")
			reqHost := req.URL.Host
			for _, noProxy := range noProxyURLs {
				if strings.TrimSpace(noProxy) == reqHost {
					return nil, nil
				}
			}
		}
		// Determine the appropriate proxy URL based on the scheme (http or https)
		if proxyConfig != nil {
			if req.URL.Scheme == "https" && proxyConfig.HTTPSProxy != "" {
				return url.Parse(proxyConfig.HTTPSProxy)
			}
			if req.URL.Scheme == "http" && proxyConfig.HTTPProxy != "" {
				return url.Parse(proxyConfig.HTTPProxy)
			}
		}
		return nil, nil
	}
}

// Retrieves proxy configuration from environment variables
func loadProxyConfigFromEnv() *ProxyConfig {
	return &ProxyConfig{
		HTTPProxy:  viper.GetString("HTTP_PROXY"),
		HTTPSProxy: viper.GetString("HTTPS_PROXY"),
		NoProxy:    viper.GetString("NO_PROXY"),
	}
}

// Creates a new GitHub client with optional proxy and enterprise hostname support
func initializeGitHubClient(config GitHubClientConfig) (*github.Client, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	// Create an OAuth2 HTTP client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Token})

	// Set up proxy configuration if available
	proxyConfig := loadProxyConfigFromEnv()
	transport := &http.Transport{
		Proxy:                 buildProxyFunction(proxyConfig),
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       10 * time.Second,
	}

	// Create an HTTP client with the configured transport
	tc := oauth2.NewClient(ctx, ts)
	tc.Transport = &oauth2.Transport{
		Base:   transport,
		Source: ts,
	}

	// Create the GitHub client using the HTTP client
	client := github.NewClient(tc)

	// If a hostname is provided, configure the client for GitHub Enterprise
	if config.Hostname != "" {
		baseURL, err := url.Parse(config.Hostname)
		if err != nil {
			return nil, fmt.Errorf("invalid hostname URL provided (%s): %w", baseURL, err)
		}
		client, err = client.WithEnterpriseURLs(config.Hostname, config.Hostname)
		if err != nil {
			return nil, fmt.Errorf("failed to configure enterprise URLs for %s: %w", config.Hostname, err)
		}
	}

	return client, nil
}

// Retries the given operation with a context, using an exponential backoff strategy
func retryWithExponentialBackoff(ctx context.Context, operation func() error) error {
	// Retrieve the maximum number of retries from configuration, defaulting to 3 if not set
	maxRetries := viper.GetInt("RETRY_MAX")
	if maxRetries <= 0 {
		maxRetries = 3
	}

	// Retrieve the retry delay from configuration, defaulting to 1 second if not set
	retryDelay, err := time.ParseDuration(viper.GetString("RETRY_DELAY"))
	if err != nil {
		retryDelay = time.Second
	}

	var lastErr error
	// Attempt the operation, retrying with exponential backoff if it fails
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := operation(); err == nil {
			// If the operation succeeds, return nil
			return nil
		} else {
			lastErr = err
			// If the operation fails and more retries are allowed, wait before retrying
			if attempt < maxRetries {
				waitTime := retryDelay * time.Duration(1<<uint(attempt-1))
				pterm.Warning.Printf("Attempt %d failed, retrying in %v: %v\n", attempt, waitTime, lastErr)

				// select waits for either context cancellation or the backoff timer to expire
				select {
				// Handles context cancellation (timeout, deadline, or explicit cancel)
				case <-ctx.Done():
					return fmt.Errorf("operation cancelled: %w", ctx.Err())

				// Waits for backoff duration before retrying the operation
				case <-time.After(waitTime):
					continue
				}
			}
		}
	}
	// If all attempts fail, return the last encountered error
	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}

// Wrapper function to retry an operation with a default context
func retryWithDefaultContext(operation func() error) error {
	// Create a longer-lived context for retries
	ctx, cancel := createLongLivedContext()
	// Retry the operation using the created context
	err := retryWithExponentialBackoff(ctx, operation)
	cancel()
	return err
}

// Parses a GitHub Actions variable into a map representation
func parseGitHubVariable(variable *github.ActionsVariable, scope string) map[string]string {
	// Return nil if the variable is nil or has no name
	if variable == nil || variable.Name == "" {
		return nil
	}

	// Create a map with variable details, including scope and visibility
	parsedVar := map[string]string{
		"Name":  variable.Name,
		"Value": variable.Value,
		"Scope": scope,
	}
	// Set the visibility to the provided value or use the default visibility if not set
	if variable.Visibility != nil {
		parsedVar["Visibility"] = *variable.Visibility
	} else {
		parsedVar["Visibility"] = defaultVariableVisibility
	}

	return parsedVar
}

// Retrieves variables from a GitHub organization or repository
func fetchGitHubVariables(entityType, org, repo, token string, hostname ...string) ([]map[string]string, error) {
	// Validate that the organization name is provided
	if org == "" {
		return nil, fmt.Errorf("organization name is required")
	}
	// Validate that the repository name is provided for repository-level variables
	if entityType == EntityTypeRepository && repo == "" {
		return nil, fmt.Errorf("repository name is required")
	}

	// Initialize a new GitHub client
	client, err := initializeGitHubClient(GitHubClientConfig{Token: token, Hostname: extractHostname(hostname...)})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub client: %w", err)
	}

	var variables *github.ActionsVariables
	// Retry the variable retrieval operation
	err = retryWithDefaultContext(func() error {
		ctx, cancel := createAPITimeoutContext()
		defer cancel()
		var apiErr error

		// Retrieve variables based on entity type (organization or repository)
		if entityType == EntityTypeOrg {
			variables, _, apiErr = client.Actions.ListOrgVariables(ctx, org, nil)
		} else {
			variables, _, apiErr = client.Actions.ListRepoVariables(ctx, org, repo, nil)
		}
		return apiErr
	})

	// Handle any errors from the variable retrieval process
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s variables: %w", entityType, err)
	}

	if variables == nil {
		return nil, fmt.Errorf("no variables data returned for %s %s", entityType, org)
	}

	// Parse and collect the variables into a slice of maps
	var parsedVariables []map[string]string
	scope := entityType
	if entityType == EntityTypeRepository {
		scope = repo
	}

	for _, variable := range variables.Variables {
		parsedVar := parseGitHubVariable(variable, scope)
		if parsedVar != nil {
			parsedVariables = append(parsedVariables, parsedVar)
		}
	}

	return parsedVariables, nil
}

// Retrieves organization-level variables from GitHub
func FetchOrgVariables(org, token string, hostname ...string) ([]map[string]string, error) {
	// Calls fetchGitHubVariables for organization-level variables
	return fetchGitHubVariables(EntityTypeOrg, org, "", token, hostname...)
}

// Retrieves repository-level variables from GitHub
func FetchRepoVariables(org, repo, token string, hostname ...string) ([]map[string]string, error) {
	// Calls fetchGitHubVariables for repository-level variables
	return fetchGitHubVariables(EntityTypeRepository, org, repo, token, hostname...)
}

// Creates a variable in a GitHub organization or repository
func addGitHubVariable(entityType, org, repo, name, value, visibility, token string, hostname ...string) error {
	// Validate that the organization name and variable name are provided
	if org == "" || name == "" {
		return fmt.Errorf("organization name and variable name are required")
	}
	// Validate that the repository name is provided for repository-level variables
	if entityType == EntityTypeRepository && repo == "" {
		return fmt.Errorf("repository name is required")
	}

	// Check if the repository exists if creating a repo variable
	if entityType == EntityTypeRepository {
		exists, err := doesRepositoryExist(org, repo, token, hostname...)
		if err != nil {
			return fmt.Errorf("failed to check repository existence: %w", err)
		}
		if !exists {
			return fmt.Errorf("repository %s does not exist in organization %s", repo, org)
		}
	}

	// Initialize a new GitHub client
	client, err := initializeGitHubClient(GitHubClientConfig{Token: token, Hostname: extractHostname(hostname...)})
	if err != nil {
		return fmt.Errorf("failed to initialize GitHub client: %w", err)
	}

	// Set default visibility if not provided
	if visibility == "" {
		visibility = defaultVariableVisibility
	}

	// Create the GitHub Actions variable
	variable := &github.ActionsVariable{
		Name:       name,
		Value:      value,
		Visibility: github.String(visibility),
	}

	// Retry the variable creation operation
	err = retryWithDefaultContext(func() error {
		ctx, cancel := createAPITimeoutContext()
		defer cancel()

		// Create the variable based on the entity type (organization or repository)
		if entityType == EntityTypeOrg {
			_, err := client.Actions.CreateOrgVariable(ctx, org, variable)
			return err
		}
		_, err = client.Actions.CreateRepoVariable(ctx, org, repo, variable)
		return err
	})

	// Handle any errors from the variable creation process
	if err != nil {
		return fmt.Errorf("failed to create %s variable %s: %w", entityType, name, err)
	}

	return nil
}

// Creates an organization-level variable in GitHub
func AddOrgVariable(org, name, value, visibility, token string, hostname ...string) error {
	// Calls addGitHubVariable for an organization-level variable
	return addGitHubVariable(EntityTypeOrg, org, "", name, value, visibility, token, hostname...)
}

// Creates a repository-level variable in GitHub
func AddRepoVariable(org, repo, name, value, visibility, token string, hostname ...string) error {
	// Calls addGitHubVariable for a repository-level variable
	return addGitHubVariable(EntityTypeRepository, org, repo, name, value, visibility, token, hostname...)
}

// Checks if a repository exists in a given organization
func doesRepositoryExist(org, repo, token string, hostname ...string) (bool, error) {
	// Initialize a new GitHub client
	client, err := initializeGitHubClient(GitHubClientConfig{Token: token, Hostname: extractHostname(hostname...)})
	if err != nil {
		return false, fmt.Errorf("failed to initialize GitHub client: %w", err)
	}

	// Create a context with a timeout
	ctx, cancel := createAPITimeoutContext()
	defer cancel()

	// Attempt to retrieve the repository
	_, resp, err := client.Repositories.Get(ctx, org, repo)
	if err != nil {
		return false, nil
	}
	// Return true if the repository is found (status code 200)
	return resp.StatusCode == 200, nil
}

// Lists paginated GitHub resources, such as repositories
func listPaginatedRepositories(fetch func(opts *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error)) ([]string, error) {
	// Set up pagination options, requesting 100 items per page
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allResources []string

	// Iterate through pages of results
	for {
		repos, resp, err := fetch(opts)
		if err != nil {
			return nil, err
		}
		if repos == nil {
			return nil, fmt.Errorf("no data returned")
		}

		// Collect repository names from the current page
		for _, repo := range repos {
			if repo != nil && repo.Name != nil {
				allResources = append(allResources, *repo.Name)
			}
		}

		// If there are no more pages, break the loop
		if resp == nil || resp.NextPage == 0 {
			break
		}
		// Move to the next page
		opts.Page = resp.NextPage
	}

	return allResources, nil
}

// Retrieves a list of repositories for a given organization
func FetchAllRepositories(org, token string, hostname ...string) ([]string, error) {
	// Initialize a new GitHub client
	client, err := initializeGitHubClient(GitHubClientConfig{Token: token, Hostname: extractHostname(hostname...)})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub client: %w", err)
	}

	// Use listPaginatedRepositories to fetch all repositories in the organization
	return listPaginatedRepositories(func(opts *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error) {
		ctx, cancel := createAPITimeoutContext()
		defer cancel()
		return client.Repositories.ListByOrg(ctx, org, opts)
	})
}
