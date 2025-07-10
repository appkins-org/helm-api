package template

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/appkins-org/helm-api/internal/config"
	"gopkg.in/yaml.v3"
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
	Values []string `json:"set,omitempty" schema:"set"`

	// ValueFiles contains paths to values files
	ValueFiles []string `json:"values,omitempty" schema:"values"`

	// StringValues contains values as strings (--set equivalent)
	StringValues []string `json:"string_values,omitempty" schema:"set_string"`

	// FileValues contains values from files (--set-file equivalent)
	FileValues []string `json:"set_file,omitempty" schema:"set_file"`

	// JSONValues contains values from JSON strings (--set-json equivalent)
	JSONValues []string `json:"set_json,omitempty" schema:"set_json"`

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

	NamespaceLabels []string `json:"namespace_labels,omitempty" schema:"namespace_labels"`
}

// ProcessTemplate processes a Helm template request and returns the rendered YAML.
// Deprecated: Use ProcessTemplateWithConfig instead.
func ProcessTemplate(req TemplateRequest) (string, error) {
	// For backward compatibility, use default configuration
	// This function should eventually be removed in favor of ProcessTemplateWithConfig
	cfg := &config.Config{
		Helm: config.HelmConfig{
			PluginsDirectory: "/tmp/.helm/plugins",
			RepositoryCache:  "/tmp/.helm/repository",
			RepositoryConfig: "/tmp/.helm/repositories.yaml",
			RegistryConfig:   "/tmp/.helm/registry/config.json",
		},
	}
	return ProcessTemplateWithConfig(req, cfg)
}

// renderManifests renders the release manifests to YAML.
func renderManifests(
	rel *release.Release,
	showOnly []string,
	skipTests bool,
	createNamespace bool,
	namespace string,
	namespaceLabels []string,
	req TemplateRequest,
) (string, error) {
	var manifests bytes.Buffer
	manifestMap := make(map[string]string)

	// Add namespace manifest if CreateNamespace is true and namespace is not default
	if createNamespace && namespace != "" && namespace != "default" {
		manifests.WriteString("---\n")
		manifests.WriteString("# Source: namespace.yaml\n")

		namespaceMetadata := map[string]any{
			"name": namespace,
		}
		if len(namespaceLabels) > 0 {
			labels := map[string]string{}
			for _, label := range namespaceLabels {
				if k, v, ok := strings.Cut(label, "="); ok {
					labels[k] = v
				}
			}
			namespaceMetadata["labels"] = labels
		}

		namespaceManifest := map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   namespaceMetadata,
		}

		var nsBuffer bytes.Buffer
		enc := yaml.NewEncoder(&nsBuffer)
		enc.SetIndent(2)
		if err := enc.Encode(&namespaceManifest); err != nil {
			return "", fmt.Errorf("failed to encode namespace manifest: %w", err)
		}

		nsManifestStr := nsBuffer.String()
		manifestMap["namespace.yaml"] = nsManifestStr
		manifests.WriteString(nsManifestStr)
	}

	// Process main manifest
	if rel.Manifest != "" {
		manifestParts := strings.Split(rel.Manifest, "---\n")
		for _, manifestPart := range manifestParts {
			manifestPart = strings.TrimSpace(manifestPart)
			if manifestPart == "" {
				continue
			}

			// Extract source file name and add to map
			lines := strings.SplitN(manifestPart, "\n", 5)
			var sourceFile string
			for _, line := range lines {
				if after, ok := strings.CutPrefix(strings.TrimSpace(line), "# Source: "); ok {
					sourceFile = strings.TrimSpace(after)
					break
				}
			}

			// Add Helm labels and annotations to the manifest
			enrichedManifest, err := addHelmMetadata(manifestPart, req.ReleaseName, req.Namespace)
			if err != nil {
				return "", fmt.Errorf("failed to add Helm metadata: %w", err)
			}

			if sourceFile != "" {
				manifestMap[sourceFile] = enrichedManifest
			}

			if !strings.HasPrefix(enrichedManifest, "---\n") {
				manifests.WriteString("---\n")
			}
			manifests.WriteString(enrichedManifest)
		}
	}

	// Process hooks if not disabled
	for _, hook := range rel.Hooks {
		if skipTests && isTestHook(hook) {
			continue
		}

		// Add Helm labels and annotations to hook manifests
		enrichedHook, err := addHelmMetadata(hook.Manifest, req.ReleaseName, req.Namespace)
		if err != nil {
			return "", fmt.Errorf("failed to add Helm metadata to hook: %w", err)
		}

		manifestMap[hook.Path] = enrichedHook
		manifests.WriteString("---\n")
		manifests.WriteString(fmt.Sprintf("# Source: %s\n", hook.Path))
		manifests.WriteString(enrichedHook)
	}

	result := manifests.String()

	// Filter by showOnly if specified
	if len(showOnly) > 0 {
		result = filterManifests(manifestMap, showOnly)
	}

	return result, nil
}

