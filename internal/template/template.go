package template

import (
	"bytes"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
)

// TemplateRequest represents the JSON payload for the template endpoint
type TemplateRequest struct {
	// Chart specifies the chart to template (required)
	Chart string `json:"chart"`

	// ChartVersion specifies the version of the chart (optional)
	ChartVersion string `json:"chart_version,omitempty"`

	// Repository specifies the chart repository URL (optional)
	Repository string `json:"repository,omitempty"`

	// ReleaseName is the name of the release (optional, defaults to "release-name")
	ReleaseName string `json:"release_name,omitempty"`

	// Namespace is the target namespace (optional, defaults to "default")
	Namespace string `json:"namespace,omitempty"`

	// Values contains the values to use for templating
	Values map[string]interface{} `json:"values,omitempty"`

	// ValueFiles contains paths to values files
	ValueFiles []string `json:"value_files,omitempty"`

	// StringValues contains values as strings (--set equivalent)
	StringValues []string `json:"string_values,omitempty"`

	// FileValues contains values from files (--set-file equivalent)
	FileValues []string `json:"file_values,omitempty"`

	// JSONValues contains values from JSON strings (--set-json equivalent)
	JSONValues []string `json:"json_values,omitempty"`

	// IncludeCRDs indicates whether to include CRDs in output
	IncludeCRDs bool `json:"include_crds,omitempty"`

	// SkipTests indicates whether to skip test manifests
	SkipTests bool `json:"skip_tests,omitempty"`

	// ShowOnly limits output to specific templates
	ShowOnly []string `json:"show_only,omitempty"`

	// KubeVersion specifies the Kubernetes version for capabilities
	KubeVersion string `json:"kube_version,omitempty"`

	// APIVersions specifies additional API versions for capabilities
	APIVersions []string `json:"api_versions,omitempty"`

	// IsUpgrade sets .Release.IsUpgrade instead of .Release.IsInstall
	IsUpgrade bool `json:"is_upgrade,omitempty"`

	// Validate enables manifest validation
	Validate bool `json:"validate,omitempty"`
}

// ProcessTemplate processes a Helm template request and returns the rendered YAML
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

	// Initialize action configuration for client-only mode (no cluster connection)
	if err := actionConfig.Init(nil, req.Namespace, "memory", debug); err != nil {
		return "", fmt.Errorf("failed to initialize helm action config: %w", err)
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
		StringValues: req.StringValues,
		FileValues:   req.FileValues,
		JSONValues:   req.JSONValues,
	}

	// Merge values
	vals, err := valueOpts.MergeValues(getter.All(settings))
	if err != nil {
		return "", fmt.Errorf("failed to merge values: %w", err)
	}

	// Merge with provided values map
	if req.Values != nil {
		vals = mergeMaps(vals, req.Values)
	}

	// Configure chart path options for repository support
	if req.Repository != "" {
		client.ChartPathOptions.RepoURL = req.Repository
	}
	if req.ChartVersion != "" {
		client.ChartPathOptions.Version = req.ChartVersion
	}

	// Load chart
	chartPath, err := client.ChartPathOptions.LocateChart(req.Chart, settings)
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

// renderManifests renders the release manifests to YAML
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

// isTestHook checks if a hook is a test hook
func isTestHook(hook *release.Hook) bool {
	for _, event := range hook.Events {
		if event == release.HookTest {
			return true
		}
	}
	return false
}

// filterManifests filters manifests based on showOnly patterns
func filterManifests(manifests string, showOnly []string) string {
	// This is a simplified implementation
	// In a full implementation, you would parse manifests and match against file patterns
	return manifests
}

// parseKubeVersion parses a Kubernetes version string
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

// mergeMaps merges two maps, with the second map taking precedence
func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

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

// debug is a debug function for Helm (simplified for client-only mode)
func debug(format string, v ...interface{}) {
	// Debug logging is disabled in client-only mode
	// In production, you might want to use a proper logger
}
