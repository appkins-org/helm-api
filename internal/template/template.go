package template

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

// TemplateRequest represents the JSON payload for the template endpoint.
type TemplateRequest struct {
	// Chart specifies the chart to template (required)
	Chart string `json:"chart" schema:"chart,required"`

	// ChartVersion specifies the version of the chart (optional)
	ChartVersion string `json:"chart_version,omitempty" schema:"chart_version"`

	// Repository specifies the chart repository URL (optional)
	Repository string `json:"repository,omitempty" schema:"repository"`

	// ReleaseName is the name of the release (optional, defaults to "release-name")
	ReleaseName string `json:"release_name,omitempty" schema:"release_name"`

	// Namespace is the target namespace (optional, defaults to "default")
	Namespace string `json:"namespace,omitempty" schema:"namespace"`

	// Values contains the values to use for templating
	Values []string `json:"values,omitempty" schema:"set"`

	// ValueFiles contains paths to values files
	ValueFiles []string `json:"value_files,omitempty" schema:"values"`

	// StringValues contains values as strings (--set equivalent)
	StringValues []string `json:"string_values,omitempty" schema:"set_string"`

	// FileValues contains values from files (--set-file equivalent)
	FileValues []string `json:"file_values,omitempty" schema:"set_file"`

	// JSONValues contains values from JSON strings (--set-json equivalent)
	JSONValues []string `json:"json_values,omitempty" schema:"set_json"`

	// IncludeCRDs indicates whether to include CRDs in output
	IncludeCRDs bool `json:"include_crds,omitempty" schema:"include_crds"`

	// SkipTests indicates whether to skip test manifests
	SkipTests bool `json:"skip_tests,omitempty" schema:"skip_tests"`

	// ShowOnly limits output to specific templates
	ShowOnly []string `json:"show_only,omitempty" schema:"show_only"`

	// KubeVersion specifies the Kubernetes version for capabilities
	KubeVersion string `json:"kube_version,omitempty" schema:"kube_version"`

	// APIVersions specifies additional API versions for capabilities
	APIVersions []string `json:"api_versions,omitempty" schema:"api_versions"`

	// IsUpgrade sets .Release.IsUpgrade instead of .Release.IsInstall
	IsUpgrade bool `json:"is_upgrade,omitempty" schema:"is_upgrade"`

	// Validate enables manifest validation
	Validate bool `json:"validate,omitempty" schema:"validate"`

	// CreateNamespace indicates whether to create the namespace if it doesn't exist
	CreateNamespace bool `json:"create_namespace,omitempty" schema:"create_namespace"`
}

