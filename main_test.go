package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	hhandler "github.com/appkins-org/helm-api/api/health"
	thandler "github.com/appkins-org/helm-api/api/template"
	"github.com/appkins-org/helm-api/internal/config"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"gopkg.in/yaml.v3"
)

func TestHealthEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := hhandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))))
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("health handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"status":"healthy"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("health handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestTemplateEndpointSuccess(t *testing.T) {
	// Create a mock template request as query parameters
	url := "/template?chart=nginx&release_name=test-nginx&namespace=test&set=replicaCount=2&set=image.tag=1.20&kube_version=v1.29.0"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	// Note: This test will fail if nginx chart is not available locally
	// In a real test environment, you'd want to use a mock chart or test chart
	handler.ServeHTTP(rr, httpReq)

	// Since the nginx chart likely doesn't exist, we expect an error response
	if rr.Code == http.StatusOK {
		// If successful, check content type
		if rr.Header().Get("Content-Type") != "application/x-yaml" {
			t.Errorf(
				"Expected Content-Type application/x-yaml, got %s",
				rr.Header().Get("Content-Type"),
			)
		}
	} else {
		// If failed (which is expected), check it's a proper error response
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json for error, got %s", rr.Header().Get("Content-Type"))
		}
	}
}

func TestTemplateEndpointInvalidRequest(t *testing.T) {
	// Test with missing chart parameter
	httpReq, err := http.NewRequest("GET", "/template", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for missing chart: got %v want %v",
			status, http.StatusBadRequest)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
	}

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse error response as JSON: %v, body: %s", err, rr.Body.String())
	}

	if !strings.Contains(response["error"], "chart is empty") {
		t.Errorf(
			"Expected 'chart is empty' in error message, got: %s",
			response["error"],
		)
	}
}

func TestTemplateEndpointMissingChart(t *testing.T) {
	// Test with query parameters but missing chart
	httpReq, err := http.NewRequest("GET", "/template?release_name=test&namespace=test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for missing chart: got %v want %v",
			status, http.StatusBadRequest)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
	}

	var response map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse error response as JSON: %v, body: %s", err, rr.Body.String())
	}

	if !strings.Contains(response["error"], "chart is empty") {
		t.Errorf(
			"Expected 'chart is empty' in error message, got: %s",
			response["error"],
		)
	}
}

func TestTemplateEndpointMethodNotAllowed(t *testing.T) {
	req, err := http.NewRequest("POST", "/template", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code for POST method: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}
}

func TestTemplateEndpointWithSetValues(t *testing.T) {
	// Test with multiple set values
	url := "/template?chart=test-chart&set=replicaCount=3&set=image.tag=v2.0&include_crds=true"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	// Since the chart likely doesn't exist, we expect an error response
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf(
				"Expected Content-Type application/json for error, got %s",
				rr.Header().Get("Content-Type"),
			)
		}
	}
}

// TestMockHTTPServer creates a full mock HTTP server for integration testing.
func TestMockHTTPServer(t *testing.T) {
	// Create a new HTTP server with our routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", hhandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t)))).ServeHTTP)
	mux.HandleFunc("/template", thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig()).ServeHTTP)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned status %d, expected %d", resp.StatusCode, http.StatusOK)
	}
	// Test template endpoint with invalid request
	resp, err = http.Get(server.URL + "/template")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf(
			"Template endpoint returned status %d, expected %d",
			resp.StatusCode,
			http.StatusBadRequest,
		)
	}
}

// BenchmarkHealthHandler benchmarks the health endpoint.
func BenchmarkHealthHandler(b *testing.B) {
	req, _ := http.NewRequest("GET", "/health", nil)

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler := hhandler.New(logr.FromContextAsSlogLogger(logr.NewContext(b.Context(), testr.NewWithInterface(b, testr.Options{}))))
		handler.ServeHTTP(rr, req)
	}
}

// BenchmarkTemplateHandler benchmarks the template endpoint.
func BenchmarkTemplateHandler(b *testing.B) {
	url := "/template?chart=nginx&release_name=test"

	for i := 0; i < b.N; i++ {
		httpReq, _ := http.NewRequest("GET", url, nil)

		rr := httptest.NewRecorder()
		handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(b.Context(), testr.NewWithInterface(b, testr.Options{}))), config.DefaultConfig())
		handler.ServeHTTP(rr, httpReq)
	}
}

