# gh-migrate-variables

![build](https://github.com/mona-actions/gh-migrate-variables/actions/workflows/build.yml/badge.svg)
![GitHub Release](https://img.shields.io/github/v/release/mona-actions/gh-migrate-variables)

`gh-migrate-variables` is a [GitHub CLI](https://cli.github.com) extension to assist in the migration of variables between GitHub organizations. While [GitHub Enterprise Importer](https://github.com/github/gh-gei) provides excellent features for organization migration, there are gaps when it comes to migrating GitHub Actions variables. This extension aims to fill those gaps. Whether you're consolidating organizations, setting up new environments, or need to replicate variables across organizations, this extension can help.

## Install

```bash
gh extension install mona-actions/gh-migrate-variables
```

## Usage: Export

Export organization-level and repository-level variables to a CSV file.

```bash
Usage:
  migrate-variables export [flags]

Flags:
  -h, --help                         help for export
  -n, --source-hostname string       GitHub Enterprise Server hostname (optional) Ex. github.example.com
  -o, --source-organization string   Organization to export (required)
  -t, --source-token string          GitHub token (required)
```

### Example Export Command

```bash
gh migrate-variables export \
    -o mona-actions \
    -t ghp_xxxxxxxxxxxx
```

This will create a file named `mona-actions_variables.csv` containing all organization and repository variables. The export process provides a summary:

```
📊 Export Summary:
Total repositories found: 155
✅ Successfully processed: 155 repositories
❌ Failed to process: 0 repositories
📝 Total variables exported: 3
📁 Output file: mona-actions_variables.csv
🕐 Total time: 45s

✅ Export completed successfully!
```

## Usage: Sync

Recreates variables from a CSV file to a target organization, maintaining visibility settings and scopes.

```bash
Usage:
  migrate-variables sync [flags]

Flags:
  -f, --file string                  CSV mapping file path to use for syncing variables (required)
  -h, --help                         help for sync
  -n, --target-hostname string       GitHub Enterprise Server hostname (optional) Ex. github.example.com
  -o, --target-organization string   Target Organization to sync variables to (required)
  -t, --target-token string          Target Organization GitHub token. Scopes: admin:org (required)
```

### Example Sync Command

```bash
gh migrate-variables sync \
    --file mona-actions_variables.csv \
    --target-organization mona-emu \
    --target-token ghp_xxxxxxxxxxxx
```

The sync process also provides a summary of the sync/migration process:

```
📊 Sync Summary:
Total variables processed: 3
✅ Successfully created: 3
❌ Failed: 0
🚧 Skipped: 0 
🕐 Total time: 7s

✅ Sync completed successfully!
```

### Variables CSV Format

The tool exports and imports variables using the following CSV format:

```csv
Name,Value,Scope,Visibility
ORG_VAR,org-value,organization,all
REPO_VAR,repo-value,repository-name,private
```

- `Scope`: Use "organization" for org-level variables, or the repository name for repo-level variables
- `Visibility`: One of "all", "private", or "selected" for org variables; always "private" for repo variables

## Required Permissions

### For Export
- Organization variables: `read:org`
- Repository variables: `repo`

### For Sync
- `admin:org` scope is required for creating organization variables
- `repo` scope is required for creating repository variables

## Proxy Support

The tool supports proxy configuration through both command-line flags and environment variables:

### Command-line flags:
```bash
Global Flags:
    --http-proxy string    HTTP proxy (can also use HTTP_PROXY env var)
    --https-proxy string   HTTPS proxy (can also use HTTPS_PROXY env var)
    --no-proxy string      No proxy list (can also use NO_PROXY env var) 
```
```bash
# Example usage with proxy:
gh migrate-variables sync \
    --target-organization mona-actions \
    --target-token ghp_xxxxxxxxxxxx \
    --https-proxy https://proxy.example.com:8080
```

```bash
# Example with environment variables:
export HTTPS_PROXY=https://proxy.example.com:8080
export NO_PROXY=github.internal.com
export GHMV_TARGET_TOKEN=ghp_...
```
```bash
gh migrate-variables export \
    --source-organization mona-actions
```

## Environment Variables

The tool supports loading configuration from a `.env` file. This provides an alternative to command-line flags and allows you to store your configuration securely.

### Using a .env file

1. Create a `.env` file in your working directory:

```bash
# GitHub Migration Variables (GHMV)
GHMV_SOURCE_ORGANIZATION=mona-actions # Source organization name
GHMV_SOURCE_HOSTNAME=                 # Source hostname
GHMV_SOURCE_TOKEN= ghp_xxx            # Source token
GHMV_TARGET_ORGANIZATION=mone-emu     # Target organization name
GHMV_TARGET_HOSTNAME=                 # Target hostname
GHMV_TARGET_TOKEN= ghp_yyy            # Source token
GHMV_FILE=${GHMV_SOURCE_ORGANIZATION}_variables.csv # Input CSV file name
```

2. Run the commands without flags - the tool will automatically load values from the .env file:

```bash
gh migrate-variables export
```
```bash
gh migrate-variables sync
```

When both environment variables and command-line flags are provided, the command-line flags take precedence. This allows you to override specific values while still using the .env file for most configuration.

### Example with Mixed Usage

```bash
# Load most values from .env but override the target organization
gh migrate-variables sync --target-organization different-org
```

## Retry Configuration

The tool includes configurable retry behavior for API calls:

```bash
Global Flags:
    --retry-delay string   Delay between retries (default "1s")
    --retry-max int        Maximum retry attempts (default 3)
```

Example usage with retry configuration:

```bash
gh migrate-variables export \
    --retry-max 5 \
    --retry-delay 2s
```

This configuration allows you to:
- Adjust the number of retry attempts for failed API calls
- Modify the delay between retry attempts
- Handle temporary API issues or rate limiting more gracefully

## Limitations

- Repository-level variables can only be created if the repository exists in the target organization
- Environment-specific variables should be reviewed before syncing to ensure appropriate values
- Repository visibility settings must be considered when setting organization variable visibility
- The tool will retry failed API calls but may still encounter persistent issues (e.g. network)

## License

- [MIT](./license) (c) [Mona-Actions](https://github.com/mona-actions)
- [Contributing](./contributing.md)