// ProcessTemplate processes a Helm template request and returns the rendered YAML.
func ProcessTemplate(req TemplateRequest) (string, error) {
	if req.Chart == "" {
		return "", fmt.Errorf("chart is required")
	}

	// Set defaults
	if req.ReleaseName == "" {
		req.ReleaseName = "release-name"
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	// Create Helm configuration
	settings := cli.New()
	actionConfig := new(action.Configuration)

	settings.PluginsDirectory = "/tmp/.plugins" // Disable plugins directory for client-only mode
	settings.RepositoryCache = "/tmp/.cache"    // Disable repository cache for client-only mode

	if err := os.MkdirAll(settings.PluginsDirectory, 0o775); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}

	if err := os.MkdirAll(settings.RepositoryCache, 0o775); err != nil {
		return "", fmt.Errorf("failed to create repository cache directory: %w", err)
	}

	// Initialize action configuration for client-only mode (no cluster connection)
	if err := actionConfig.Init(nil, req.Namespace, "memory", debug); err != nil {
		return "", fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Setup registry client for OCI support
	registryClient, err := registry.NewClient()
	if err != nil {
		return "", fmt.Errorf("failed to create registry client: %w", err)
	}
	actionConfig.RegistryClient = registryClient

	// Handle repository setup if needed
	if req.Repository != "" {
		if registry.IsOCI(req.Repository) {
			// Handle OCI repository login/caching if needed
			if err := handleOCIRepository(registryClient, req.Repository, req.Chart); err != nil {
				return "", fmt.Errorf("failed to handle OCI repository: %w", err)
			}
		} else {
			// Handle HTTP/HTTPS repository setup
			if err := handleHTTPRepository(settings, req.Repository, req.Chart); err != nil {
				return "", fmt.Errorf("failed to handle HTTP repository: %w", err)
			}
		}
	}

	// Create install action (for templating)
	client := action.NewInstall(actionConfig)
	client.DryRun = true
	client.ClientOnly = true // Always use client-only mode
	client.ReleaseName = req.ReleaseName
	client.Namespace = req.Namespace
	client.Replace = true
	client.IncludeCRDs = req.IncludeCRDs
	client.IsUpgrade = req.IsUpgrade
	client.CreateNamespace = req.CreateNamespace

	// Set Kubernetes version if provided, otherwise use latest stable
	if req.KubeVersion != "" {
		kubeVersion, err := parseKubeVersion(req.KubeVersion)
		if err != nil {
			return "", fmt.Errorf("invalid kube version '%s': %w", req.KubeVersion, err)
		}
		client.KubeVersion = kubeVersion
	} else {
		// Default to latest stable Kubernetes version
		client.KubeVersion = &chartutil.KubeVersion{
			Version: "v1.32.0",
			Major:   "1",
			Minor:   "32",
		}
	}

	// Set API versions if provided
	if len(req.APIVersions) > 0 {
		client.APIVersions = req.APIVersions
	}

	// Prepare values
	valueOpts := &values.Options{
		ValueFiles:   req.ValueFiles,
		Values:       req.Values,
		StringValues: req.StringValues,
		FileValues:   req.FileValues,
		JSONValues:   req.JSONValues,
	}

	// Merge values
	vals, err := valueOpts.MergeValues(getter.All(settings))
	if err != nil {
		return "", fmt.Errorf("failed to merge values: %w", err)
	}

	// Configure chart path options for repository support
	chartName := req.Chart
	if req.Repository != "" {
		if registry.IsOCI(req.Repository) {
			// For OCI repositories, construct the full OCI path
			if !strings.Contains(chartName, req.Repository) {
				// If chart doesn't already contain the full OCI path, construct it
				chartName = strings.TrimSuffix(req.Repository, "/") + "/" + strings.TrimPrefix(chartName, "/")
			}
		} else {
			// For HTTP/HTTPS repositories, use the repository name format that Helm expects
			repoName := generateRepoName(req.Repository)
			chartName = repoName + "/" + req.Chart
		}
	}

	if req.ChartVersion != "" {
		client.Version = req.ChartVersion
		client.ChartPathOptions.Version = req.ChartVersion
	}

	// Load chart
	chartPath, err := client.LocateChart(chartName, settings)
	if err != nil {
		return "", fmt.Errorf("failed to locate chart: %w", err)
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return "", fmt.Errorf("failed to load chart: %w", err)
	}

	// Validate chart
	if err := chartRequested.Validate(); err != nil {
		return "", fmt.Errorf("chart validation failed: %w", err)
	}

	// Run template
	rel, err := client.Run(chartRequested, vals)
	if err != nil {
		return "", fmt.Errorf("failed to run template: %w", err)
	}

	return renderManifests(rel, req.ShowOnly, req.SkipTests)
}

// renderManifests renders the release manifests to YAML.
func renderManifests(rel *release.Release, showOnly []string, skipTests bool) (string, error) {
	var manifests bytes.Buffer

	// Write main manifest
	if rel.Manifest != "" {
		manifests.WriteString(strings.TrimSpace(rel.Manifest))
		manifests.WriteString("\n")
	}

	// Write hooks if not disabled
	for _, hook := range rel.Hooks {
		if skipTests && isTestHook(hook) {
			continue
		}
		manifests.WriteString("---\n")
		manifests.WriteString(fmt.Sprintf("# Source: %s\n", hook.Path))
		manifests.WriteString(hook.Manifest)
		manifests.WriteString("\n")
	}

	result := manifests.String()

	// Filter by showOnly if specified
	if len(showOnly) > 0 {
		result = filterManifests(result, showOnly)
	}

	return result, nil
}

// isTestHook checks if a hook is a test hook.
func isTestHook(hook *release.Hook) bool {
	return slices.Contains(hook.Events, release.HookTest)
}

// filterManifests filters manifests based on showOnly patterns.
func filterManifests(manifests string, showOnly []string) string {
	// This is a simplified implementation
	// In a full implementation, you would parse manifests and match against file patterns
	return manifests
}

// parseKubeVersion parses a Kubernetes version string.
func parseKubeVersion(kubeVersion string) (*chartutil.KubeVersion, error) {
	// Simple version parsing - in production you'd want more robust parsing
	parts := strings.Split(strings.TrimPrefix(kubeVersion, "v"), ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid version format")
	}

	return &chartutil.KubeVersion{
		Version: kubeVersion,
		Major:   parts[0],
		Minor:   parts[1],
	}, nil
}

// mergeMaps merges two maps, with the second map taking precedence.
func mergeMaps(a, b map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy from a
	for k, v := range a {
		result[k] = v
	}

	// Override/add from b
	for k, v := range b {
		result[k] = v
	}

	return result
}

// debug is a debug function for Helm (simplified for client-only mode).
func debug(format string, v ...any) {
	// Debug logging is disabled in client-only mode
	// In production, you might want to use a proper logger
}

// handleOCIRepository handles OCI repository operations for caching and downloading
func handleOCIRepository(registryClient *registry.Client, repository, chart string) error {
	if !registry.IsOCI(repository) {
		return nil // Not an OCI repository, nothing to do
	}

	// For OCI repositories, we don't need explicit login for public repositories
	// The registry client will handle caching automatically when the chart is pulled
	// This is a simplified implementation without credential handling

	// Log that we're handling an OCI repository
	debug("Handling OCI repository: %s for chart: %s", repository, chart)

	return nil
}

// handleHTTPRepository handles HTTP/HTTPS repository setup for caching and chart downloading
func handleHTTPRepository(settings *cli.EnvSettings, repository, chart string) error {
	if repository == "" || registry.IsOCI(repository) {
		return nil // Not an HTTP repository or empty, nothing to do
	}

	debug("Handling HTTP repository: %s for chart: %s", repository, chart)

	// Ensure the repository cache directory exists
	if err := os.MkdirAll(settings.RepositoryCache, 0755); err != nil {
		return fmt.Errorf("failed to create repository cache directory: %w", err)
	}

	// Generate a repository name based on the URL for caching
	repoName := generateRepoName(repository)

	// Create repository entry
	repoEntry := &repo.Entry{
		Name: repoName,
		URL:  repository,
	}

	// Load or create repositories file
	repoFile := repo.NewFile()
	if settings.RepositoryConfig != "" {
		if _, err := os.Stat(settings.RepositoryConfig); err == nil {
			repoFile, err = repo.LoadFile(settings.RepositoryConfig)
			if err != nil {
				debug("Warning: failed to load existing repository file, creating new one: %v", err)
				repoFile = repo.NewFile()
			}
		}
	}

	// Check if repository already exists
	if !repoFile.Has(repoName) {
		// Add the repository to the file
		repoFile.Add(repoEntry)

		// Write the repositories file
		if err := repoFile.WriteFile(settings.RepositoryConfig, 0644); err != nil {
			return fmt.Errorf("failed to write repository config: %w", err)
		}
	}

	// Create a ChartRepository for downloading the index
	chartRepo, err := repo.NewChartRepository(repoEntry, getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	// Set the cache file path
	chartRepo.CachePath = settings.RepositoryCache

	// Download and cache the repository index
	indexFile, err := chartRepo.DownloadIndexFile()
	if err != nil {
		return fmt.Errorf("failed to download repository index from %s: %w", repository, err)
	}

	debug("Repository index downloaded successfully: %s", indexFile)
	debug("Repository setup completed for: %s", repository)
	return nil
}

// generateRepoName generates a repository name from a URL
func generateRepoName(repoURL string) string {
	// Simple implementation: use a hash or simplified name
	// Remove protocol and special characters
	name := strings.ReplaceAll(repoURL, "https://", "")
	name = strings.ReplaceAll(name, "http://", "")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return name
}