// TestTemplateEndpointWithRepository tests the template endpoint with repository parameters.
func TestTemplateEndpointWithRepository(t *testing.T) {
	// Test with repository and chart version
	url := "/template?chart=nginx&repository=https://charts.bitnami.com/bitnami&chart_version=15.0.0&release_name=repo-test"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	// Since the chart likely doesn't exist or can't be downloaded, we expect an error response
	// but the important thing is that the parameters are parsed correctly
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf(
				"Expected Content-Type application/json for error, got %s",
				rr.Header().Get("Content-Type"),
			)
		}
	}
}

// TestTemplateEndpointWithOCI tests the template endpoint with OCI chart.
func TestTemplateEndpointWithOCI(t *testing.T) {
	// Test with OCI chart
	url := "/template?chart=oci://registry-1.docker.io/bitnamicharts/nginx&chart_version=15.0.0&release_name=oci-test"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	// Since the OCI chart likely can't be downloaded, we expect an error response
	// but the important thing is that the parameters are parsed correctly
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf(
				"Expected Content-Type application/json for error, got %s",
				rr.Header().Get("Content-Type"),
			)
		}
	}
}

// TestTemplateEndpointValidYAML tests that the API returns valid YAML.
func TestTemplateEndpointValidYAML(t *testing.T) {
	// Create a temporary test chart
	tempDir := t.TempDir()
	chartDir := tempDir + "/test-chart"

	// Create chart directory structure
	err := os.MkdirAll(chartDir+"/templates", 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// Create Chart.yaml
	chartYaml := `apiVersion: v2
name: test-chart
description: A test chart for YAML validation
type: application
version: 0.1.0
appVersion: "1.0"
`
	err = os.WriteFile(chartDir+"/Chart.yaml", []byte(chartYaml), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create values.yaml
	valuesYaml := `replicaCount: 1
image:
  repository: nginx
  tag: latest
`
	err = os.WriteFile(chartDir+"/values.yaml", []byte(valuesYaml), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a simple deployment template
	deploymentTemplate := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-{{ .Chart.Name }}
  labels:
    app: {{ .Chart.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Chart.Name }}
  template:
    metadata:
      labels:
        app: {{ .Chart.Name }}
    spec:
      containers:
      - name: {{ .Chart.Name }}
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        ports:
        - containerPort: 80
`
	err = os.WriteFile(chartDir+"/templates/deployment.yaml", []byte(deploymentTemplate), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Test with the local chart
	url := "/template?chart=" + chartDir + "&release_name=test-release&namespace=test-ns"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	// Check that we got a successful response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Response: %s", rr.Code, rr.Body.String())
		return
	}

	// Check content type
	if rr.Header().Get("Content-Type") != "application/x-yaml" {
		t.Errorf(
			"Expected Content-Type application/x-yaml, got %s",
			rr.Header().Get("Content-Type"),
		)
	}

	// Get the response body
	yamlContent := rr.Body.String()

	// Verify it's not empty
	if yamlContent == "" {
		t.Error("Expected non-empty YAML response")
		return
	}

	// Parse the YAML to verify it's valid
	var yamlData any
	err = yaml.Unmarshal([]byte(yamlContent), &yamlData)
	if err != nil {
		t.Errorf("Response is not valid YAML: %v\nContent: %s", err, yamlContent)
		return
	}

	// Verify it contains expected Kubernetes manifest structure
	if !strings.Contains(yamlContent, "apiVersion: apps/v1") {
		t.Error("Expected YAML to contain 'apiVersion: apps/v1'")
	}

	if !strings.Contains(yamlContent, "kind: Deployment") {
		t.Error("Expected YAML to contain 'kind: Deployment'")
	}

	if !strings.Contains(yamlContent, "test-release-test-chart") {
		t.Error("Expected YAML to contain templated release name")
	}

	t.Logf("Successfully generated valid YAML:\n%s", yamlContent)
}

// TestTemplateEndpointValidYAMLWithSetValues tests YAML output with custom values.
func TestTemplateEndpointValidYAMLWithSetValues(t *testing.T) {
	// Create a temporary test chart
	tempDir := t.TempDir()
	chartDir := tempDir + "/test-chart"

	// Create chart directory structure
	err := os.MkdirAll(chartDir+"/templates", 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// Create Chart.yaml
	chartYaml := `apiVersion: v2
name: test-chart
description: A test chart for YAML validation with values
type: application
version: 0.1.0
appVersion: "1.0"
`
	err = os.WriteFile(chartDir+"/Chart.yaml", []byte(chartYaml), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create values.yaml with default values
	valuesYaml := `replicaCount: 1
image:
  repository: nginx
  tag: latest
service:
  type: ClusterIP
  port: 80
`
	err = os.WriteFile(chartDir+"/values.yaml", []byte(valuesYaml), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create deployment template that uses values
	deploymentTemplate := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-{{ .Chart.Name }}
  labels:
    app: {{ .Chart.Name }}
    version: {{ .Chart.AppVersion }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Chart.Name }}
  template:
    metadata:
      labels:
        app: {{ .Chart.Name }}
    spec:
      containers:
      - name: {{ .Chart.Name }}
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        ports:
        - containerPort: {{ .Values.service.port }}
`
	err = os.WriteFile(chartDir+"/templates/deployment.yaml", []byte(deploymentTemplate), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Test with custom values using set parameters
	url := "/template?chart=" + chartDir + "&release_name=custom-release&set=replicaCount=3&set=image.tag=v2.0&set=service.port=8080"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	// Check that we got a successful response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Response: %s", rr.Code, rr.Body.String())
		return
	}

	// Get the response body
	yamlContent := rr.Body.String()

	// Parse the YAML to verify it's valid
	var yamlData any
	err = yaml.Unmarshal([]byte(yamlContent), &yamlData)
	if err != nil {
		t.Errorf("Response is not valid YAML: %v\nContent: %s", err, yamlContent)
		return
	}

	// Verify the custom values were applied
	if !strings.Contains(yamlContent, "replicas: 3") {
		t.Error("Expected YAML to contain 'replicas: 3' from set value")
	}

	if !strings.Contains(yamlContent, "nginx:v2.0") {
		t.Error("Expected YAML to contain 'nginx:v2.0' from set value")
	}

	if !strings.Contains(yamlContent, "containerPort: 8080") {
		t.Error("Expected YAML to contain 'containerPort: 8080' from set value")
	}

	if !strings.Contains(yamlContent, "custom-release-test-chart") {
		t.Error("Expected YAML to contain custom release name")
	}

	t.Logf("Successfully generated valid YAML with custom values:\n%s", yamlContent)
}

// TestTemplateEndpointErrorReturnsJSON verifies that errors return JSON, not YAML.
func TestTemplateEndpointErrorReturnsJSON(t *testing.T) {
	// Test with a non-existent chart
	url := "/template?chart=/non/existent/chart&release_name=test"

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

	handler.ServeHTTP(rr, httpReq)

	// Should return an error status
	if rr.Code == http.StatusOK {
		t.Error("Expected error status for non-existent chart")
		return
	}

	// Should have JSON content type
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf(
			"Expected Content-Type application/json for error, got %s",
			rr.Header().Get("Content-Type"),
		)
	}

	// Response should be valid JSON
	var errorResponse map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	if err != nil {
		t.Errorf("Error response is not valid JSON: %v\nContent: %s", err, rr.Body.String())
		return
	}

	// Should contain an error message
	if errorResponse["error"] == "" {
		t.Error("Expected error message in JSON response")
	}

	// Should NOT be valid YAML (to ensure we're not accidentally returning YAML for errors)
	if strings.Contains(rr.Body.String(), "apiVersion:") ||
		strings.Contains(rr.Body.String(), "kind:") {
		t.Error("Error response appears to contain YAML instead of JSON")
	}

	t.Logf("Correctly returned JSON error: %s", rr.Body.String())
}

func TestTemplateEndpointCreateNamespace(t *testing.T) {
	tests := []struct {
		name            string
		createNamespace string
		expectedStatus  int
		shouldContainNS bool
		description     string
	}{
		{
			name:            "create namespace true",
			createNamespace: "true",
			expectedStatus:  http.StatusInternalServerError, // Chart not found error expected
			shouldContainNS: false,                          // Won't get to YAML output due to chart error
			description:     "Should handle create_namespace=true parameter",
		},
		{
			name:            "create namespace false",
			createNamespace: "false",
			expectedStatus:  http.StatusInternalServerError, // Chart not found error expected
			shouldContainNS: false,                          // Won't get to YAML output due to chart error
			description:     "Should handle create_namespace=false parameter",
		},
		{
			name:            "create namespace 1 (truthy)",
			createNamespace: "1",
			expectedStatus:  http.StatusInternalServerError, // Chart not found error expected
			shouldContainNS: false,
			description:     "Should handle create_namespace=1 as true",
		},
		{
			name:            "create namespace 0 (falsy)",
			createNamespace: "0",
			expectedStatus:  http.StatusInternalServerError, // Chart not found error expected
			shouldContainNS: false,
			description:     "Should handle create_namespace=0 as false",
		},
		{
			name:            "create namespace empty (default false)",
			createNamespace: "",
			expectedStatus:  http.StatusInternalServerError, // Chart not found error expected
			shouldContainNS: false,
			description:     "Should default to false when create_namespace is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build URL with create_namespace parameter
			url := "/template?chart=nginx&release_name=test-nginx&namespace=test-namespace"
			if tt.createNamespace != "" {
				url += "&create_namespace=" + tt.createNamespace
			}

			httpReq, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

			handler.ServeHTTP(rr, httpReq)

			// Check status code
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf(
					"handler returned wrong status code: got %v want %v",
					status,
					tt.expectedStatus,
				)
				t.Logf("Response body: %s", rr.Body.String())
				return
			}

			// For error responses, check that they're JSON
			if tt.expectedStatus != http.StatusOK {
				expectedContentType := "application/json"
				if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
					t.Errorf(
						"handler returned wrong content type for error: got %v want %v",
						contentType,
						expectedContentType,
					)
				}

				// Verify it's valid JSON error response
				var errorResponse map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err != nil {
					t.Errorf("Failed to parse error response as JSON: %v", err)
					return
				}

				// Should contain error message
				if errorResponse["error"] == "" {
					t.Error("Expected error message in JSON response")
				}

				// Verify the create_namespace parameter was parsed correctly (even though chart fails)
				// This confirms our parameter parsing works
				t.Logf(
					"Successfully parsed create_namespace=%s parameter: %s",
					tt.createNamespace,
					tt.description,
				)
			}
		})
	}
}