// isTestHook checks if a hook is a test hook.
func isTestHook(hook *release.Hook) bool {
	return slices.Contains(hook.Events, release.HookTest)
}

/*
filterManifests Implementation Review and Improvements

ORIGINAL IMPLEMENTATION ISSUES:
- Time Complexity: O(M × L × S) where M=manifests, L=lines per manifest, S=showOnly size
- Used strings.SplitSeq with slices.Collect (Go 1.23+ features) but inefficiently
- Called slices.Contains repeatedly (O(S) each time)
- Checked entire manifest content against showOnly patterns (incorrect logic)
- Processed every line of every manifest even when not needed

IMPROVED IMPLEMENTATION V2:
- Time Complexity: O(S) - optimal scaling using manifest map lookup
- Space Complexity: O(M) for the manifest map + O(S) for the showOnly set
- Uses pre-built manifest map with source file keys for O(1) lookup
- No string parsing needed during filtering - source files extracted once during rendering
- Direct map lookup instead of iterating through all manifests
- Cleaner separation of concerns between rendering and filtering

PERFORMANCE GAINS:
- Map-based filtering: O(S) instead of O(M × L × S)
- No duplicate parsing of Source comments
- Memory efficient with direct map access
- Scales only with showOnly size, not manifest count or content

STRATEGY VALIDATION:
- Helm templates always include "# Source: filename" comments at the top of manifests
- showOnly patterns match these source filenames exactly
- Case-sensitive matching (as per Helm behavior)
- Proper YAML document separation with "---\n"
- Enhanced with Helm metadata injection for better resource management
*/

