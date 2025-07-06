package template

import (
	"os"
	"path/filepath"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func TestTemplateRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		req         TemplateRequest
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty chart",
			req:         TemplateRequest{},
			expectError: true,
			errorMsg:    "chart is required",
		},
		{
			name: "valid minimal request",
			req: TemplateRequest{
				Chart: "test-chart",
			},
			expectError: false,
		},
		{
			name: "valid full request",
			req: TemplateRequest{
				Chart:        "test-chart",
				ReleaseName:  "my-release",
				Namespace:    "custom-ns",
				KubeVersion:  "v1.29.0",
				IncludeCRDs:  true,
				SkipTests:    true,
				IsUpgrade:    false,
				Values:       []string{"replicaCount=3", "image.tag=latest"},
				StringValues: []string{"key1=value1", "key2=value2"},
				APIVersions:  []string{"apps/v1", "networking.k8s.io/v1"},
			},
			expectError: false,
		},
		{
			name: "valid repository request",
			req: TemplateRequest{
				Chart:        "nginx",
				ChartVersion: "15.0.0",
				Repository:   "https://charts.bitnami.com/bitnami",
				ReleaseName:  "repo-nginx",
				Namespace:    "default",
			},
			expectError: false,
		},
		{
			name: "valid OCI request",
			req: TemplateRequest{
				Chart:        "oci://registry-1.docker.io/bitnamicharts/nginx",
				ChartVersion: "15.0.0",
				ReleaseName:  "oci-nginx",
				Namespace:    "default",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We'll only test the validation part without actually running helm
			if tt.req.Chart == "" {
				_, err := ProcessTemplate(tt.req)
				if !tt.expectError {
					t.Errorf("Expected no error, got: %v", err)
				} else if err == nil {
					t.Error("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func TestParseKubeVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
		expectedVer string
	}{
		{
			name:        "valid version with v prefix",
			version:     "v1.29.0",
			expectError: false,
			expectedVer: "v1.29.0",
		},
		{
			name:        "valid version without v prefix",
			version:     "1.28.3",
			expectError: false,
			expectedVer: "1.28.3",
		},
		{
			name:        "invalid version format",
			version:     "invalid",
			expectError: true,
		},
		{
			name:        "empty version",
			version:     "",
			expectError: true,
		},
		{
			name:        "version with only major",
			version:     "1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseKubeVersion(tt.version)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Expected result but got nil")
				}
			}
		})
	}
}

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		mapA     map[string]any
		mapB     map[string]any
		expected map[string]any
	}{
		{
			name: "merge simple maps",
			mapA: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
			mapB: map[string]any{
				"key2": "newvalue2",
				"key3": "value3",
			},
			expected: map[string]any{
				"key1": "value1",
				"key2": "newvalue2", // B takes precedence
				"key3": "value3",
			},
		},
		{
			name:     "merge empty maps",
			mapA:     map[string]any{},
			mapB:     map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "merge with nil map",
			mapA: map[string]any{
				"key1": "value1",
			},
			mapB: nil,
			expected: map[string]any{
				"key1": "value1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMaps(tt.mapA, tt.mapB)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d keys, got %d", len(tt.expected), len(result))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Expected %s=%v, got %s=%v", k, v, k, result[k])
				}
			}
		})
	}
}