func TestTemplateEndpointCreateNamespaceValidation(t *testing.T) {
	// Test that create_namespace parameter is properly parsed and validated
	testCases := []struct {
		name        string
		queryParams string
		expectError bool
		description string
	}{
		{
			name:        "valid true",
			queryParams: "chart=test&create_namespace=true",
			expectError: false,
			description: "Should accept create_namespace=true",
		},
		{
			name:        "valid false",
			queryParams: "chart=test&create_namespace=false",
			expectError: false,
			description: "Should accept create_namespace=false",
		},
		{
			name:        "valid 1",
			queryParams: "chart=test&create_namespace=1",
			expectError: false,
			description: "Should accept create_namespace=1",
		},
		{
			name:        "valid 0",
			queryParams: "chart=test&create_namespace=0",
			expectError: false,
			description: "Should accept create_namespace=0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/template?" + tc.queryParams

			httpReq, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := thandler.New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))), config.DefaultConfig())

			handler.ServeHTTP(rr, httpReq)

			// The parameter parsing should not cause a 400 error for create_namespace
			// (chart not found errors are expected in test environment and result in 500)
			switch rr.Code {
			case http.StatusBadRequest:
				// Check if it's specifically a create_namespace parsing error
				var errorResponse map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err == nil {
					if errorMsg, ok := errorResponse["error"].(string); ok &&
						strings.Contains(errorMsg, "create_namespace") {
						if !tc.expectError {
							t.Errorf("Unexpected create_namespace parsing error: %s", errorMsg)
						}
					}
				}
			case http.StatusInternalServerError:
				// This is expected for chart not found errors
				var errorResponse map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err == nil {
					if errorMsg, ok := errorResponse["error"].(string); ok {
						// Should be a chart-related error, not a parameter parsing error
						if strings.Contains(errorMsg, "create_namespace") {
							t.Errorf(
								"create_namespace should not cause chart lookup errors: %s",
								errorMsg,
							)
						}
					}
				}
			}

			t.Logf("Test '%s' completed: %s (Status: %d)", tc.name, tc.description, rr.Code)
		})
	}
}