// filterManifests filters manifests based on showOnly patterns using a manifest map.
// Improved implementation with O(S) complexity where S is showOnly size.
func filterManifests(manifestMap map[string]string, showOnly []string) string {
	if len(showOnly) == 0 {
		// Return all manifests joined together with proper separators
		// Sort keys for deterministic output in tests
		keys := make([]string, 0, len(manifestMap))
		for k := range manifestMap {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		var result strings.Builder
		first := true
		for _, key := range keys {
			manifest := strings.TrimSpace(manifestMap[key])
			if manifest == "" {
				continue
			}
			if !first {
				result.WriteString("\n---\n")
			} else {
				// First manifest should start with ---
				if !strings.HasPrefix(manifest, "---") {
					result.WriteString("---\n")
				}
				first = false
			}
			result.WriteString(manifest)
		}
		return result.String()
	}

	// Convert showOnly to a set for O(1) lookups
	showOnlySet := make(map[string]bool, len(showOnly))
	for _, pattern := range showOnly {
		showOnlySet[pattern] = true
	}

	var filtered []string

	// Look up manifests by their source file names in the order specified in showOnly
	// to maintain deterministic output
	for _, pattern := range showOnly {
		if manifest, exists := manifestMap[pattern]; exists {
			filtered = append(filtered, strings.TrimSpace(manifest))
		}
	}

	if len(filtered) > 0 {
		return strings.Join(filtered, "\n---\n")
	}

	return ""
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

// debug is a debug function for Helm with structured logging.
func debug(format string, v ...any) {
	slog.Debug(fmt.Sprintf(format, v...))
}

// handleHTTPRepository handles HTTP/HTTPS repository setup for caching and chart downloading.
func handleHTTPRepository(settings *cli.EnvSettings, repository, chart string) error {
	logger := slog.Default()

	if repository == "" || registry.IsOCI(repository) {
		return nil // Not an HTTP repository or empty, nothing to do
	}

	logger.Debug("Handling HTTP repository", "repository", repository, "chart", chart)

	// Ensure the repository cache directory exists
	if err := os.MkdirAll(settings.RepositoryCache, 0o755); err != nil {
		logger.Error(
			"Failed to create repository cache directory",
			"error",
			err,
			"path",
			settings.RepositoryCache,
		)
		return fmt.Errorf("failed to create repository cache directory: %w", err)
	}

	// Generate a repository name based on the URL for caching
	repoName := generateRepoName(repository)
	logger.Debug("Generated repository name", "repo_name", repoName, "repository", repository)

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
				logger.Warn(
					"Failed to load existing repository file, creating new one",
					"error",
					err,
					"path",
					settings.RepositoryConfig,
				)
				repoFile = repo.NewFile()
			}
		}
	}

	// Check if repository already exists
	if !repoFile.Has(repoName) {
		logger.Info(
			"Adding repository to configuration",
			"repo_name",
			repoName,
			"repository",
			repository,
		)
		// Add the repository to the file
		repoFile.Add(repoEntry)

		// Write the repositories file
		if err := repoFile.WriteFile(settings.RepositoryConfig, 0o644); err != nil {
			logger.Error(
				"Failed to write repository config",
				"error",
				err,
				"path",
				settings.RepositoryConfig,
			)
			return fmt.Errorf("failed to write repository config: %w", err)
		}
	} else {
		logger.Debug("Repository already exists in configuration", "repo_name", repoName)
	}

	// Create a ChartRepository for downloading the index
	chartRepo, err := repo.NewChartRepository(repoEntry, getter.All(settings))
	if err != nil {
		logger.Error("Failed to create chart repository", "error", err, "repository", repository)
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	// Set the cache file path
	chartRepo.CachePath = settings.RepositoryCache

	// Download and cache the repository index
	logger.Info("Downloading repository index", "repository", repository)
	indexFile, err := chartRepo.DownloadIndexFile()
	if err != nil {
		logger.Error("Failed to download repository index", "error", err, "repository", repository)
		return fmt.Errorf("failed to download repository index from %s: %w", repository, err)
	}

	logger.Info("Repository setup completed successfully",
		"repository", repository,
		"repo_name", repoName,
		"index_file", indexFile,
	)
	return nil
}

// generateRepoName generates a repository name from a URL.
func generateRepoName(repoURL string) string {
	// Simple implementation: use a hash or simplified name
	// Remove protocol and special characters
	name := strings.ReplaceAll(repoURL, "https://", "")
	name = strings.ReplaceAll(name, "http://", "")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return name
}

// addHelmMetadata adds Helm-specific labels and annotations to a manifest
func addHelmMetadata(manifestContent, releaseName, namespace string) (string, error) {
	if strings.TrimSpace(manifestContent) == "" {
		return manifestContent, nil
	}

	// Parse YAML manifest
	var manifest map[string]any
	if err := yaml.Unmarshal([]byte(manifestContent), &manifest); err != nil {
		// If we can't parse as YAML, return as-is (might be a comment or non-standard format)
		return manifestContent, nil
	}

	// Ensure metadata exists
	metadata, ok := manifest["metadata"].(map[string]any)
	if !ok {
		metadata = make(map[string]any)
		manifest["metadata"] = metadata
	}

	// Add/update labels
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		labels = make(map[string]any)
	}

	// Add managed-by label if not present
	if _, exists := labels["app.kubernetes.io/managed-by"]; !exists {
		labels["app.kubernetes.io/managed-by"] = "Helm"
	}

	metadata["labels"] = labels

	// Add/update annotations
	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok {
		annotations = make(map[string]any)
	}

	// Add Helm annotations
	annotations["meta.helm.sh/release-name"] = releaseName
	annotations["meta.helm.sh/release-namespace"] = namespace

	metadata["annotations"] = annotations

	// Marshal back to YAML
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(manifest); err != nil {
		return "", fmt.Errorf("failed to encode manifest with Helm metadata: %w", err)
	}

	return buffer.String(), nil
}