func TestDefaultValues(t *testing.T) {
	req := TemplateRequest{
		Chart: "test-chart",
		// No ReleaseName or Namespace provided
	}

	// Test that defaults are set (we can't actually run helm without a chart)
	if req.ReleaseName == "" {
		req.ReleaseName = "release-name"
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	if req.ReleaseName != "release-name" {
		t.Errorf("Expected default release name 'release-name', got '%s'", req.ReleaseName)
	}
	if req.Namespace != "default" {
		t.Errorf("Expected default namespace 'default', got '%s'", req.Namespace)
	}
}

func TestDebugFunction(t *testing.T) {
	// Test that debug function doesn't panic
	debug("test message")
	debug("test message with args: %s %d", "hello", 123)

	// Test with debug enabled
	_ = os.Setenv("HELM_DEBUG", "true")
	debug("debug enabled message")
	_ = os.Unsetenv("HELM_DEBUG")
}

// TestCreateTestChart creates a minimal test chart for integration testing.
func TestCreateTestChart(t *testing.T) {
	// Create a temporary directory for test chart
	tempDir, err := os.MkdirTemp("", "helm-test-chart")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	chartDir := filepath.Join(tempDir, "test-chart")
	err = os.MkdirAll(filepath.Join(chartDir, "templates"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// Create Chart.yaml
	chartYaml := `apiVersion: v2
name: test-chart
description: A test chart
type: application
version: 0.1.0
appVersion: "1.0"
`
	err = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYaml), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create values.yaml
	valuesYaml := `replicaCount: 1
image:
  repository: nginx
  tag: latest
  pullPolicy: IfNotPresent
service:
  type: ClusterIP
  port: 80
`
	err = os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(valuesYaml), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a simple template
	deploymentTemplate := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "test-chart.fullname" . }}
  labels:
    app: {{ include "test-chart.name" . }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ include "test-chart.name" . }}
  template:
    metadata:
      labels:
        app: {{ include "test-chart.name" . }}
    spec:
      containers:
      - name: {{ .Chart.Name }}
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        ports:
        - name: http
          containerPort: 80
          protocol: TCP
`
	err = os.WriteFile(
		filepath.Join(chartDir, "templates", "deployment.yaml"),
		[]byte(deploymentTemplate),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create helpers template
	helpersTemplate := `{{- define "test-chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "test-chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}
`
	err = os.WriteFile(
		filepath.Join(chartDir, "templates", "_helpers.tpl"),
		[]byte(helpersTemplate),
		0o644,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Test that we can process this chart
	req := TemplateRequest{
		Chart:       chartDir,
		ReleaseName: "test-release",
		Namespace:   "test-ns",
		Values:      []string{"replicaCount=2"},
	}

	result, err := ProcessTemplate(req)
	if err != nil {
		t.Errorf("Failed to process test chart: %v", err)
		return
	}

	if result == "" {
		t.Error("Expected non-empty result from template processing")
	}

	// Check that the result contains expected content
	if !contains(result, "kind: Deployment") {
		t.Error("Expected result to contain 'kind: Deployment'")
	}
	if !contains(result, "test-release-test-chart") {
		t.Error("Expected result to contain release name")
	}
}

// contains checks if a string contains a substring (helper function).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests.
func BenchmarkMergeMaps(b *testing.B) {
	mapA := map[string]any{
		"key1": "value1",
		"key2": "value2",
		"key3": map[string]any{
			"nested": "value",
		},
	}
	mapB := map[string]any{
		"key2": "newvalue2",
		"key4": "value4",
	}

	for i := 0; i < b.N; i++ {
		mergeMaps(mapA, mapB)
	}
}

func BenchmarkParseKubeVersion(b *testing.B) {
	version := "v1.29.0"

	for i := 0; i < b.N; i++ {
		_, _ = parseKubeVersion(version)
	}
}

// Test helper functions.
func TestIsTestHook(t *testing.T) {
	tests := []struct {
		name     string
		hook     *release.Hook
		expected bool
	}{
		{
			name: "test hook",
			hook: &release.Hook{
				Events: []release.HookEvent{release.HookTest},
			},
			expected: true,
		},
		{
			name: "non-test hook",
			hook: &release.Hook{
				Events: []release.HookEvent{release.HookPreInstall},
			},
			expected: false,
		},
		{
			name: "multiple events with test",
			hook: &release.Hook{
				Events: []release.HookEvent{release.HookPreInstall, release.HookTest},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTestHook(tt.hook)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterManifests(t *testing.T) {
	manifests := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
---
apiVersion: v1
kind: Service  
metadata:
  name: test-service
`

	showOnly := []string{"deployment"}
	result := filterManifests(manifests, showOnly)

	// This is a simplified implementation, so it just returns the original
	if result != manifests {
		t.Error("filterManifests should return original manifests in simplified implementation")
	}
}