// ProcessTemplateWithConfig processes a Helm template request using the provided configuration.
func ProcessTemplateWithConfig(req TemplateRequest, cfg *config.Config) (string, error) {
	logger := slog.Default()

	// Validate required fields
	if req.Chart == "" {
		logger.Error("Chart parameter is required")
		return "", fmt.Errorf("chart is required")
	}

	// Set defaults
	if req.ReleaseName == "" {
		req.ReleaseName = "release-name"
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	logger.Info("Starting template processing",
		"chart", req.Chart,
		"repository", req.Repository,
		"chart_version", req.ChartVersion,
		"release_name", req.ReleaseName,
		"namespace", req.Namespace,
	)

	// Create Helm configuration using provided config
	settings := cli.New()
	actionConfig := new(action.Configuration)

	// Use config values instead of hardcoded paths
	settings.PluginsDirectory = cfg.Helm.PluginsDirectory
	settings.RepositoryCache = cfg.Helm.RepositoryCache
	settings.RepositoryConfig = cfg.Helm.RepositoryConfig
	settings.RegistryConfig = cfg.Helm.RegistryConfig

	logger.Debug("Setting up Helm environment",
		"plugins_dir", settings.PluginsDirectory,
		"repo_cache", settings.RepositoryCache,
		"repo_config", settings.RepositoryConfig,
		"registry_config", settings.RegistryConfig,
	)

	// Initialize action configuration for client-only mode (no cluster connection)
	logger.Debug("Initializing Helm action configuration")
	if err := actionConfig.Init(nil, req.Namespace, "memory", debug); err != nil {
		logger.Error("Failed to initialize Helm action config", "error", err)
		return "", fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Setup registry client for OCI support
	logger.Debug("Setting up registry client for OCI support")
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		logger.Error("Failed to create registry client", "error", err)
		return "", fmt.Errorf("failed to create registry client: %w", err)
	}
	actionConfig.RegistryClient = registryClient

	// Handle repository setup if needed
	if req.Repository != "" {
		logger.Info(
			"Setting up repository",
			"repository",
			req.Repository,
			"is_oci",
			registry.IsOCI(req.Repository),
		)
		if !registry.IsOCI(req.Repository) {
			// Handle HTTP/HTTPS repository setup
			if err := handleHTTPRepository(settings, req.Repository, req.Chart); err != nil {
				logger.Error(
					"Failed to handle HTTP repository",
					"error",
					err,
					"repository",
					req.Repository,
				)
				return "", fmt.Errorf("failed to handle HTTP repository: %w", err)
			}
		}
	}

	// Create install action (for templating)
	logger.Debug("Creating Helm install action for templating")
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
		logger.Debug("Parsing custom Kubernetes version", "kube_version", req.KubeVersion)
		kubeVersion, err := parseKubeVersion(req.KubeVersion)
		if err != nil {
			logger.Error(
				"Invalid Kubernetes version",
				"error",
				err,
				"kube_version",
				req.KubeVersion,
			)
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
				chartName = strings.TrimSuffix(
					req.Repository,
					"/",
				) + "/" + strings.TrimPrefix(
					chartName,
					"/",
				)
			}
		} else {
			// For HTTP/HTTPS repositories, use the repository name format that Helm expects
			repoName := generateRepoName(req.Repository)
			chartName = repoName + "/" + req.Chart
		}
	}

	if req.ChartVersion != "" {
		client.Version = req.ChartVersion
		client.Version = req.ChartVersion
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

	return renderManifests(rel, req.ShowOnly, req.SkipTests, req.CreateNamespace, req.Namespace, req.NamespaceLabels, req)
}
